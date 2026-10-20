package game

import (
	"errors"
	"fmt"
	"time"
)

type TileSpec struct {
	Letter string `json:"letter"`
	Value  int    `json:"value"`
	Count  int    `json:"count"`
	Blank  bool   `json:"blank"`
}

func BuildBag(specs []TileSpec) ([]Tile, error) {
	bag := make([]Tile, 0)
	index := 0
	for _, spec := range specs {
		if spec.Count < 0 {
			return nil, fmt.Errorf("negative tile count for %q", spec.Letter)
		}
		if spec.Value < 0 {
			return nil, fmt.Errorf("negative tile value for %q", spec.Letter)
		}
		if !spec.Blank {
			if spec.Letter == "" {
				return nil, errors.New("letter tile has an empty letter")
			}
		} else if spec.Letter != "" {
			return nil, errors.New("blank tile must have an empty letter")
		}
		for count := 0; count < spec.Count; count++ {
			index++
			letter := spec.Letter
			bag = append(bag, Tile{
				ID:     TileID(fmt.Sprintf("t%04d", index)),
				Letter: letter,
				Value:  spec.Value,
				Blank:  spec.Blank,
			})
		}
	}
	if len(bag) == 0 {
		return nil, errors.New("tile bag is empty")
	}
	return bag, nil
}

func ShuffleTiles(rng *RNG, tiles []Tile) {
	rng.Shuffle(len(tiles), func(i, j int) {
		tiles[i], tiles[j] = tiles[j], tiles[i]
	})
}

func DrawTiles(bag *[]Tile, count int) []Tile {
	if count <= 0 || len(*bag) == 0 {
		return nil
	}
	if count > len(*bag) {
		count = len(*bag)
	}
	start := len(*bag) - count
	drawn := append([]Tile(nil), (*bag)[start:]...)
	*bag = (*bag)[:start]
	return drawn
}

type PlayerSeed struct {
	ID   PlayerID
	Name string
}

func NewGame(id GameID, config Config, seeds []PlayerSeed, specs []TileSpec, now time.Time) (*Game, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if len(seeds) != config.PlayerCount {
		return nil, fmt.Errorf("expected %d players, got %d", config.PlayerCount, len(seeds))
	}
	seen := make(map[PlayerID]bool, len(seeds))
	players := make([]Player, 0, len(seeds))
	for _, seed := range seeds {
		if seed.ID == "" || seed.Name == "" {
			return nil, errors.New("player id and name are required")
		}
		if seen[seed.ID] {
			return nil, fmt.Errorf("duplicate player %q", seed.ID)
		}
		seen[seed.ID] = true
		players = append(players, Player{ID: seed.ID, Name: seed.Name})
	}
	bag, err := BuildBag(specs)
	if err != nil {
		return nil, err
	}
	if len(bag) < config.RackSize()*config.PlayerCount {
		return nil, errors.New("tile bag does not contain enough tiles for the configured rack size")
	}
	rng := NewRNG(config.Seed)
	ShuffleTiles(rng, bag)
	for index := range players {
		players[index].Rack = DrawTiles(&bag, config.RackSize())
		ShuffleTiles(rng, players[index].Rack)
	}
	if config.BingoBonus == 0 {
		config.BingoBonus = config.RackSize() * 5
	}
	game := &Game{
		SchemaVersion: SchemaVersion,
		ID:            id,
		Creator:       players[0].ID,
		Config:        config,
		Specs:         append([]TileSpec(nil), specs...),
		Status:        StatusPlaying,
		Players:       players,
		Current:       0,
		Board:         make([]PlacedTile, 0),
		Bag:           bag,
		Revision:      1,
		MoveCount:     0,
		Moves:         make([]MoveRecord, 0),
		CreatedAt:     now.UTC(),
		UpdatedAt:     now.UTC(),
	}
	return game, nil
}
