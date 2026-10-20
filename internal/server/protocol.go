package server

import (
	"encoding/json"

	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/view"
)

const (
	TypePlayersSearch = "players.search"
	TypeGamesList     = "games.list"
	TypeGameCreate    = "game.create"
	TypeGameDelete    = "game.delete"
	TypeGameDeleted   = "game.deleted"
	TypeGameState     = "game.state"
	TypeMovePreview   = "move.preview"
	TypeMoveSubmit    = "move.submit"
	TypeHintGet       = "hint.get"
	TypeError         = "error"
	TypeGamesChanged  = "games.changed"
	TypeMoveAccepted  = "move.accepted"
	TypeHint          = "hint.result"
)

type Envelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type Outgoing struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Payload   any    `json:"payload,omitempty"`
}

type SearchPlayersRequest struct {
	Query  string `json:"query"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

type SearchPlayersResponse struct {
	Players    []PlayerResult `json:"players"`
	Total      int            `json:"total"`
	NextOffset int            `json:"nextOffset"`
}

type PlayerResult struct {
	ID       game.PlayerID `json:"id"`
	Username string        `json:"username"`
}

type GamesListRequest struct{}

type GamesListResponse struct {
	Games []view.Card `json:"games"`
}

type CreateGameRequest struct {
	Language      game.Language   `json:"language"`
	Level         game.Level      `json:"level"`
	MaxWordLength int             `json:"maxWordLength"`
	PlayerCount   int             `json:"playerCount"`
	Opponents     []game.PlayerID `json:"opponents"`
}

type GameStateRequest struct {
	GameID game.GameID `json:"gameId"`
}

type DeleteGameRequest struct {
	GameID game.GameID `json:"gameId"`
}

type GameDeleted struct {
	GameID game.GameID `json:"gameId"`
}

type PreviewRequest struct {
	GameID game.GameID      `json:"gameId"`
	Move   game.MoveRequest `json:"move"`
}

type SubmitRequest struct {
	GameID game.GameID      `json:"gameId"`
	Move   game.MoveRequest `json:"move"`
}

type HintRequest struct {
	GameID game.GameID `json:"gameId"`
}

type MovePreview struct {
	Score int               `json:"score"`
	Bingo bool              `json:"bingo"`
	Words []game.FormedWord `json:"words"`
}

type MoveAccepted struct {
	GameID game.GameID     `json:"gameId"`
	Move   game.MoveRecord `json:"move"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
