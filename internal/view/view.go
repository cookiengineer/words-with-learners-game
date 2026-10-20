package view

import (
	"time"

	"github.com/cookiengineer/words-with-learners-app/internal/game"
)

type Config struct {
	Language      game.Language `json:"language"`
	Level         game.Level    `json:"level"`
	MaxWordLength int           `json:"maxWordLength"`
	PlayerCount   int           `json:"playerCount"`
	BoardSize     int           `json:"boardSize"`
	BingoBonus    int           `json:"bingoBonus"`
}

func ConfigFrom(config game.Config) Config {
	return Config{
		Language:      config.Language,
		Level:         config.Level,
		MaxWordLength: config.MaxWordLength,
		PlayerCount:   config.PlayerCount,
		BoardSize:     config.BoardSize,
		BingoBonus:    config.BingoBonus,
	}
}

type Tile struct {
	ID     game.TileID `json:"id"`
	Letter string      `json:"letter"`
	Value  int         `json:"value"`
	Blank  bool        `json:"blank"`
}

func TileFrom(tile game.Tile) Tile {
	return Tile{ID: tile.ID, Letter: tile.Letter, Value: tile.Value, Blank: tile.Blank}
}

type PlacedTile struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Letter string `json:"letter"`
	Value  int    `json:"value"`
	Blank  bool   `json:"blank"`
}

type Player struct {
	ID     game.PlayerID `json:"id"`
	Name   string        `json:"name"`
	Score  int           `json:"score"`
	Seat   int           `json:"seat"`
	Tiles  int           `json:"tiles"`
	IsTurn bool          `json:"isTurn"`
	IsYou  bool          `json:"isYou"`
	Online bool          `json:"online"`
}

type Game struct {
	ID            game.GameID         `json:"id"`
	Config        Config              `json:"config"`
	Status        game.Status         `json:"status"`
	Board         []PlacedTile        `json:"board"`
	Bonuses       [][]game.Bonus      `json:"bonuses"`
	Players       []Player            `json:"players"`
	CurrentPlayer game.PlayerID       `json:"currentPlayerId"`
	Revision      uint64              `json:"revision"`
	MoveCount     int                 `json:"moveCount"`
	BagCount      int                 `json:"bagCount"`
	Rack          []Tile              `json:"rack"`
	IsYourTurn    bool                `json:"isYourTurn"`
	LastMove      *game.MoveRecord    `json:"lastMove,omitempty"`
	AutoReshuffle *game.AutoReshuffle `json:"autoReshuffle,omitempty"`
	Winner        *game.PlayerID      `json:"winner,omitempty"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
}

type CardPlayer struct {
	ID    game.PlayerID `json:"id"`
	Name  string        `json:"name"`
	Score int           `json:"score"`
}

type Card struct {
	ID            game.GameID    `json:"id"`
	Config        Config         `json:"config"`
	Status        game.Status    `json:"status"`
	Players       []CardPlayer   `json:"players"`
	CurrentPlayer game.PlayerID  `json:"currentPlayerId"`
	YourTurn      bool           `json:"yourTurn"`
	Winner        *game.PlayerID `json:"winner,omitempty"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

type Hint struct {
	Found  bool   `json:"found"`
	Hint   string `json:"hint"`
	Length int    `json:"length"`
	Score  int    `json:"score"`
}

func Build(value *game.Game, viewer game.PlayerID, online map[game.PlayerID]bool) Game {
	viewerIndex := value.PlayerIndexByID(viewer)
	layout := game.WWFLayout()
	bonuses := make([][]game.Bonus, game.BoardSize)
	for y := 0; y < game.BoardSize; y++ {
		bonuses[y] = make([]game.Bonus, game.BoardSize)
		for x := 0; x < game.BoardSize; x++ {
			bonuses[y][x] = layout[y][x]
		}
	}
	players := make([]Player, len(value.Players))
	for index := range value.Players {
		player := &value.Players[index]
		players[index] = Player{
			ID:     player.ID,
			Name:   player.Name,
			Score:  player.Score,
			Seat:   index,
			Tiles:  len(player.Rack),
			IsTurn: index == value.Current,
			IsYou:  player.ID == viewer,
			Online: online[player.ID],
		}
	}
	board := make([]PlacedTile, len(value.Board))
	for index, placed := range value.Board {
		board[index] = PlacedTile{
			X:      placed.X,
			Y:      placed.Y,
			Letter: placed.Tile.Letter,
			Value:  placed.Tile.Value,
			Blank:  placed.Tile.Blank,
		}
	}
	rack := make([]Tile, 0)
	if viewerIndex >= 0 {
		for _, tile := range value.Players[viewerIndex].Rack {
			rack = append(rack, TileFrom(tile))
		}
	}
	lastMove := (*game.MoveRecord)(nil)
	if len(value.Moves) > 0 {
		last := value.Moves[len(value.Moves)-1]
		lastMove = &last
	}
	currentPlayer := game.PlayerID("")
	if current := value.CurrentPlayer(); current != nil {
		currentPlayer = current.ID
	}
	var winner *game.PlayerID
	if value.Winner != nil {
		copyWinner := *value.Winner
		winner = &copyWinner
	}
	var reshuffle *game.AutoReshuffle
	if value.LastReshuffle != nil {
		copyReshuffle := *value.LastReshuffle
		copyReshuffle.Returned = append([]game.TileID(nil), value.LastReshuffle.Returned...)
		copyReshuffle.Drawn = append([]game.TileID(nil), value.LastReshuffle.Drawn...)
		reshuffle = &copyReshuffle
	}
	return Game{
		ID:            value.ID,
		Config:        ConfigFrom(value.Config),
		Status:        value.Status,
		Board:         board,
		Bonuses:       bonuses,
		Players:       players,
		CurrentPlayer: currentPlayer,
		Revision:      value.Revision,
		MoveCount:     value.MoveCount,
		BagCount:      len(value.Bag),
		Rack:          rack,
		IsYourTurn:    viewerIndex >= 0 && viewerIndex == value.Current && value.Status == game.StatusPlaying,
		LastMove:      lastMove,
		AutoReshuffle: reshuffle,
		Winner:        winner,
		CreatedAt:     value.CreatedAt,
		UpdatedAt:     value.UpdatedAt,
	}
}

func BuildCard(value *game.Game, viewer game.PlayerID) Card {
	players := make([]CardPlayer, len(value.Players))
	for index := range value.Players {
		player := &value.Players[index]
		players[index] = CardPlayer{ID: player.ID, Name: player.Name, Score: player.Score}
	}
	current := game.PlayerID("")
	if value.CurrentPlayer() != nil {
		current = value.CurrentPlayer().ID
	}
	var winner *game.PlayerID
	if value.Winner != nil {
		copyWinner := *value.Winner
		winner = &copyWinner
	}
	return Card{
		ID:            value.ID,
		Config:        ConfigFrom(value.Config),
		Status:        value.Status,
		Players:       players,
		CurrentPlayer: current,
		YourTurn:      value.Status == game.StatusPlaying && value.CurrentPlayer() != nil && value.CurrentPlayer().ID == viewer,
		Winner:        winner,
		UpdatedAt:     value.UpdatedAt,
	}
}
