package server

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cookiengineer/gows"
	app "github.com/cookiengineer/words-with-learners-app"
	"github.com/cookiengineer/words-with-learners-app/internal/auth"
	"github.com/cookiengineer/words-with-learners-app/internal/config"
	"github.com/cookiengineer/words-with-learners-app/internal/dictionary"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/store"
	"github.com/cookiengineer/words-with-learners-app/internal/view"
)

func init() {
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

type gameSession struct {
	mutex        sync.Mutex
	value        *game.Game
	hintRevision uint64
	hintPlayer   game.PlayerID
	hintValue    view.Hint
	hintValid    bool
}

type client struct {
	profile *store.Profile
	socket  *gows.WebSocket
	server  *Server
	closed  bool
	mutex   sync.Mutex
}

type Server struct {
	cfg          config.Config
	store        *store.Store
	auth         *auth.Manager
	dictionaries *dictionary.Library
	publicFS     fs.FS
	games        map[game.GameID]*gameSession
	gamesMutex   sync.RWMutex
	clients      map[*client]struct{}
	clientsMutex sync.RWMutex
	ws           *gows.Server
	logger       *log.Logger
}

func New(cfg config.Config) (*Server, error) {
	profiles, err := store.New(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	var publicFS fs.FS
	var dictFS fs.FS
	if cfg.Dev {
		publicFS = os.DirFS(cfg.PublicDir)
		dictFS = os.DirFS(cfg.DictDir)
	} else {
		publicFS, err = fs.Sub(app.Files, "public")
		if err != nil {
			return nil, err
		}
		dictFS, err = fs.Sub(app.Files, "dictionaries")
		if err != nil {
			return nil, err
		}
	}
	library, err := dictionary.LoadLibrary(dictFS)
	if err != nil {
		return nil, err
	}
	authManager, err := auth.NewManager(profiles)
	if err != nil {
		return nil, err
	}
	server := &Server{
		cfg:          cfg,
		store:        profiles,
		auth:         authManager,
		dictionaries: library,
		publicFS:     publicFS,
		games:        make(map[game.GameID]*gameSession),
		clients:      make(map[*client]struct{}),
		ws:           &gows.Server{},
		logger:       log.New(os.Stdout, "[wwl] ", log.LstdFlags),
	}
	loaded, err := profiles.LoadGames()
	if err != nil {
		return nil, err
	}
	for _, loadedGame := range loaded {
		dict, dictErr := library.Get(loadedGame.Config.Language, loadedGame.Config.Level)
		if dictErr != nil {
			return nil, fmt.Errorf("load game %s: %w", loadedGame.ID, dictErr)
		}
		if cfg.VerifyGame {
			if replayErr := game.Replay(loadedGame, dict); replayErr != nil {
				return nil, fmt.Errorf("verify game %s: %w", loadedGame.ID, replayErr)
			}
		}
		if reshuffle := ensurePlayable(loadedGame, dict); reshuffle != nil {
			loadedGame.Revision++
			loadedGame.UpdatedAt = time.Now().UTC()
			reshuffle.Revision = loadedGame.Revision
			loadedGame.LastReshuffle = reshuffle
			loadedGame.Reshuffles = append(loadedGame.Reshuffles, *reshuffle)
			if saveErr := profiles.SaveGame(loadedGame); saveErr != nil {
				return nil, fmt.Errorf("save reshuffled game %s: %w", loadedGame.ID, saveErr)
			}
			server.logger.Printf("auto-reshuffled stuck rack in game %s for %s", loadedGame.ID, reshuffle.PlayerID)
		}
		server.games[loadedGame.ID] = &gameSession{value: loadedGame}
	}
	return server, nil
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	mux.HandleFunc("/api/auth/login", server.handleLogin)
	mux.HandleFunc("/api/auth/logout", server.handleLogout)
	mux.HandleFunc("/api/auth/me", server.handleMe)
	mux.Handle("/", http.FileServer(http.FS(server.publicFS)))
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "same-origin")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' ws: wss:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(response, request)
	})
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ProfileResponse struct {
	ID       game.PlayerID `json:"id"`
	Username string        `json:"username"`
}

func profileResponse(profile *store.Profile) ProfileResponse {
	return ProfileResponse{ID: profile.ID, Username: profile.Username}
}

func (server *Server) handleLogin(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var payload LoginRequest
	if err := decodeJSON(request, &payload); err != nil {
		writeError(response, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	profile, token, err := server.auth.LoginOrRegister(payload.Username, payload.Password, time.Now())
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeError(response, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrInvalidPassword):
			writeError(response, http.StatusBadRequest, "invalid_input", err.Error())
		default:
			writeError(response, http.StatusInternalServerError, "internal_error", "login failed")
		}
		return
	}
	server.auth.SetCookie(response, request, token)
	writeJSON(response, http.StatusOK, profileResponse(profile))
}

func (server *Server) handleLogout(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	token := server.auth.TokenFromRequest(request)
	server.auth.Logout(token)
	server.auth.ClearCookie(response, request)
	writeJSON(response, http.StatusOK, map[string]bool{"ok": true})
}

func (server *Server) handleMe(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	token := server.auth.TokenFromRequest(request)
	playerID, ok := server.auth.Authenticate(token, time.Now())
	if !ok {
		if token != "" {
			server.auth.ClearCookie(response, request)
		}
		writeError(response, http.StatusUnauthorized, "unauthorized", "log in first")
		return
	}
	profile, ok := server.store.Profile(playerID)
	if !ok {
		writeError(response, http.StatusUnauthorized, "unauthorized", "profile not found")
		return
	}
	writeJSON(response, http.StatusOK, profileResponse(profile))
}

func upgradeWebSocket(response http.ResponseWriter, request *http.Request) (net.Conn, error) {
	if !headerHasToken(request.Header, "Upgrade", "websocket") {
		writeError(response, http.StatusBadRequest, "invalid_upgrade", "missing websocket upgrade header")
		return nil, fmt.Errorf("missing upgrade header")
	}
	if !headerHasToken(request.Header, "Connection", "upgrade") {
		writeError(response, http.StatusBadRequest, "invalid_upgrade", "missing websocket connection header")
		return nil, fmt.Errorf("missing connection header")
	}
	if request.Header.Get("Sec-WebSocket-Version") != "13" {
		writeError(response, http.StatusUpgradeRequired, "unsupported_version", "websocket version 13 is required")
		return nil, fmt.Errorf("unsupported websocket version")
	}
	key := request.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		writeError(response, http.StatusBadRequest, "invalid_upgrade", "missing websocket key")
		return nil, fmt.Errorf("missing websocket key")
	}
	hijacker, ok := response.(http.Hijacker)
	if !ok {
		http.Error(response, "websocket upgrade is not supported", http.StatusInternalServerError)
		return nil, fmt.Errorf("response does not support hijacking")
	}
	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	accept := base64.StdEncoding.EncodeToString(sum[:])
	_, err = fmt.Fprintf(buffer, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", accept)
	if err == nil {
		err = buffer.Flush()
	}
	if err != nil {
		connection.Close()
		return nil, err
	}
	return connection, nil
}

func headerHasToken(headers http.Header, name string, token string) bool {
	for _, value := range headers[http.CanonicalHeaderKey(name)] {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

func decodeJSON(request *http.Request, target any) error {
	defer request.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}

func writeError(response http.ResponseWriter, status int, code string, message string) {
	writeJSON(response, status, ErrorPayload{Code: code, Message: message})
}

func (server *Server) handleWebSocket(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	if origin := request.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(parsed.Host, request.Host) {
			writeError(response, http.StatusForbidden, "invalid_origin", "websocket origin does not match this host")
			return
		}
	}
	token := server.auth.TokenFromRequest(request)
	playerID, ok := server.auth.Authenticate(token, time.Now())
	if !ok {
		if token != "" {
			server.auth.ClearCookie(response, request)
		}
		writeError(response, http.StatusUnauthorized, "unauthorized", "log in first")
		return
	}
	profile, ok := server.store.Profile(playerID)
	if !ok {
		writeError(response, http.StatusUnauthorized, "unauthorized", "profile not found")
		return
	}
	connection, err := upgradeWebSocket(response, request)
	if err != nil {
		return
	}
	socket := gows.NewWebSocket(connection, server.ws)
	conn := &client{profile: profile, socket: socket, server: server}
	socket.OnMessage = func(message []byte) {
		server.handleMessage(conn, message)
	}
	socket.OnClose = func() {
		conn.mutex.Lock()
		conn.closed = true
		conn.mutex.Unlock()
		server.clientsMutex.Lock()
		delete(server.clients, conn)
		server.clientsMutex.Unlock()
		server.logger.Printf("websocket disconnected: %s", profile.Username)
		server.broadcastGamesChanged()
	}
	server.clientsMutex.Lock()
	server.clients[conn] = struct{}{}
	server.clientsMutex.Unlock()
	server.logger.Printf("websocket connected: %s", profile.Username)
	go socket.Init()
}

func (conn *client) send(messageType string, requestID string, payload any) {
	if conn == nil || conn.socket == nil {
		return
	}
	raw, err := json.Marshal(Outgoing{Type: messageType, RequestID: requestID, Payload: payload})
	if err != nil {
		return
	}
	_ = conn.socket.Send(raw)
}

func (server *Server) onlinePlayers() map[game.PlayerID]bool {
	result := make(map[game.PlayerID]bool)
	server.clientsMutex.RLock()
	defer server.clientsMutex.RUnlock()
	for conn := range server.clients {
		result[conn.profile.ID] = true
	}
	return result
}

func (server *Server) session(id game.GameID) (*gameSession, bool) {
	server.gamesMutex.RLock()
	defer server.gamesMutex.RUnlock()
	session, ok := server.games[id]
	return session, ok
}

func (server *Server) allSessions() []*gameSession {
	server.gamesMutex.RLock()
	defer server.gamesMutex.RUnlock()
	result := make([]*gameSession, 0, len(server.games))
	for _, session := range server.games {
		result = append(result, session)
	}
	return result
}

func (server *Server) broadcastGamesChanged() {
	server.clientsMutex.RLock()
	clients := make([]*client, 0, len(server.clients))
	for conn := range server.clients {
		clients = append(clients, conn)
	}
	server.clientsMutex.RUnlock()
	for _, conn := range clients {
		conn.send(TypeGamesChanged, "", map[string]bool{"changed": true})
	}
}

func (server *Server) broadcastGame(value *game.Game) {
	online := server.onlinePlayers()
	server.clientsMutex.RLock()
	clients := make([]*client, 0, len(server.clients))
	for conn := range server.clients {
		if value.HasPlayer(conn.profile.ID) {
			clients = append(clients, conn)
		}
	}
	server.clientsMutex.RUnlock()
	for _, conn := range clients {
		conn.send(TypeGameState, "", view.Build(value, conn.profile.ID, online))
	}
}

func (server *Server) gameCards(playerID game.PlayerID) []view.Card {
	sessions := server.allSessions()
	cards := make([]view.Card, 0)
	for _, session := range sessions {
		session.mutex.Lock()
		value := session.value
		if value.HasPlayer(playerID) {
			cards = append(cards, view.BuildCard(value, playerID))
		}
		session.mutex.Unlock()
	}
	sort.Slice(cards, func(i, j int) bool {
		return cards[i].UpdatedAt.After(cards[j].UpdatedAt)
	})
	return cards
}

func cloneGame(value *game.Game) (*game.Game, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var clone game.Game
	if err := json.Unmarshal(raw, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}
