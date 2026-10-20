package game

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SchemaVersion = 1
	BoardSize     = 15
	MinRackSize   = 5
	MaxRackSize   = 15
	MinPlayers    = 2
	MaxPlayers    = 4
)

type Language string

const (
	LanguageDutch   Language = "nl"
	LanguageEnglish Language = "en"
	LanguageGerman  Language = "de"
	LanguageSpanish Language = "es"
	LanguageFrench  Language = "fr"
)

var Languages = []Language{
	LanguageDutch,
	LanguageEnglish,
	LanguageGerman,
	LanguageSpanish,
	LanguageFrench,
}

func (language Language) Valid() bool {
	switch language {
	case LanguageDutch, LanguageEnglish, LanguageGerman, LanguageSpanish, LanguageFrench:
		return true
	default:
		return false
	}
}

func (language Language) Name() string {
	switch language {
	case LanguageDutch:
		return "Dutch"
	case LanguageEnglish:
		return "English"
	case LanguageGerman:
		return "German"
	case LanguageSpanish:
		return "Spanish"
	case LanguageFrench:
		return "French"
	default:
		return string(language)
	}
}

type Level string

const (
	LevelA1 Level = "A1"
	LevelA2 Level = "A2"
	LevelB1 Level = "B1"
	LevelB2 Level = "B2"
	LevelC1 Level = "C1"
	LevelC2 Level = "C2"
)

var Levels = []Level{LevelA1, LevelA2, LevelB1, LevelB2, LevelC1, LevelC2}

func (level Level) Valid() bool {
	switch level {
	case LevelA1, LevelA2, LevelB1, LevelB2, LevelC1, LevelC2:
		return true
	default:
		return false
	}
}

type PlayerID string
type GameID string
type TileID string

type Bonus string

const (
	BonusNone  Bonus = "none"
	BonusDL    Bonus = "dl"
	BonusTL    Bonus = "tl"
	BonusDW    Bonus = "dw"
	BonusTW    Bonus = "tw"
	BonusStart Bonus = "start"
)

func (bonus Bonus) Valid() bool {
	switch bonus {
	case BonusNone, BonusDL, BonusTL, BonusDW, BonusTW, BonusStart:
		return true
	default:
		return false
	}
}

type Direction string

const (
	DirectionHorizontal Direction = "horizontal"
	DirectionVertical   Direction = "vertical"
)

type Coord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (coord Coord) Valid() bool {
	return coord.X >= 0 && coord.X < BoardSize && coord.Y >= 0 && coord.Y < BoardSize
}

func (coord Coord) Key() string {
	return fmt.Sprintf("%d:%d", coord.X, coord.Y)
}

type Tile struct {
	ID     TileID `json:"id"`
	Letter string `json:"letter"`
	Value  int    `json:"value"`
	Blank  bool   `json:"blank"`
}

func (tile Tile) IsBlankTile() bool {
	return tile.Blank && tile.Letter == ""
}

func (tile Tile) EffectiveLetter() string {
	return tile.Letter
}

type PlacedTile struct {
	Tile Tile `json:"tile"`
	X    int  `json:"x"`
	Y    int  `json:"y"`
}

func (placed PlacedTile) Coord() Coord {
	return Coord{X: placed.X, Y: placed.Y}
}

type Config struct {
	Language      Language `json:"language"`
	Level         Level    `json:"level"`
	MaxWordLength int      `json:"maxWordLength"`
	PlayerCount   int      `json:"playerCount"`
	BoardSize     int      `json:"boardSize"`
	BingoBonus    int      `json:"bingoBonus"`
	Seed          int64    `json:"seed"`
}

func (config Config) Validate() error {
	if !config.Language.Valid() {
		return fmt.Errorf("unsupported language %q", config.Language)
	}
	if !config.Level.Valid() {
		return fmt.Errorf("unsupported level %q", config.Level)
	}
	if config.MaxWordLength < MinRackSize || config.MaxWordLength > MaxRackSize {
		return fmt.Errorf("max word length must be between %d and %d", MinRackSize, MaxRackSize)
	}
	if config.PlayerCount < MinPlayers || config.PlayerCount > MaxPlayers {
		return fmt.Errorf("player count must be between %d and %d", MinPlayers, MaxPlayers)
	}
	if config.BoardSize != BoardSize {
		return fmt.Errorf("board size must be %d", BoardSize)
	}
	if config.BingoBonus < 0 {
		return errors.New("bingo bonus cannot be negative")
	}
	return nil
}

func (config Config) RackSize() int {
	return config.MaxWordLength
}

type Status string

const (
	StatusPlaying  Status = "playing"
	StatusFinished Status = "finished"
)

type Player struct {
	ID    PlayerID `json:"id"`
	Name  string   `json:"name"`
	Score int      `json:"score"`
	Rack  []Tile   `json:"rack"`
}

type Game struct {
	SchemaVersion int          `json:"schemaVersion"`
	ID            GameID       `json:"id"`
	Config        Config       `json:"config"`
	Specs         []TileSpec   `json:"tileSpecs"`
	Status        Status       `json:"status"`
	Players       []Player     `json:"players"`
	Current       int          `json:"current"`
	Board         []PlacedTile `json:"board"`
	Bag           []Tile       `json:"bag"`
	Revision      uint64       `json:"revision"`
	MoveCount     int          `json:"moveCount"`
	Scoreless     int          `json:"scoreless"`
	Winner        *PlayerID    `json:"winner,omitempty"`
	Moves         []MoveRecord `json:"moves"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

func (game *Game) CurrentPlayer() *Player {
	if game == nil || game.Current < 0 || game.Current >= len(game.Players) {
		return nil
	}
	return &game.Players[game.Current]
}

func (game *Game) PlayerByID(id PlayerID) *Player {
	for index := range game.Players {
		if game.Players[index].ID == id {
			return &game.Players[index]
		}
	}
	return nil
}

func (game *Game) PlayerIndexByID(id PlayerID) int {
	for index := range game.Players {
		if game.Players[index].ID == id {
			return index
		}
	}
	return -1
}

func (game *Game) HasPlayer(id PlayerID) bool {
	return game.PlayerIndexByID(id) >= 0
}

type MoveKind string

const (
	MovePlace    MoveKind = "place"
	MovePass     MoveKind = "pass"
	MoveExchange MoveKind = "exchange"
)

func (kind MoveKind) Valid() bool {
	switch kind {
	case MovePlace, MovePass, MoveExchange:
		return true
	default:
		return false
	}
}

type Placement struct {
	TileID      TileID `json:"tileId"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	BlankLetter string `json:"blankLetter,omitempty"`
}

type MoveRequest struct {
	MoveID           string      `json:"moveId"`
	ExpectedRevision uint64      `json:"expectedRevision"`
	Kind             MoveKind    `json:"kind"`
	Placements       []Placement `json:"placements,omitempty"`
	Exchange         []TileID    `json:"exchange,omitempty"`
}

func (move MoveRequest) Validate() error {
	if strings.TrimSpace(move.MoveID) == "" {
		return errors.New("move id is required")
	}
	if !move.Kind.Valid() {
		return fmt.Errorf("unsupported move kind %q", move.Kind)
	}
	switch move.Kind {
	case MovePlace:
		if len(move.Placements) == 0 {
			return errors.New("a placement move needs at least one tile")
		}
		if len(move.Exchange) > 0 {
			return errors.New("a placement move cannot exchange tiles")
		}
	case MovePass:
		if len(move.Placements) > 0 || len(move.Exchange) > 0 {
			return errors.New("a pass cannot place or exchange tiles")
		}
	case MoveExchange:
		if len(move.Exchange) == 0 {
			return errors.New("an exchange needs at least one tile")
		}
		if len(move.Placements) > 0 {
			return errors.New("an exchange cannot place tiles")
		}
	}
	return nil
}

type ResolvedPlacement struct {
	Tile Tile `json:"tile"`
	X    int  `json:"x"`
	Y    int  `json:"y"`
}

type FormedWord struct {
	X         int       `json:"x"`
	Y         int       `json:"y"`
	Direction Direction `json:"direction"`
	Word      string    `json:"word"`
	Score     int       `json:"score"`
	Primary   bool      `json:"primary"`
}

type MoveRecord struct {
	MoveID      string              `json:"moveId"`
	Seq         int                 `json:"seq"`
	PlayerID    PlayerID            `json:"playerId"`
	Kind        MoveKind            `json:"kind"`
	Placements  []ResolvedPlacement `json:"placements,omitempty"`
	Exchange    []TileID            `json:"exchange,omitempty"`
	Words       []FormedWord        `json:"words,omitempty"`
	Score       int                 `json:"score"`
	ScoresAfter []int               `json:"scoresAfter"`
	Drawn       []TileID            `json:"drawn,omitempty"`
	BagCount    int                 `json:"bagCount"`
	BoardHash   string              `json:"boardHash"`
	At          time.Time           `json:"at"`
}

type RuleError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (err *RuleError) Error() string {
	return err.Message
}

func RuleErrorf(code string, format string, args ...any) *RuleError {
	return &RuleError{Code: code, Message: fmt.Sprintf(format, args...)}
}
