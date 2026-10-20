package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cookiengineer/words-with-learners-app/internal/game"
)

func TestReadEntriesNewWordListFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "words.csv")
	content := "\ufeffrank,word,cefr\n1,ich,A0\n#,Lingvoberry comment,\n2,gehen,A1\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := readEntries(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Word != "ich" || entries[1].Word != "gehen" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestReadEntriesLegacyWordHintFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "words.csv")
	content := "word,hint\nhuis,house\ntuin,garden\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := readEntries(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Hint != "house" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestParseWordListName(t *testing.T) {
	lang, level, ok := parseWordListName("lingvoberry-ru-b1-words.csv")
	if !ok || lang != game.LanguageRussian || level != game.LevelB1 {
		t.Fatalf("unexpected result: %s %s %v", lang, level, ok)
	}
}

func TestValidateWord(t *testing.T) {
	letters := map[string]bool{"A": true, "B": true}
	if reason := validateWord("AB", letters); reason != "" {
		t.Fatalf("unexpected reason: %s", reason)
	}
	if reason := validateWord("A", letters); reason != "too_short" {
		t.Fatalf("unexpected reason: %s", reason)
	}
	if reason := validateWord("ABC", letters); reason != "unknown_letter:C" {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestRunDirectoryMergesA0IntoA1(t *testing.T) {
	root := t.TempDir()
	dictionaries := filepath.Join(root, "dictionaries")
	if err := os.MkdirAll(filepath.Join(dictionaries, "de"), 0o755); err != nil {
		t.Fatal(err)
	}
	alphabet := `{"schemaVersion":1,"language":"de","letters":[{"letter":"A","value":1,"count":2},{"letter":"B","value":1,"count":2},{"letter":"C","value":1,"count":2}],"blankCount":0}`
	if err := os.WriteFile(filepath.Join(dictionaries, "de", "alphabet.json"), []byte(alphabet), 0o644); err != nil {
		t.Fatal(err)
	}
	words := filepath.Join(root, "words")
	if err := os.MkdirAll(words, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(words, "lingvoberry-de-a0-words.csv"), []byte("rank,word,cefr\n1,AB,A0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(words, "lingvoberry-de-a1-words.csv"), []byte("rank,word,cefr\n401,BC,A1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, err := runDirectory(options{
		input:        words,
		dictionaries: dictionaries,
		mergeA0:      true,
		workers:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %#v", issues)
	}
	raw, err := os.ReadFile(filepath.Join(dictionaries, "de", "a1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"word": "AB"`) || !strings.Contains(string(raw), `"word": "BC"`) {
		t.Fatalf("A0 word was not merged into A1: %s", raw)
	}
	if _, err := os.Stat(filepath.Join(dictionaries, "de", "a0.json")); !os.IsNotExist(err) {
		t.Fatal("A0 output should not exist")
	}
}
