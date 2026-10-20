package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cookiengineer/words-with-learners-app/internal/config"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/view"
	"golang.org/x/net/websocket"
)

type testServer struct {
	app  *Server
	http *httptest.Server
}

type wsOutgoing struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Payload   any    `json:"payload,omitempty"`
}

type wsIncoming struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

var requestCounter atomic.Int64

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	app, err := New(config.Config{DataDir: t.TempDir(), Dev: false})
	if err != nil {
		t.Fatal(err)
	}
	return &testServer{app: app, http: httptest.NewServer(app.Handler())}
}

func (server *testServer) register(t *testing.T, username string) (ProfileResponse, string) {
	t.Helper()
	body := fmt.Sprintf(`{"username":%q,"password":"password123"}`, username)
	response, err := http.Post(server.http.URL+"/api/auth/login", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", response.StatusCode)
	}
	var profile ProfileResponse
	if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
		t.Fatal(err)
	}
	cookies := response.Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie")
	}
	return profile, cookies[0].String()
}

func (server *testServer) dial(t *testing.T, cookie string) *websocket.Conn {
	t.Helper()
	wsURL := strings.Replace(server.http.URL, "http://", "ws://", 1) + "/ws"
	dialConfig, err := websocket.NewConfig(wsURL, server.http.URL)
	if err != nil {
		t.Fatal(err)
	}
	dialConfig.Header.Set("Cookie", cookie)
	conn, err := websocket.DialConfig(dialConfig)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	return conn
}

func wsRequest(t *testing.T, conn *websocket.Conn, messageType string, payload any, target any) {
	t.Helper()
	requestID := fmt.Sprintf("req-%d", requestCounter.Add(1))
	if err := websocket.JSON.Send(conn, wsOutgoing{Type: messageType, RequestID: requestID, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	for {
		var incoming wsIncoming
		if err := websocket.JSON.Receive(conn, &incoming); err != nil {
			t.Fatal(err)
		}
		if incoming.RequestID != requestID {
			continue
		}
		if incoming.Type == TypeError {
			var failure ErrorPayload
			_ = json.Unmarshal(incoming.Payload, &failure)
			t.Fatalf("%s failed: %s (%s)", messageType, failure.Message, failure.Code)
		}
		if target != nil {
			if err := json.Unmarshal(incoming.Payload, target); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
}

func wsExpectError(t *testing.T, conn *websocket.Conn, messageType string, payload any, code string) {
	t.Helper()
	requestID := fmt.Sprintf("req-%d", requestCounter.Add(1))
	if err := websocket.JSON.Send(conn, wsOutgoing{Type: messageType, RequestID: requestID, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	for {
		var incoming wsIncoming
		if err := websocket.JSON.Receive(conn, &incoming); err != nil {
			t.Fatal(err)
		}
		if incoming.RequestID != requestID {
			continue
		}
		if incoming.Type != TypeError {
			t.Fatalf("expected error, got %s", incoming.Type)
		}
		var failure ErrorPayload
		if err := json.Unmarshal(incoming.Payload, &failure); err != nil {
			t.Fatal(err)
		}
		if failure.Code != code {
			t.Fatalf("error code = %s, want %s", failure.Code, code)
		}
		return
	}
}

func TestLoginOrRegister(t *testing.T) {
	server := newTestServer(t)
	defer server.http.Close()

	login := func(username string, password string) (*http.Response, ProfileResponse) {
		t.Helper()
		body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
		response, err := http.Post(server.http.URL+"/api/auth/login", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		var profile ProfileResponse
		if response.StatusCode == http.StatusOK {
			if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
				t.Fatal(err)
			}
		}
		return response, profile
	}

	created, createdProfile := login("alice", "password123")
	created.Body.Close()
	if created.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d", created.StatusCode)
	}

	loggedIn, loggedInProfile := login("alice", "password123")
	loggedIn.Body.Close()
	if loggedIn.StatusCode != http.StatusOK || loggedInProfile.ID != createdProfile.ID {
		t.Fatalf("login status = %d, profile = %#v", loggedIn.StatusCode, loggedInProfile)
	}

	wrong, _ := login("alice", "wrong-password")
	wrong.Body.Close()
	if wrong.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d", wrong.StatusCode)
	}
}

func TestWebSocketGameFlow(t *testing.T) {
	server := newTestServer(t)
	defer server.http.Close()
	host, hostCookie := server.register(t, "host")
	opponent, opponentCookie := server.register(t, "guest")
	hostConn := server.dial(t, hostCookie)
	defer hostConn.Close()
	opponentConn := server.dial(t, opponentCookie)
	defer opponentConn.Close()

	var created view.Game
	wsRequest(t, hostConn, TypeGameCreate, CreateGameRequest{
		Language:      game.LanguageEnglish,
		Level:         game.LevelA1,
		MaxWordLength: 7,
		PlayerCount:   2,
		Opponents:     []game.PlayerID{opponent.ID},
	}, &created)
	if created.ID == "" || created.Revision != 1 {
		t.Fatalf("unexpected created game: %#v", created)
	}

	session, ok := server.app.session(created.ID)
	if !ok {
		t.Fatal("game session missing")
	}
	session.mutex.Lock()
	player := session.value.CurrentPlayer()
	player.Rack = []game.Tile{
		{ID: "h1", Letter: "H", Value: 4},
		{ID: "a1", Letter: "A", Value: 1},
		{ID: "p1", Letter: "P", Value: 3},
		{ID: "p2", Letter: "P", Value: 3},
		{ID: "y1", Letter: "Y", Value: 4},
		{ID: "x1", Letter: "X", Value: 8},
		{ID: "z1", Letter: "Z", Value: 10},
	}
	opponentPlayer := &session.value.Players[1]
	opponentPlayer.Rack = []game.Tile{
		{ID: "o1", Letter: "X", Value: 8},
		{ID: "o2", Letter: "X", Value: 8},
		{ID: "o3", Letter: "X", Value: 8},
		{ID: "o4", Letter: "X", Value: 8},
		{ID: "o5", Letter: "X", Value: 8},
		{ID: "o6", Letter: "X", Value: 8},
		{ID: "o7", Letter: "X", Value: 8},
	}
	session.mutex.Unlock()

	placements := []game.Placement{
		{TileID: "h1", X: 5, Y: 7},
		{TileID: "a1", X: 6, Y: 7},
		{TileID: "p1", X: 7, Y: 7},
		{TileID: "p2", X: 8, Y: 7},
		{TileID: "y1", X: 9, Y: 7},
	}
	move := game.MoveRequest{
		MoveID:           "move-1",
		ExpectedRevision: created.Revision,
		Kind:             game.MovePlace,
		Placements:       placements,
	}
	var accepted MoveAccepted
	wsRequest(t, hostConn, TypeMoveSubmit, SubmitRequest{GameID: created.ID, Move: move}, &accepted)
	if accepted.Move.Score != 15 {
		t.Fatalf("score = %d, want 15", accepted.Move.Score)
	}

	var opponentState view.Game
	wsRequest(t, opponentConn, TypeGameState, GameStateRequest{GameID: created.ID}, &opponentState)
	if opponentState.Revision != 2 {
		t.Fatalf("revision = %d, want 2", opponentState.Revision)
	}
	if !opponentState.IsYourTurn {
		t.Fatal("opponent should be on turn")
	}
	if len(opponentState.Rack) != 7 {
		t.Fatalf("opponent rack length = %d, want 7", len(opponentState.Rack))
	}
	if opponentState.Players[0].ID != host.ID {
		t.Fatal("host seat missing")
	}
	if opponentState.AutoReshuffle == nil || opponentState.AutoReshuffle.PlayerID != opponent.ID {
		t.Fatalf("opponent was not auto-reshuffled: %#v", opponentState.AutoReshuffle)
	}
	if opponentState.AutoReshuffle.Revision != opponentState.Revision {
		t.Fatalf("reshuffle revision = %d, state revision = %d", opponentState.AutoReshuffle.Revision, opponentState.Revision)
	}
	dict, err := server.app.dictionaries.Get(created.Config.Language, created.Config.Level)
	if err != nil {
		t.Fatal(err)
	}
	session.mutex.Lock()
	playable := findHintFor(session.value, dict, session.value.Current, true).Found
	session.mutex.Unlock()
	if !playable {
		t.Fatal("reshuffled opponent still has no playable word")
	}
	wsExpectError(t, opponentConn, TypeMoveSubmit, SubmitRequest{GameID: created.ID, Move: game.MoveRequest{
		MoveID:           "move-exchange",
		ExpectedRevision: 2,
		Kind:             game.MoveExchange,
		Exchange:         []game.TileID{"o1"},
	}}, "exchange_removed")

	wsExpectError(t, hostConn, TypeMoveSubmit, SubmitRequest{GameID: created.ID, Move: game.MoveRequest{
		MoveID:           "move-stale",
		ExpectedRevision: 1,
		Kind:             game.MovePass,
	}}, "stale_revision")

	session.mutex.Lock()
	foreignTile := session.value.Players[0].Rack[0].ID
	session.mutex.Unlock()
	wsExpectError(t, opponentConn, TypeMoveSubmit, SubmitRequest{GameID: created.ID, Move: game.MoveRequest{
		MoveID:           "move-foreign",
		ExpectedRevision: 2,
		Kind:             game.MovePlace,
		Placements:       []game.Placement{{TileID: foreignTile, X: 0, Y: 0}},
	}}, "foreign_tile")

	var hint view.Hint
	wsRequest(t, opponentConn, TypeHintGet, HintRequest{GameID: created.ID}, &hint)

	var list GamesListResponse
	wsRequest(t, opponentConn, TypeGamesList, GamesListRequest{}, &list)
	if len(list.Games) != 1 || list.Games[0].ID != created.ID {
		t.Fatalf("game list = %#v", list.Games)
	}

	var search SearchPlayersResponse
	wsRequest(t, hostConn, TypePlayersSearch, SearchPlayersRequest{Query: "guest"}, &search)
	if len(search.Players) != 1 || search.Players[0].ID != opponent.ID {
		t.Fatalf("search = %#v", search.Players)
	}
}
