package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cookiengineer/words-with-learners-app/internal/dictionary"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
)

func main() {
	var (
		input    = flag.String("input", "", "CSV file with word,hint columns or JSON with a words array")
		language = flag.String("language", "", "language code: nl, en, de, es, fr")
		level    = flag.String("level", "", "CEFR level: A1, A2, B1, B2, C1, C2")
		dictDir  = flag.String("dictionaries", "dictionaries", "dictionary root directory")
		output   = flag.String("output", "", "output file (default: <dictionaries>/<language>/<level>.json)")
		merge    = flag.Bool("merge", true, "merge with an existing level file")
	)
	flag.Parse()
	if *input == "" || *language == "" || *level == "" {
		flag.Usage()
		os.Exit(2)
	}
	lang := game.Language(strings.ToLower(*language))
	lvl := game.Level(strings.ToUpper(*level))
	if !lang.Valid() || !lvl.Valid() {
		log.Fatalf("unsupported language or level: %s %s", lang, lvl)
	}
	alphabetPath := filepath.Join(*dictDir, string(lang), "alphabet.json")
	alphabetRaw, err := os.ReadFile(alphabetPath)
	if err != nil {
		log.Fatalf("read alphabet: %v", err)
	}
	var alphabet dictionary.AlphabetFile
	if err := json.Unmarshal(alphabetRaw, &alphabet); err != nil {
		log.Fatalf("parse alphabet: %v", err)
	}
	letters := make(map[string]bool)
	for _, spec := range alphabet.Letters {
		letters[strings.ToUpper(spec.Letter)] = true
	}
	entries, err := readEntries(*input)
	if err != nil {
		log.Fatal(err)
	}
	merged := make(map[string]string)
	if *merge {
		if *output == "" {
			*output = filepath.Join(*dictDir, string(lang), strings.ToLower(string(lvl))+".json")
		}
		if raw, readErr := os.ReadFile(*output); readErr == nil {
			var existing dictionary.WordFile
			if jsonErr := json.Unmarshal(raw, &existing); jsonErr == nil {
				for _, entry := range existing.Words {
					merged[strings.ToUpper(entry.Word)] = entry.Hint
				}
			}
		}
	}
	for _, entry := range entries {
		word := strings.ToUpper(strings.TrimSpace(entry.Word))
		if utf8.RuneCountInString(word) < 2 || utf8.RuneCountInString(word) > game.MaxRackSize {
			log.Printf("skip %q: invalid length", entry.Word)
			continue
		}
		valid := true
		for _, letter := range word {
			if !letters[string(letter)] {
				log.Printf("skip %q: letter %q is not in the alphabet", word, string(letter))
				valid = false
				break
			}
		}
		if valid {
			merged[word] = strings.TrimSpace(entry.Hint)
		}
	}
	if len(merged) == 0 {
		log.Fatal("no usable words")
	}
	words := make([]dictionary.WordEntry, 0, len(merged))
	for word, hint := range merged {
		words = append(words, dictionary.WordEntry{Word: word, Hint: hint})
	}
	sort.Slice(words, func(i, j int) bool { return words[i].Word < words[j].Word })
	if *output == "" {
		*output = filepath.Join(*dictDir, string(lang), strings.ToLower(string(lvl))+".json")
	}
	payload := dictionary.WordFile{
		SchemaVersion: 1,
		Language:      lang,
		Level:         lvl,
		Words:         words,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*output, append(raw, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %d words to %s\n", len(words), *output)
}

func readEntries(path string) ([]dictionary.WordEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "{") {
		var payload dictionary.WordFile
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		return payload.Words, nil
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	entries := make([]dictionary.WordEntry, 0, len(rows))
	for index, row := range rows {
		if len(row) == 0 {
			continue
		}
		if index == 0 && strings.EqualFold(strings.TrimSpace(row[0]), "word") {
			continue
		}
		hint := ""
		if len(row) > 1 {
			hint = row[1]
		}
		entries = append(entries, dictionary.WordEntry{Word: row[0], Hint: hint})
	}
	return entries, nil
}
