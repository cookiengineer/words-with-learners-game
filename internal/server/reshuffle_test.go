package server

import (
	"encoding/json"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	app "github.com/cookiengineer/words-with-learners-app"
	appconfig "github.com/cookiengineer/words-with-learners-app/internal/config"
	"github.com/cookiengineer/words-with-learners-app/internal/dictionary"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/store"
)

type overlayFS struct {
	base    fs.FS
	overlay fs.FS
}

func (value overlayFS) Open(name string) (fs.File, error) {
	if _, err := fs.Stat(value.overlay, name); err == nil {
		return value.overlay.Open(name)
	}
	return value.base.Open(name)
}

func loadReplayEnglish(t *testing.T) *dictionary.Dictionary {
	t.Helper()
	root, err := fs.Sub(app.Files, "dictionaries")
	if err != nil {
		t.Fatal(err)
	}
	payload := dictionary.WordFile{
		SchemaVersion: 1,
		Language:      game.LanguageEnglish,
		Level:         game.LevelA1,
		Words: []dictionary.WordEntry{
			{Word: "QI", Hint: "life force"},
			{Word: "QAT", Hint: "a plant"},
			{Word: "QUIT", Hint: "to leave"},
			{Word: "QUIZ", Hint: "a test"},
			{Word: "QUAD", Hint: "a courtyard"},
			{Word: "QUEST", Hint: "a search"},
			{Word: "QUILT", Hint: "a blanket"},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	overlay := fstest.MapFS{
		"en/a1.json": &fstest.MapFile{Data: raw},
	}
	library, err := dictionary.LoadLibrary(overlayFS{base: root, overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	dict, err := library.Get(game.LanguageEnglish, game.LevelA1)
	if err != nil {
		t.Fatal(err)
	}
	return dict
}

func loadEnglish(t *testing.T) *dictionary.Dictionary {
	t.Helper()
	root, err := fs.Sub(app.Files, "dictionaries")
	if err != nil {
		t.Fatal(err)
	}
	library, err := dictionary.LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	dict, err := library.Get(game.LanguageEnglish, game.LevelA1)
	if err != nil {
		t.Fatal(err)
	}
	return dict
}

func testGame(t *testing.T, dict *dictionary.Dictionary) *game.Game {
	t.Helper()
	config := game.Config{
		Language:      game.LanguageEnglish,
		Level:         game.LevelA1,
		MaxWordLength: 7,
		PlayerCount:   2,
		BoardSize:     game.BoardSize,
		BingoBonus:    35,
		Seed:          123,
	}
	value, err := game.NewGame("test-game", config, []game.PlayerSeed{
		{ID: "p1", Name: "One"},
		{ID: "p2", Name: "Two"},
	}, dict.Specs(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func stuckRack() []game.Tile {
	return []game.Tile{
		{ID: "x1", Letter: "X", Value: 8},
		{ID: "x2", Letter: "X", Value: 8},
		{ID: "x3", Letter: "X", Value: 8},
		{ID: "x4", Letter: "X", Value: 8},
		{ID: "x5", Letter: "X", Value: 8},
		{ID: "x6", Letter: "X", Value: 8},
		{ID: "x7", Letter: "X", Value: 8},
	}
}

func TestEnsurePlayableReshufflesStuckRack(t *testing.T) {
	dict := loadEnglish(t)
	value := testGame(t, dict)
	value.Players[value.Current].Rack = stuckRack()
	if findHintFor(value, dict, value.Current, true).Found {
		t.Fatal("test rack unexpectedly has a playable word")
	}
	reshuffle := ensurePlayable(value, dict)
	if reshuffle == nil {
		t.Fatal("expected an automatic reshuffle")
	}
	if reshuffle.PlayerID != value.Players[value.Current].ID || reshuffle.Attempts == 0 {
		t.Fatalf("unexpected reshuffle: %#v", reshuffle)
	}
	if !findHintFor(value, dict, value.Current, true).Found {
		t.Fatal("reshuffled rack still has no playable word")
	}
}

func TestEnsurePlayableWithoutBagLeavesRackAlone(t *testing.T) {
	dict := loadEnglish(t)
	value := testGame(t, dict)
	value.Players[value.Current].Rack = stuckRack()
	value.Bag = nil
	reshuffle := ensurePlayable(value, dict)
	if reshuffle != nil {
		t.Fatal("did not expect a reshuffle without bag tiles")
	}
	if len(value.Players[value.Current].Rack) != 7 {
		t.Fatal("rack changed unexpectedly")
	}
}

func TestEnsureRackContainsMakesWordPlayable(t *testing.T) {
	dict := loadEnglish(t)
	value := testGame(t, dict)
	value.Players[value.Current].Rack = stuckRack()
	wanted := make([]game.Tile, 0, 5)
	needed := map[string]int{"H": 1, "A": 1, "P": 2, "Y": 1}
	for index := range value.Bag {
		tile := value.Bag[index]
		if needed[tile.Letter] > 0 {
			needed[tile.Letter]--
			wanted = append(wanted, tile)
		}
	}
	if len(wanted) != 5 {
		t.Fatalf("could not find HAPPY tiles in bag: %#v", wanted)
	}
	_, _, ok := game.EnsureRackContains(value, value.Current, wanted)
	if !ok {
		t.Fatal("EnsureRackContains failed")
	}
	rackIDs := make(map[game.TileID]bool)
	for _, tile := range value.Players[value.Current].Rack {
		rackIDs[tile.ID] = true
	}
	for _, tile := range wanted {
		if !rackIDs[tile.ID] {
			t.Fatalf("wanted tile %s is not in the rack", tile.ID)
		}
	}
	if !findHintFor(value, dict, value.Current, true).Found {
		t.Fatal("rack should now have a playable word")
	}
}

func findTestMove(value *game.Game, dict *dictionary.Dictionary) []game.Placement {
	player := value.CurrentPlayer()
	if player == nil {
		return nil
	}
	board := game.BoardMap(value.Board)
	for _, entry := range dict.Entries() {
		length := len([]rune(entry.Word))
		if length < 2 || length > value.Config.MaxWordLength {
			continue
		}
		for _, direction := range []game.Direction{game.DirectionHorizontal, game.DirectionVertical} {
			for startY := 0; startY < game.BoardSize; startY++ {
				for startX := 0; startX < game.BoardSize; startX++ {
					if direction == game.DirectionHorizontal && startX+length > game.BoardSize {
						continue
					}
					if direction == game.DirectionVertical && startY+length > game.BoardSize {
						continue
					}
					placements, ok := matchWord(player.Rack, board, entry.Word, game.Coord{X: startX, Y: startY}, direction)
					if !ok || len(placements) == 0 {
						continue
					}
					if _, _, _, err := game.ValidatePlacement(value, dict, placements); err == nil {
						return placements
					}
				}
			}
		}
	}
	return nil
}

func TestReplayWithAutoReshuffle(t *testing.T) {
	dict := loadReplayEnglish(t)
	var value *game.Game
	for seed := int64(1); seed < 400; seed++ {
		config := game.Config{
			Language:      game.LanguageEnglish,
			Level:         game.LevelA1,
			MaxWordLength: 7,
			PlayerCount:   2,
			BoardSize:     game.BoardSize,
			BingoBonus:    35,
			Seed:          seed,
		}
		candidate, err := game.NewGame("replay-reshuffle", config, []game.PlayerSeed{
			{ID: "p1", Name: "One"},
			{ID: "p2", Name: "Two"},
		}, dict.Specs(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		move := findTestMove(candidate, dict)
		if move == nil {
			continue
		}
		if _, err := game.ApplyMove(candidate, dict, "p1", game.MoveRequest{
			MoveID:           "move-1",
			ExpectedRevision: candidate.Revision,
			Kind:             game.MovePlace,
			Placements:       move,
		}, time.Now()); err != nil {
			continue
		}
		if findHintFor(candidate, dict, candidate.Current, true).Found {
			continue
		}
		value = candidate
		break
	}
	if value == nil {
		t.Fatal("could not find a seed with a valid first move and a stuck next rack")
	}
	reshuffle := ensurePlayable(value, dict)
	if reshuffle == nil {
		t.Fatal("expected the stuck rack to be reshuffled")
	}
	reshuffle.Revision = value.Revision
	value.LastReshuffle = reshuffle
	value.Reshuffles = append(value.Reshuffles, *reshuffle)
	if !findHintFor(value, dict, value.Current, true).Found {
		t.Fatal("reshuffle did not create a playable rack")
	}
	secondMove := findTestMove(value, dict)
	if secondMove == nil {
		t.Fatal("could not find the second move")
	}
	if _, err := game.ApplyMove(value, dict, value.CurrentPlayer().ID, game.MoveRequest{
		MoveID:           "move-2",
		ExpectedRevision: value.Revision,
		Kind:             game.MovePlace,
		Placements:       secondMove,
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := game.Replay(value, dict); err != nil {
		t.Fatal(err)
	}
}

func TestServerLoadReshufflesStuckGame(t *testing.T) {
	dir := t.TempDir()
	dict := loadEnglish(t)
	profiles, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	config := game.Config{
		Language:      game.LanguageEnglish,
		Level:         game.LevelA1,
		MaxWordLength: 7,
		PlayerCount:   2,
		BoardSize:     game.BoardSize,
		BingoBonus:    35,
		Seed:          99,
	}
	value, err := game.NewGame("stuck-game", config, []game.PlayerSeed{
		{ID: "p1", Name: "One"},
		{ID: "p2", Name: "Two"},
	}, dict.Specs(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	value.Players[value.Current].Rack = stuckRack()
	if err := profiles.SaveGame(value); err != nil {
		t.Fatal(err)
	}
	loaded, err := New(appconfig.Config{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	session, ok := loaded.session(value.ID)
	if !ok {
		t.Fatal("game was not loaded")
	}
	session.mutex.Lock()
	defer session.mutex.Unlock()
	if len(session.value.Reshuffles) == 0 {
		t.Fatal("loaded stuck game was not reshuffled")
	}
	if !findHintFor(session.value, dict, session.value.Current, true).Found {
		t.Fatal("loaded game still has no playable word")
	}
}
