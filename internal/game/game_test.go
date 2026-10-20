package game

import (
	"strings"
	"testing"
	"time"
)

type testLexicon struct {
	words   map[string]bool
	letters map[string]bool
}

func newTestLexicon(words []string) *testLexicon {
	lex := &testLexicon{words: map[string]bool{}, letters: map[string]bool{}}
	for _, word := range words {
		lex.words[word] = true
		for _, letter := range word {
			lex.letters[string(letter)] = true
		}
	}
	return lex
}

func (lex *testLexicon) NormalizeLetter(letter string) string {
	return strings.ToUpper(strings.TrimSpace(letter))
}
func (lex *testLexicon) NormalizeWord(word string) string {
	return strings.ToUpper(strings.TrimSpace(word))
}
func (lex *testLexicon) HasLetter(letter string) bool {
	return lex.letters[strings.ToUpper(letter)]
}
func (lex *testLexicon) ContainsWord(word string) bool {
	return lex.words[strings.ToUpper(word)]
}

func baseGame(rack []Tile) *Game {
	return &Game{
		SchemaVersion: SchemaVersion,
		ID:            "game-1",
		Config:        Config{Language: LanguageEnglish, Level: LevelA1, MaxWordLength: 7, PlayerCount: 2, BoardSize: BoardSize, BingoBonus: 35, Seed: 1},
		Status:        StatusPlaying,
		Players: []Player{
			{ID: "p1", Name: "One", Rack: rack},
			{ID: "p2", Name: "Two", Rack: []Tile{{ID: "ok", Letter: "O", Value: 1}}},
		},
		Current:  0,
		Revision: 1,
	}
}

func tiles(letters string) []Tile {
	result := make([]Tile, 0, len(letters))
	for index, letter := range letters {
		result = append(result, Tile{ID: TileID(strings.Repeat("x", index+1)), Letter: string(letter), Value: 1})
	}
	return result
}

func TestLayoutCounts(t *testing.T) {
	layout := WWFLayout()
	counts := map[Bonus]int{}
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			counts[layout[y][x]]++
		}
	}
	if counts[BonusTW] != 8 || counts[BonusDW] != 12 || counts[BonusTL] != 16 || counts[BonusDL] != 24 || counts[BonusStart] != 1 {
		t.Fatalf("unexpected bonus counts: %#v", counts)
	}
}

func TestFirstMoveValidation(t *testing.T) {
	rack := []Tile{{ID: "c", Letter: "C", Value: 3}, {ID: "a", Letter: "A", Value: 1}, {ID: "t", Letter: "T", Value: 1}}
	value := baseGame(rack)
	lex := newTestLexicon([]string{"CAT"})
	placements := []Placement{{TileID: "c", X: 6, Y: 7}, {TileID: "a", X: 7, Y: 7}, {TileID: "t", X: 8, Y: 7}}
	resolved, words, score, err := ValidatePlacement(value, lex, placements)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 3 || len(words) != 1 || words[0].Word != "CAT" || !words[0].Primary {
		t.Fatalf("unexpected words: %#v", words)
	}
	if score != 5 {
		t.Fatalf("score = %d, want 5", score)
	}
}

func TestFirstMoveMustCoverCenter(t *testing.T) {
	rack := []Tile{{ID: "c", Letter: "C", Value: 3}, {ID: "a", Letter: "A", Value: 1}, {ID: "t", Letter: "T", Value: 1}}
	value := baseGame(rack)
	lex := newTestLexicon([]string{"CAT"})
	placements := []Placement{{TileID: "c", X: 0, Y: 0}, {TileID: "a", X: 1, Y: 0}, {TileID: "t", X: 2, Y: 0}}
	_, _, _, err := ValidatePlacement(value, lex, placements)
	if err == nil {
		t.Fatal("expected center validation error")
	}
}

func TestPlacementGapRejected(t *testing.T) {
	rack := []Tile{{ID: "c", Letter: "C", Value: 3}, {ID: "a", Letter: "A", Value: 1}, {ID: "t", Letter: "T", Value: 1}}
	value := baseGame(rack)
	lex := newTestLexicon([]string{"CAT"})
	placements := []Placement{{TileID: "c", X: 5, Y: 7}, {TileID: "a", X: 7, Y: 7}, {TileID: "t", X: 8, Y: 7}}
	_, _, _, err := ValidatePlacement(value, lex, placements)
	if err == nil {
		t.Fatal("expected gap validation error")
	}
}

func TestApplyMoveAndReplay(t *testing.T) {
	now := time.Now()
	config := Config{Language: LanguageEnglish, Level: LevelA1, MaxWordLength: 5, PlayerCount: 2, BoardSize: BoardSize, BingoBonus: 25, Seed: 42}
	specs := []TileSpec{{Letter: "A", Value: 1, Count: 40}}
	value, err := NewGame("game-replay", config, []PlayerSeed{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}, specs, now)
	if err != nil {
		t.Fatal(err)
	}
	lex := newTestLexicon([]string{"AAAAA"})
	placements := make([]Placement, 0)
	for index, tile := range value.Players[0].Rack {
		placements = append(placements, Placement{TileID: tile.ID, X: 5 + index, Y: 7})
	}
	record, err := ApplyMove(value, lex, "p1", MoveRequest{MoveID: "m1", ExpectedRevision: value.Revision, Kind: MovePlace, Placements: placements}, now)
	if err != nil {
		t.Fatal(err)
	}
	if record.Score != 30 {
		t.Fatalf("score = %d, want 30", record.Score)
	}
	if value.Current != 1 {
		t.Fatalf("current = %d, want 1", value.Current)
	}
	if err := Replay(value, lex); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateMoveIDIsIdempotent(t *testing.T) {
	now := time.Now()
	config := Config{Language: LanguageEnglish, Level: LevelA1, MaxWordLength: 5, PlayerCount: 2, BoardSize: BoardSize, BingoBonus: 25, Seed: 42}
	specs := []TileSpec{{Letter: "A", Value: 1, Count: 40}}
	value, err := NewGame("game-idem", config, []PlayerSeed{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}, specs, now)
	if err != nil {
		t.Fatal(err)
	}
	lex := newTestLexicon([]string{"AAAAA"})
	placements := make([]Placement, 0)
	for index, tile := range value.Players[0].Rack {
		placements = append(placements, Placement{TileID: tile.ID, X: 5 + index, Y: 7})
	}
	request := MoveRequest{MoveID: "m1", ExpectedRevision: value.Revision, Kind: MovePlace, Placements: placements}
	if _, err := ApplyMove(value, lex, "p1", request, now); err != nil {
		t.Fatal(err)
	}
	revision := value.Revision
	if _, err := ApplyMove(value, lex, "p1", request, now); err != nil {
		t.Fatal(err)
	}
	if value.Revision != revision {
		t.Fatalf("revision changed on duplicate move: %d -> %d", revision, value.Revision)
	}
}

func TestScoreWordWithBonuses(t *testing.T) {
	layout := WWFLayout()
	board := map[Coord]PlacedTile{
		{X: 2, Y: 7}: {Tile: Tile{Letter: "C", Value: 3}, X: 2, Y: 7},
		{X: 3, Y: 7}: {Tile: Tile{Letter: "A", Value: 1}, X: 3, Y: 7},
		{X: 4, Y: 7}: {Tile: Tile{Letter: "T", Value: 1}, X: 4, Y: 7},
	}
	newAt := map[Coord]bool{{X: 2, Y: 7}: true, {X: 3, Y: 7}: true, {X: 4, Y: 7}: true}
	score, word := ScoreWord(layout, board, Coord{X: 2, Y: 7}, DirectionHorizontal, newAt)
	if word != "CAT" || score != 10 {
		t.Fatalf("score = %d for %q, want 10 for CAT", score, word)
	}
}

func TestCrossWordValidation(t *testing.T) {
	value := baseGame([]Tile{{ID: "s", Letter: "S", Value: 1}})
	value.Board = []PlacedTile{
		{Tile: Tile{ID: "c", Letter: "C", Value: 3}, X: 6, Y: 7},
		{Tile: Tile{ID: "a", Letter: "A", Value: 1}, X: 7, Y: 7},
		{Tile: Tile{ID: "t", Letter: "T", Value: 1}, X: 8, Y: 7},
	}
	value.Current = 1
	value.Players[1].Rack = []Tile{{ID: "s", Letter: "S", Value: 1}}
	lex := newTestLexicon([]string{"CAT", "CATS"})
	_, words, score, err := ValidatePlacement(value, lex, []Placement{{TileID: "s", X: 9, Y: 7}})
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 1 || words[0].Word != "CATS" || score != 6 {
		t.Fatalf("unexpected words %#v score %d", words, score)
	}
}

func TestFinishWithAward(t *testing.T) {
	value := baseGame(nil)
	value.Players[0].Score = 5
	value.Players[0].Rack = nil
	value.Players[1].Score = 10
	value.Players[1].Rack = []Tile{{Letter: "B", Value: 3}, {Letter: "C", Value: 4}}
	finishWithAward(value, 0)
	if value.Status != StatusFinished {
		t.Fatalf("status = %s", value.Status)
	}
	if value.Players[0].Score != 12 || value.Players[1].Score != 3 {
		t.Fatalf("scores = %d, %d; want 12, 3", value.Players[0].Score, value.Players[1].Score)
	}
	if value.Winner == nil || *value.Winner != "p1" {
		t.Fatalf("winner = %#v", value.Winner)
	}
}
