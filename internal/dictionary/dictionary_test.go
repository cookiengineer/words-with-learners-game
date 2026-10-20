package dictionary

import (
	"io/fs"
	"testing"

	app "github.com/cookiengineer/words-with-learners-app"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
)

func TestValidationAcceptsWordsFromOtherLevelsButHintsStayInLevel(t *testing.T) {
	root, err := fs.Sub(app.Files, "dictionaries")
	if err != nil {
		t.Fatal(err)
	}
	library, err := LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	a1, err := library.Get(game.LanguageGerman, game.LevelA1)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := library.Get(game.LanguageGerman, game.LevelA2)
	if err != nil {
		t.Fatal(err)
	}
	c1, err := library.Get(game.LanguageGerman, game.LevelC1)
	if err != nil {
		t.Fatal(err)
	}
	if !a1.ContainsWord("HÖHE") || !c1.ContainsWord("HÖHE") {
		t.Fatal("validation should accept the A2 word HÖHE at every level")
	}
	if !a1.ContainsWord("HÖHERER") || !c1.ContainsWord("HÖHERER") {
		t.Fatal("validation should accept the C2 word HÖHERER at every level")
	}
	if a1.HintFor("HÖHE") != "" || c1.HintFor("HÖHE") != "" {
		t.Fatal("hints should stay within the selected level")
	}
	if a2.HintFor("HÖHE") != "height, altitude" {
		t.Fatalf("unexpected A2 hint: %q", a2.HintFor("HÖHE"))
	}
	for _, entry := range c1.Entries() {
		if entry.Word == "HÖHE" {
			t.Fatal("C1 entries should not include the A2 word HÖHE")
		}
	}
}

func TestBlankTileFormingHigherLevelWord(t *testing.T) {
	root, err := fs.Sub(app.Files, "dictionaries")
	if err != nil {
		t.Fatal(err)
	}
	library, err := LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	dict, err := library.Get(game.LanguageGerman, game.LevelC1)
	if err != nil {
		t.Fatal(err)
	}
	value := &game.Game{
		Config: game.Config{
			Language:      game.LanguageGerman,
			Level:         game.LevelC1,
			MaxWordLength: 10,
			PlayerCount:   2,
			BoardSize:     game.BoardSize,
			BingoBonus:    50,
		},
		Status: game.StatusPlaying,
		Players: []game.Player{
			{ID: "p1", Name: "One", Rack: []game.Tile{
				{ID: "h1", Letter: "H", Value: 2},
				{ID: "o1", Letter: "Ö", Value: 8},
				{ID: "h2", Letter: "H", Value: 2},
				{ID: "blank", Letter: "", Value: 0, Blank: true},
			}},
			{ID: "p2", Name: "Two"},
		},
		Current:  0,
		Revision: 1,
	}
	placements := []game.Placement{
		{TileID: "h1", X: 6, Y: 7},
		{TileID: "o1", X: 7, Y: 7},
		{TileID: "h2", X: 8, Y: 7},
		{TileID: "blank", X: 9, Y: 7, BlankLetter: "e"},
	}
	resolved, words, _, err := game.ValidatePlacement(value, dict, placements)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 1 || words[0].Word != "HÖHE" {
		t.Fatalf("unexpected words: %#v", words)
	}
	if resolved[3].Tile.Letter != "E" || resolved[3].Tile.Value != 0 {
		t.Fatalf("unexpected blank tile: %#v", resolved[3].Tile)
	}
}
