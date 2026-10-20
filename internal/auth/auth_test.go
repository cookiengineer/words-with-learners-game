package auth

import (
	"errors"
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
	profile, token, err := manager.LoginOrRegister("alice", "password123", now)
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

func TestLoginOrRegister(t *testing.T) {
	profiles, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	manager, err := NewManager(profiles)
	if err != nil {
		t.Fatal(err)
	}

	created, _, err := manager.LoginOrRegister("alice", "password123", now)
	if err != nil {
		t.Fatal(err)
	}
	if created.Username != "alice" {
		t.Fatalf("created username = %q", created.Username)
	}

	loggedIn, _, err := manager.LoginOrRegister("ALICE", "password123", now)
	if err != nil {
		t.Fatal(err)
	}
	if loggedIn.ID != created.ID {
		t.Fatalf("logged in as %q, want %q", loggedIn.ID, created.ID)
	}

	if _, _, err := manager.LoginOrRegister("alice", "wrong-password", now); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
	if _, _, err := manager.LoginOrRegister("x", "password123", now); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("invalid username error = %v", err)
	}
	if _, _, err := manager.LoginOrRegister("bob", "short", now); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("invalid password error = %v", err)
	}
}
