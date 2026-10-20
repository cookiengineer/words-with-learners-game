package server

import (
	"errors"
	"io/fs"
	"reflect"
	"strconv"
	"testing"
	"time"

	app "github.com/cookiengineer/words-with-learners-app"
	"github.com/cookiengineer/words-with-learners-app/internal/dictionary"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/view"
)

func loadHintDictionary(t *testing.T, language game.Language, level game.Level) *dictionary.Dictionary {
	t.Helper()
	root, err := fs.Sub(app.Files, "dictionaries")
	if err != nil {
		t.Fatal(err)
	}
	library, err := dictionary.LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	dict, err := library.Get(language, level)
	if err != nil {
		t.Fatal(err)
	}
	return dict
}

func TestFindHint(t *testing.T) {
	dict := loadHintDictionary(t, game.LanguageDutch, game.LevelA1)
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

func TestFindHintIncludesValidatedCrossWordPlacement(t *testing.T) {
	dict := loadHintDictionary(t, game.LanguageGerman, game.LevelA1)
	config := game.Config{
		Language:      game.LanguageGerman,
		Level:         game.LevelA1,
		MaxWordLength: 7,
		PlayerCount:   1,
		BoardSize:     game.BoardSize,
		BingoBonus:    35,
		Seed:          7,
	}
	value, err := game.NewGame("hint-cross-game", config, []game.PlayerSeed{
		{ID: "p1", Name: "One"},
	}, dict.Specs(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]int)
	for _, spec := range dict.Specs() {
		values[spec.Letter] = spec.Value
	}
	placed := make([]game.PlacedTile, 0)
	add := func(x, y int, letter string) {
		placed = append(placed, game.PlacedTile{
			Tile: game.Tile{ID: game.TileID("b" + strconv.Itoa(len(placed))), Letter: letter, Value: values[letter]},
			X:    x,
			Y:    y,
		})
	}
	add(9, 5, "H")
	add(9, 6, "Ä")
	add(9, 8, "D")
	add(9, 9, "E")
	add(12, 2, "H")
	add(12, 3, "E")
	add(12, 4, "L")
	add(12, 5, "F")
	add(12, 6, "E")
	add(7, 7, "J")
	add(9, 7, "N")
	add(10, 7, "G")
	add(11, 7, "E")
	add(12, 7, "N")
	add(7, 8, "A")
	placed = append(placed, game.PlacedTile{
		Tile: game.Tile{ID: "blank-u", Letter: "U", Value: 0, Blank: true},
		X:    8,
		Y:    7,
	})
	value.Board = placed

	rackLetters := []string{"U", "N", "D", "R", "T", "M", "D"}
	rack := make([]game.Tile, 0, len(rackLetters))
	for index, letter := range rackLetters {
		rack = append(rack, game.Tile{
			ID:     game.TileID("r" + letter + string(rune('0'+index))),
			Letter: letter,
			Value:  values[letter],
		})
	}
	value.Players[0].Rack = rack

	hint := findHint(value, dict)
	if !hint.Found {
		t.Fatal("expected a hint for TUN")
	}
	wantPlacements := []view.HintPlacement{{X: 7, Y: 10}, {X: 8, Y: 10}, {X: 9, Y: 10}}
	if !reflect.DeepEqual(hint.Placements, wantPlacements) {
		t.Fatalf("unexpected hint placements: %#v", hint.Placements)
	}

	tileID := func(letter string) game.TileID {
		for _, tile := range rack {
			if tile.Letter == letter {
				return tile.ID
			}
		}
		t.Fatalf("missing rack tile %s", letter)
		return ""
	}
	placements := []game.Placement{
		{TileID: tileID("T"), X: 7, Y: 10},
		{TileID: tileID("U"), X: 8, Y: 10},
		{TileID: tileID("N"), X: 9, Y: 10},
	}
	_, words, _, err := game.ValidatePlacementFor(value, dict, 0, placements)
	if err != nil {
		t.Fatalf("hint placement is invalid: %v", err)
	}
	foundTUN, foundCrossWord := false, false
	for _, word := range words {
		if word.Word == "TUN" {
			foundTUN = true
		}
		if word.Word == "HÄNDEN" && word.Direction == game.DirectionVertical {
			foundCrossWord = true
		}
	}
	if !foundTUN || !foundCrossWord {
		t.Fatalf("expected TUN and perpendicular HÄNDEN, got %#v", words)
	}

	invalid := []struct {
		name       string
		placements []game.Placement
		code       string
	}{
		{
			name: "and",
			placements: []game.Placement{
				{TileID: tileID("T"), X: 8, Y: 6},
				{TileID: tileID("N"), X: 8, Y: 8},
			},
			code: "unknown_word",
		},
		{
			name: "jungenut",
			placements: []game.Placement{
				{TileID: tileID("U"), X: 13, Y: 7},
				{TileID: tileID("T"), X: 14, Y: 7},
			},
			code: "word_length",
		},
		{
			name: "helfenut",
			placements: []game.Placement{
				{TileID: tileID("U"), X: 12, Y: 8},
				{TileID: tileID("T"), X: 12, Y: 9},
			},
			code: "word_length",
		},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, err := game.ValidatePlacementFor(value, dict, 0, test.placements)
			if err == nil {
				t.Fatal("expected placement to be rejected")
			}
			var ruleError *game.RuleError
			if !errors.As(err, &ruleError) || ruleError.Code != test.code {
				t.Fatalf("expected %s, got %v", test.code, err)
			}
		})
	}
}
