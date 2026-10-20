package store

import (
	"testing"
	"time"

	"github.com/cookiengineer/words-with-learners-app/internal/game"
)

func TestProfilesPersistAndSearch(t *testing.T) {
	dir := t.TempDir()
	profiles, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := profiles.CreateProfile("alice", []byte("hash"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profiles.CreateProfile("ALICE", []byte("hash"), time.Now()); err != ErrUsernameTaken {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := profiles.CreateProfile("bob", []byte("hash"), time.Now()); err != nil {
		t.Fatal(err)
	}
	results, total := profiles.SearchProfiles("bo", 0, 10, "")
	if total != 1 || len(results) != 1 || results[0].Username != "bob" {
		t.Fatalf("search = %#v total %d", results, total)
	}
	if len(results[0].PasswordHash) != 0 {
		t.Fatal("search leaked a password hash")
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, ok := reopened.Profile(profile.ID)
	if !ok || loaded.Username != "alice" || string(loaded.PasswordHash) != "hash" {
		t.Fatalf("loaded profile = %#v", loaded)
	}
}

func TestGamesPersist(t *testing.T) {
	dir := t.TempDir()
	profiles, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	value := &game.Game{
		SchemaVersion: game.SchemaVersion,
		ID:            "game-1",
		Config:        game.Config{Language: game.LanguageEnglish, Level: game.LevelA1, MaxWordLength: 7, PlayerCount: 2, BoardSize: game.BoardSize, BingoBonus: 35},
		Status:        game.StatusPlaying,
		Players:       []game.Player{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}},
		Board:         []game.PlacedTile{},
		Bag:           []game.Tile{},
		Moves:         []game.MoveRecord{},
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := profiles.SaveGame(value); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	games, err := reopened.LoadGames()
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].ID != value.ID {
		t.Fatalf("games = %#v", games)
	}
}
