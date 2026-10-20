package auth

import (
	"testing"
	"time"

	"github.com/cookiengineer/words-with-learners-app/internal/store"
)

func TestSessionsPersistAcrossRestarts(t *testing.T) {
	profiles, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	manager, err := NewManager(profiles)
	if err != nil {
		t.Fatal(err)
	}
	profile, token, err := manager.Register("alice", "Alice", "password123", now)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewManager(profiles)
	if err != nil {
		t.Fatal(err)
	}
	playerID, ok := reopened.Authenticate(token, now)
	if !ok || playerID != profile.ID {
		t.Fatal("session did not survive restart")
	}
	reopened.Logout(token)
	third, err := NewManager(profiles)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := third.Authenticate(token, now); ok {
		t.Fatal("logged out session survived restart")
	}
}
