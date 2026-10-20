package server

import (
	"io/fs"
	"testing"
	"time"

	app "github.com/cookiengineer/words-with-learners-app"
	"github.com/cookiengineer/words-with-learners-app/internal/dictionary"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
)

func TestFindHint(t *testing.T) {
	root, err := fs.Sub(app.Files, "dictionaries")
	if err != nil {
		t.Fatal(err)
	}
	library, err := dictionary.LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	dict, err := library.Get(game.LanguageDutch, game.LevelA1)
	if err != nil {
		t.Fatal(err)
	}
	config := game.Config{
		Language:      game.LanguageDutch,
		Level:         game.LevelA1,
		MaxWordLength: 7,
		PlayerCount:   2,
		BoardSize:     game.BoardSize,
		BingoBonus:    35,
		Seed:          7,
	}
	value, err := game.NewGame("hint-game", config, []game.PlayerSeed{
		{ID: "p1", Name: "One"},
		{ID: "p2", Name: "Two"},
	}, dict.Specs(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	value.Players[0].Rack = []game.Tile{
		{ID: "h", Letter: "H", Value: 4},
		{ID: "u", Letter: "U", Value: 2},
		{ID: "i", Letter: "I", Value: 1},
		{ID: "s", Letter: "S", Value: 2},
		{ID: "a", Letter: "A", Value: 1},
		{ID: "e", Letter: "E", Value: 1},
		{ID: "n", Letter: "N", Value: 1},
	}
	hint := findHint(value, dict)
	if !hint.Found {
		t.Fatal("expected a hint")
	}
	if hint.Hint == "" || hint.Length < 2 {
		t.Fatalf("unexpected hint: %#v", hint)
	}
}
