package dictionary

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf8"

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

func TestSupplementWordsArePlayableAtEveryLevel(t *testing.T) {
	root, err := fs.Sub(app.Files, "dictionaries")
	if err != nil {
		t.Fatal(err)
	}
	library, err := LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range game.Languages {
		seen := make(map[string]string)
		for _, kind := range supplementKinds {
			supplementPath := string(language) + "/" + kind + ".json"
			raw, err := fs.ReadFile(root, supplementPath)
			if err != nil {
				t.Fatal(err)
			}
			var supplement SupplementFile
			if err := json.Unmarshal(raw, &supplement); err != nil {
				t.Fatal(err)
			}
			if supplement.Language != language || supplement.Kind != kind {
				t.Fatalf("%s declares %s %s", supplementPath, supplement.Language, supplement.Kind)
			}
			if len(supplement.Words) == 0 {
				t.Fatalf("%s is empty", supplementPath)
			}
			for _, entry := range supplement.Words {
				word := strings.ToUpper(strings.TrimSpace(entry.Word))
				if utf8.RuneCountInString(word) < 2 || utf8.RuneCountInString(word) > game.MaxRackSize {
					t.Fatalf("%s: invalid word length for %q", supplementPath, entry.Word)
				}
				if strings.TrimSpace(entry.Hint) == "" {
					t.Fatalf("%s: missing hint for %q", supplementPath, entry.Word)
				}
				if previous, exists := seen[word]; exists {
					t.Fatalf("%s: duplicate supplement word %q (already in %s)", supplementPath, entry.Word, previous)
				}
				seen[word] = supplementPath
				for _, level := range game.Levels {
					dict, err := library.Get(language, level)
					if err != nil {
						t.Fatal(err)
					}
					if !dict.ContainsWord(word) {
						t.Fatalf("%s %s does not accept supplement word %q", language, level, word)
					}
				}
			}
		}
	}
}

func TestSupplementWordsProvideHints(t *testing.T) {
	root, err := fs.Sub(app.Files, "dictionaries")
	if err != nil {
		t.Fatal(err)
	}
	library, err := LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		language game.Language
		word     string
	}{
		{game.LanguageDutch, "ROTTERDAM"},
		{game.LanguageEnglish, "STONEHENGE"},
		{game.LanguageGerman, "ZUGSPITZE"},
		{game.LanguageSpanish, "TEIDE"},
		{game.LanguageFrench, "VERSAILLES"},
		{game.LanguageItalian, "ETNA"},
		{game.LanguageRussian, "БАЙКАЛ"},
	}
	for _, testCase := range cases {
		dict, err := library.Get(testCase.language, game.LevelA1)
		if err != nil {
			t.Fatal(err)
		}
		if !dict.ContainsWord(testCase.word) {
			t.Fatalf("%s A1 does not accept %q", testCase.language, testCase.word)
		}
		if dict.HintFor(testCase.word) == "" {
			t.Fatalf("%s A1 has no hint for %q", testCase.language, testCase.word)
		}
		found := false
		for _, entry := range dict.Entries() {
			if entry.Word == testCase.word {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s A1 entries do not include %q", testCase.language, testCase.word)
		}
	}
}

func TestLoadSupplementsRejectsDuplicates(t *testing.T) {
	files := fstest.MapFS{
		"nl/cities.json": &fstest.MapFile{Data: []byte(`{
			"schemaVersion": 1,
			"language": "nl",
			"kind": "cities",
			"words": [{"word": "AMSTERDAM", "hint": "city"}]
		}`)},
		"nl/sights.json": &fstest.MapFile{Data: []byte(`{
			"schemaVersion": 1,
			"language": "nl",
			"kind": "sights",
			"words": [{"word": "AMSTERDAM", "hint": "city"}]
		}`)},
	}
	letters := make(map[string]bool)
	for _, letter := range "AMSTERDAM" {
		letters[string(letter)] = true
	}
	if _, err := loadSupplements(files, game.LanguageDutch, letters); err == nil {
		t.Fatal("expected duplicate supplement words to be rejected")
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
