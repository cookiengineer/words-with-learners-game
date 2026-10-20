package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const CookieName = "wwl_session"

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidUsername    = errors.New("username must be 3-32 characters using letters, numbers, _ or -")
	ErrInvalidPassword    = errors.New("password must be at least 8 characters")
	usernamePattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)
)

type Session struct {
	PlayerID  game.PlayerID
	ExpiresAt time.Time
}

type Manager struct {
	store        *store.Store
	mutex        sync.RWMutex
	saveMutex    sync.Mutex
	sessions     map[string]Session
	ttl          time.Duration
	sessionsPath string
}

type persistedSession struct {
	TokenHash string        `json:"tokenHash"`
	PlayerID  game.PlayerID `json:"playerId"`
	ExpiresAt time.Time     `json:"expiresAt"`
}

type sessionFile struct {
	SchemaVersion int                `json:"schemaVersion"`
	Sessions      []persistedSession `json:"sessions"`
}

func NewManager(profiles *store.Store) (*Manager, error) {
	manager := &Manager{
		store:        profiles,
		sessions:     make(map[string]Session),
		ttl:          30 * 24 * time.Hour,
		sessionsPath: filepath.Join(profiles.Root(), "sessions.json"),
	}
	if err := manager.loadSessions(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (manager *Manager) loadSessions() error {
	raw, err := os.ReadFile(manager.sessionsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var stored sessionFile
	if err := json.Unmarshal(raw, &stored); err != nil {
		return fmt.Errorf("parse sessions: %w", err)
	}
	now := time.Now()
	for _, session := range stored.Sessions {
		if session.TokenHash == "" || session.PlayerID == "" || now.After(session.ExpiresAt) {
			continue
		}
		manager.sessions[session.TokenHash] = Session{PlayerID: session.PlayerID, ExpiresAt: session.ExpiresAt}
	}
	return nil
}

func (manager *Manager) saveSessions() error {
	manager.saveMutex.Lock()
	defer manager.saveMutex.Unlock()
	manager.mutex.RLock()
	stored := sessionFile{SchemaVersion: 1, Sessions: make([]persistedSession, 0, len(manager.sessions))}
	for hash, session := range manager.sessions {
		stored.Sessions = append(stored.Sessions, persistedSession{TokenHash: hash, PlayerID: session.PlayerID, ExpiresAt: session.ExpiresAt})
	}
	manager.mutex.RUnlock()
	raw, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(manager.sessionsPath)
	temp, err := os.CreateTemp(dir, ".sessions-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(raw, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, manager.sessionsPath)
}

func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (manager *Manager) LoginOrRegister(username string, password string, now time.Time) (*store.Profile, string, error) {
	username = strings.TrimSpace(username)
	profile, token, err := manager.Login(username, password, now)
	if err == nil {
		return profile, token, nil
	}
	if !errors.Is(err, ErrInvalidCredentials) {
		return nil, "", err
	}
	if _, ok := manager.store.ProfileByUsername(username); ok {
		return nil, "", ErrInvalidCredentials
	}
	if !usernamePattern.MatchString(username) {
		return nil, "", ErrInvalidUsername
	}
	if len(password) < 8 || len(password) > 128 {
		return nil, "", ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}
	profile, err = manager.store.CreateProfile(username, hash, now)
	if err != nil {
		if errors.Is(err, store.ErrUsernameTaken) {
			return manager.Login(username, password, now)
		}
		return nil, "", err
	}
	token, err = manager.createSession(profile.ID, now)
	if err != nil {
		return nil, "", err
	}
	profile.PasswordHash = nil
	return profile, token, nil
}

func (manager *Manager) Login(username string, password string, now time.Time) (*store.Profile, string, error) {
	profile, ok := manager.store.ProfileByUsername(username)
	if !ok {
		return nil, "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword(profile.PasswordHash, []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}
	token, err := manager.createSession(profile.ID, now)
	if err != nil {
		return nil, "", err
	}
	profile.PasswordHash = nil
	return profile, token, nil
}

func (manager *Manager) createSession(playerID game.PlayerID, now time.Time) (string, error) {
	var buffer [32]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buffer[:])
	manager.mutex.Lock()
	manager.sessions[sessionKey(token)] = Session{PlayerID: playerID, ExpiresAt: now.Add(manager.ttl)}
	manager.mutex.Unlock()
	if err := manager.saveSessions(); err != nil {
		manager.mutex.Lock()
		delete(manager.sessions, sessionKey(token))
		manager.mutex.Unlock()
		return "", err
	}
	return token, nil
}

func (manager *Manager) Authenticate(token string, now time.Time) (game.PlayerID, bool) {
	if token == "" {
		return "", false
	}
	key := sessionKey(token)
	manager.mutex.RLock()
	session, ok := manager.sessions[key]
	manager.mutex.RUnlock()
	if !ok {
		return "", false
	}
	if now.After(session.ExpiresAt) {
		manager.Logout(token)
		return "", false
	}
	return session.PlayerID, true
}

func (manager *Manager) Logout(token string) {
	manager.mutex.Lock()
	delete(manager.sessions, sessionKey(token))
	manager.mutex.Unlock()
	_ = manager.saveSessions()
}

func (manager *Manager) TokenFromRequest(request *http.Request) string {
	cookie, err := request.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (manager *Manager) SetCookie(response http.ResponseWriter, request *http.Request, token string) {
	http.SetCookie(response, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   request.TLS != nil,
		MaxAge:   int(manager.ttl.Seconds()),
	})
}

func (manager *Manager) ClearCookie(response http.ResponseWriter, request *http.Request) {
	http.SetCookie(response, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   request.TLS != nil,
		MaxAge:   -1,
	})
}

func (manager *Manager) Profile(playerID game.PlayerID) (*store.Profile, bool) {
	return manager.store.Profile(playerID)
}
