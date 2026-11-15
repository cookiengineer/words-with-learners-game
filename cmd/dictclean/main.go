package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cookiengineer/words-with-learners-app/internal/dictionary"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/wiktionary"
)

type document struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Language      game.Language          `json:"language"`
	Level         game.Level             `json:"level,omitempty"`
	Kind          string                 `json:"kind,omitempty"`
	Words         []dictionary.WordEntry `json:"words"`
}

func main() {
	var dictionaries string
	var dryRun bool
	var verbose bool
	flag.StringVar(&dictionaries, "dictionaries", "dictionaries", "dictionary root directory")
	flag.BoolVar(&dryRun, "dry-run", false, "report changes without writing files")
	flag.BoolVar(&verbose, "verbose", false, "log every changed hint")
	flag.Parse()

	paths, err := filepath.Glob(filepath.Join(dictionaries, "*", "*.json"))
	if err != nil {
		log.Fatal(err)
	}
	sort.Strings(paths)

	totalHints, changedHints, manualHints, changedFiles := 0, 0, 0, 0
	for _, path := range paths {
		if filepath.Base(path) == "alphabet.json" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("read %s: %v", path, err)
		}
		var doc document
		if err := json.Unmarshal(raw, &doc); err != nil {
			log.Fatalf("parse %s: %v", path, err)
		}

		fileChanges := 0
		for index := range doc.Words {
			hint := strings.TrimSpace(doc.Words[index].Hint)
			if hint == "" {
				continue
			}
			totalHints++
			cleaned := wiktionary.NormalizeHint(hint)
			if cleaned == "" {
				manualHints++
				log.Printf("needs manual hint: %s %s %q", path, doc.Words[index].Word, hint)
				continue
			}
			if cleaned == hint {
				continue
			}
			if verbose {
				log.Printf("change %s %s\n  old: %q\n  new: %q", path, doc.Words[index].Word, hint, cleaned)
			}
			doc.Words[index].Hint = cleaned
			fileChanges++
		}

		if fileChanges == 0 {
			continue
		}
		changedHints += fileChanges
		changedFiles++
		if dryRun {
			log.Printf("would change %d hints in %s", fileChanges, path)
			continue
		}
		payload, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			log.Fatalf("marshal %s: %v", path, err)
		}
		if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
			log.Fatalf("write %s: %v", path, err)
		}
		log.Printf("changed %d hints in %s", fileChanges, path)
	}

	prefix := ""
	if dryRun {
		prefix = "would change "
	} else {
		prefix = "changed "
	}
	fmt.Printf("%s%d of %d hints in %d files; %d hints need manual completion\n", prefix, changedHints, totalHints, changedFiles, manualHints)
	if dryRun {
		fmt.Println("dry run: no files were written")
	}
}
