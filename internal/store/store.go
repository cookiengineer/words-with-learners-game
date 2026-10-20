package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cookiengineer/words-with-learners-app/internal/game"
)

type Profile struct {
	SchemaVersion int           `json:"schemaVersion"`
	ID            game.PlayerID `json:"id"`
	Username      string        `json:"username"`
	DisplayName   string        `json:"displayName"`
	PasswordHash  []byte        `json:"passwordHash"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

type Store struct {
	root      string
	profiles  string
	games     string
	mutex     sync.RWMutex
	byID      map[game.PlayerID]*Profile
	byName    map[string]*Profile
	playerIDs []game.PlayerID
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("data directory is required")
	}
	store := &Store{
		root:      root,
		profiles:  filepath.Join(root, "profiles"),
		games:     filepath.Join(root, "games"),
		byID:      make(map[game.PlayerID]*Profile),
		byName:    make(map[string]*Profile),
		playerIDs: make([]game.PlayerID, 0),
	}
	if err := os.MkdirAll(store.profiles, 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(store.games, 0o700); err != nil {
		return nil, err
	}
	if err := store.loadProfiles(); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *Store) Root() string {
	return store.root
}

func (store *Store) loadProfiles() error {
	entries, err := os.ReadDir(store.profiles)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(store.profiles, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var profile Profile
		if err := json.Unmarshal(raw, &profile); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if profile.ID == "" || profile.Username == "" {
			return fmt.Errorf("%s: profile is missing an id or username", path)
		}
		store.byID[profile.ID] = &profile
		store.byName[strings.ToLower(profile.Username)] = &profile
		store.playerIDs = append(store.playerIDs, profile.ID)
	}
	sort.Slice(store.playerIDs, func(i, j int) bool {
		return strings.ToLower(store.byID[store.playerIDs[i]].DisplayName) < strings.ToLower(store.byID[store.playerIDs[j]].DisplayName)
	})
	return nil
}

func NewID() (string, error) {
	var buffer [16]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer[:]), nil
}

func (store *Store) CreateProfile(username string, displayName string, passwordHash []byte, now time.Time) (*Profile, error) {
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = username
	}
	key := strings.ToLower(username)
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if _, exists := store.byName[key]; exists {
		return nil, ErrUsernameTaken
	}
	id, err := NewID()
	if err != nil {
		return nil, err
	}
	profile := &Profile{
		SchemaVersion: 1,
		ID:            game.PlayerID(id),
		Username:      username,
		DisplayName:   displayName,
		PasswordHash:  append([]byte(nil), passwordHash...),
		CreatedAt:     now.UTC(),
		UpdatedAt:     now.UTC(),
	}
	if err := store.saveProfile(profile); err != nil {
		return nil, err
	}
	store.byID[profile.ID] = profile
	store.byName[key] = profile
	store.playerIDs = append(store.playerIDs, profile.ID)
	sort.Slice(store.playerIDs, func(i, j int) bool {
		return strings.ToLower(store.byID[store.playerIDs[i]].DisplayName) < strings.ToLower(store.byID[store.playerIDs[j]].DisplayName)
	})
	copyProfile := *profile
	return &copyProfile, nil
}

func (store *Store) saveProfile(profile *Profile) error {
	raw, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(store.profiles, string(profile.ID)+".json")
	return writeAtomic(path, append(raw, '\n'))
}

func (store *Store) Profile(id game.PlayerID) (*Profile, bool) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	profile, ok := store.byID[id]
	if !ok {
		return nil, false
	}
	copyProfile := *profile
	copyProfile.PasswordHash = append([]byte(nil), profile.PasswordHash...)
	return &copyProfile, true
}

func (store *Store) ProfileByUsername(username string) (*Profile, bool) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	profile, ok := store.byName[strings.ToLower(strings.TrimSpace(username))]
	if !ok {
		return nil, false
	}
	copyProfile := *profile
	copyProfile.PasswordHash = append([]byte(nil), profile.PasswordHash...)
	return &copyProfile, true
}

func (store *Store) SearchProfiles(query string, offset int, limit int, exclude game.PlayerID) ([]Profile, int) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	query = strings.ToLower(strings.TrimSpace(query))
	matches := make([]Profile, 0)
	for _, id := range store.playerIDs {
		if id == exclude {
			continue
		}
		profile := store.byID[id]
		if profile == nil {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(profile.DisplayName + " " + profile.Username)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		copyProfile := *profile
		copyProfile.PasswordHash = nil
		matches = append(matches, copyProfile)
	}
	total := len(matches)
	if offset >= total {
		return []Profile{}, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matches[offset:end], total
}

func (store *Store) ProfileCount() int {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	return len(store.playerIDs)
}

func (store *Store) LoadGames() ([]*game.Game, error) {
	entries, err := os.ReadDir(store.games)
	if err != nil {
		return nil, err
	}
	games := make([]*game.Game, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(store.games, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var loaded game.Game
		if err := json.Unmarshal(raw, &loaded); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if loaded.ID == "" {
			return nil, fmt.Errorf("%s: game is missing an id", path)
		}
		games = append(games, &loaded)
	}
	sort.Slice(games, func(i, j int) bool {
		return games[i].UpdatedAt.After(games[j].UpdatedAt)
	})
	return games, nil
}

func (store *Store) LoadGame(id game.GameID) (*game.Game, error) {
	path := filepath.Join(store.games, string(id)+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var loaded game.Game
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return nil, err
	}
	return &loaded, nil
}

func (store *Store) SaveGame(value *game.Game) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(store.games, string(value.ID)+".json")
	return writeAtomic(path, append(raw, '\n'))
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

var ErrUsernameTaken = errors.New("username is already taken")
