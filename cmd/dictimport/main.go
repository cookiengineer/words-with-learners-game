package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/cookiengineer/words-with-learners-app/internal/dictionary"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/wiktionary"
)

type options struct {
	input         string
	language      string
	level         string
	dictionaries  string
	output        string
	merge         bool
	languages     string
	levels        string
	zim           string
	zimTool       string
	zimSearchTool string
	workers       int
	report        string
	mergeA0       bool
}

type issue struct {
	Language string
	Level    string
	Word     string
	Reason   string
}

func main() {
	var opts options
	flag.StringVar(&opts.input, "input", "", "CSV/JSON file, or a directory containing lingvoberry-<language>-<level>-words.csv")
	flag.StringVar(&opts.language, "language", "", "language code: nl, en, de, es, fr, it, ru")
	flag.StringVar(&opts.level, "level", "", "CEFR level: A1, A2, B1, B2, C1, C2")
	flag.StringVar(&opts.dictionaries, "dictionaries", "dictionaries", "dictionary root directory")
	flag.StringVar(&opts.output, "output", "", "output file (default: <dictionaries>/<language>/<level>.json)")
	flag.BoolVar(&opts.merge, "merge", true, "merge with an existing level file")
	flag.StringVar(&opts.languages, "languages", "", "comma-separated languages for directory input")
	flag.StringVar(&opts.levels, "levels", "", "comma-separated levels for directory input")
	flag.StringVar(&opts.zim, "zim", "", "ZIM file used to look up Wiktionary hints")
	flag.StringVar(&opts.zimTool, "zim-tool", "zimdump", "command used to read the ZIM file")
	flag.StringVar(&opts.zimSearchTool, "zim-search-tool", "zimsearch", "command used to search the ZIM file")
	flag.IntVar(&opts.workers, "workers", 8, "parallel Wiktionary lookups")
	flag.StringVar(&opts.report, "report", "", "write skipped or hint-less words to a CSV report")
	flag.BoolVar(&opts.mergeA0, "merge-a0", true, "merge A0 word lists into A1")
	flag.Parse()

	if opts.input == "" {
		flag.Usage()
		os.Exit(2)
	}
	if opts.workers < 1 {
		opts.workers = 1
	}
	if opts.workers > 64 {
		opts.workers = 64
	}
	if opts.zim != "" {
		if _, err := os.Stat(opts.zim); err != nil {
			log.Fatalf("read zim: %v", err)
		}
		if _, err := exec.LookPath(opts.zimTool); err != nil {
			log.Fatalf("find zim tool: %v", err)
		}
		if _, err := exec.LookPath(opts.zimSearchTool); err != nil {
			log.Fatalf("find zim search tool: %v", err)
		}
	}

	info, err := os.Stat(opts.input)
	if err != nil {
		log.Fatal(err)
	}
	var issues []issue
	if info.IsDir() {
		issues, err = runDirectory(opts)
	} else {
		issues, err = runSingle(opts)
	}
	if err != nil {
		log.Fatal(err)
	}
	if opts.report != "" {
		if err := writeReport(opts.report, issues); err != nil {
			log.Fatal(err)
		}
	}
	if len(issues) > 0 {
		log.Printf("%d words were skipped or have no hint", len(issues))
	}
}

func runSingle(opts options) ([]issue, error) {
	lang := game.Language(strings.ToLower(opts.language))
	lvl := game.Level(strings.ToUpper(opts.level))
	if !lang.Valid() || !lvl.Valid() {
		return nil, fmt.Errorf("unsupported language or level: %s %s", lang, lvl)
	}
	alphabet, err := readAlphabet(opts.dictionaries, lang)
	if err != nil {
		return nil, err
	}
	output := opts.output
	if output == "" {
		output = outputPath(opts.dictionaries, lang, lvl)
	}
	files := []string{opts.input}
	issues, err := buildLevel(opts, lang, lvl, files, output, alphabet, nil)
	return issues, err
}

func runDirectory(opts options) ([]issue, error) {
	paths, err := filepath.Glob(filepath.Join(opts.input, "lingvoberry-*-*-words.csv"))
	if err != nil {
		return nil, err
	}
	grouped := make(map[game.Language]map[game.Level][]string)
	for _, path := range paths {
		lang, level, ok := parseWordListName(filepath.Base(path))
		if !ok {
			continue
		}
		if !languageAllowed(lang, opts.languages) || !levelAllowed(level, opts.levels) {
			continue
		}
		if grouped[lang] == nil {
			grouped[lang] = make(map[game.Level][]string)
		}
		grouped[lang][level] = append(grouped[lang][level], path)
	}
	if len(grouped) == 0 {
		return nil, fmt.Errorf("no lingvoberry word lists found in %s", opts.input)
	}

	cache := make(map[string]string)
	var issues []issue
	for _, lang := range game.Languages {
		levels := grouped[lang]
		if len(levels) == 0 {
			continue
		}
		alphabet, err := readAlphabet(opts.dictionaries, lang)
		if err != nil {
			return issues, err
		}
		for _, lvl := range game.Levels {
			if !levelAllowed(lvl, opts.levels) {
				continue
			}
			files := make([]string, 0, 2)
			if lvl == game.LevelA1 && opts.mergeA0 && len(levels[game.Level("A0")]) > 0 {
				files = append(files, levels[game.Level("A0")]...)
			}
			files = append(files, levels[lvl]...)
			if len(files) == 0 {
				continue
			}
			output := outputPath(opts.dictionaries, lang, lvl)
			levelIssues, err := buildLevel(opts, lang, lvl, files, output, alphabet, cache)
			if err != nil {
				return issues, err
			}
			issues = append(issues, levelIssues...)
		}
	}
	return issues, nil
}

func buildLevel(opts options, lang game.Language, lvl game.Level, files []string, output string, alphabet map[string]bool, cache map[string]string) ([]issue, error) {
	if cache == nil {
		cache = make(map[string]string)
	}
	merged := make(map[string]string)
	if opts.merge {
		if err := loadExisting(output, merged); err != nil {
			return nil, err
		}
	}
	var issues []issue
	for _, file := range files {
		entries, err := readEntries(file)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			word := normalizeWord(entry.Word)
			if reason := validateWord(word, alphabet); reason != "" {
				issues = append(issues, issue{Language: string(lang), Level: string(lvl), Word: entry.Word, Reason: reason})
				log.Printf("skip %s %s %q: %s", lang, lvl, entry.Word, reason)
				continue
			}
			hint := strings.TrimSpace(entry.Hint)
			if _, exists := merged[word]; !exists || hint != "" {
				merged[word] = hint
			}
		}
	}
	if len(merged) == 0 {
		return issues, fmt.Errorf("%s %s: no usable words", lang, lvl)
	}

	words := make([]string, 0, len(merged))
	for word := range merged {
		words = append(words, word)
	}
	sort.Strings(words)
	if opts.zim != "" {
		log.Printf("%s %s: looking up %d Wiktionary hints with %d workers", lang, lvl, len(words), opts.workers)
		missing, lookupIssues := enrichHints(opts, words, merged, lang, lvl, cache)
		for _, word := range missing {
			issues = append(issues, issue{Language: string(lang), Level: string(lvl), Word: word, Reason: "no_hint"})
		}
		issues = append(issues, lookupIssues...)
	}

	entries := make([]dictionary.WordEntry, 0, len(words))
	for _, word := range words {
		entries = append(entries, dictionary.WordEntry{Word: word, Hint: strings.TrimSpace(merged[word])})
	}
	if err := writeWordFile(output, lang, lvl, entries); err != nil {
		return issues, err
	}
	log.Printf("%s %s: wrote %d words to %s", lang, lvl, len(entries), output)
	return issues, nil
}

func enrichHints(opts options, words []string, merged map[string]string, lang game.Language, lvl game.Level, cache map[string]string) ([]string, []issue) {
	lookup := wiktionary.Lookup{ZimPath: opts.zim, Tool: opts.zimTool, SearchTool: opts.zimSearchTool}
	jobs := make(chan string)
	var mutex sync.Mutex
	var workers sync.WaitGroup
	var lookupIssues []issue
	for index := 0; index < opts.workers; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for word := range jobs {
				languageName := lang.Name()
				key := languageName + "\x00" + word
				mutex.Lock()
				cached, ok := cache[key]
				mutex.Unlock()
				if ok {
					if cached != "" {
						mutex.Lock()
						merged[word] = cached
						mutex.Unlock()
					}
					continue
				}
				hint, err := lookup.Hint(context.Background(), word, languageName)
				mutex.Lock()
				if err != nil {
					cache[key] = ""
					lookupIssues = append(lookupIssues, issue{Language: string(lang), Level: string(lvl), Word: word, Reason: "lookup_error: " + err.Error()})
				} else if hint == "" {
					cache[key] = ""
				} else {
					cache[key] = hint
					merged[word] = hint
				}
				mutex.Unlock()
			}
		}()
	}
	for _, word := range words {
		jobs <- word
	}
	close(jobs)
	workers.Wait()

	missing := make([]string, 0)
	for _, word := range words {
		if strings.TrimSpace(merged[word]) == "" {
			missing = append(missing, word)
		}
	}
	return missing, lookupIssues
}

func readAlphabet(root string, lang game.Language) (map[string]bool, error) {
	path := filepath.Join(root, string(lang), "alphabet.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read alphabet: %w", err)
	}
	var alphabet dictionary.AlphabetFile
	if err := json.Unmarshal(raw, &alphabet); err != nil {
		return nil, fmt.Errorf("parse alphabet: %w", err)
	}
	if alphabet.Language != lang {
		return nil, fmt.Errorf("%s declares language %q", path, alphabet.Language)
	}
	letters := make(map[string]bool, len(alphabet.Letters))
	for _, spec := range alphabet.Letters {
		letters[normalizeWord(spec.Letter)] = true
	}
	return letters, nil
}

func loadExisting(path string, merged map[string]string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var existing dictionary.WordFile
	if err := json.Unmarshal(raw, &existing); err != nil {
		return fmt.Errorf("parse existing dictionary: %w", err)
	}
	for _, entry := range existing.Words {
		merged[normalizeWord(entry.Word)] = strings.TrimSpace(entry.Hint)
	}
	return nil
}

func writeWordFile(path string, lang game.Language, lvl game.Level, entries []dictionary.WordEntry) error {
	payload := dictionary.WordFile{
		SchemaVersion: 1,
		Language:      lang,
		Level:         lvl,
		Words:         entries,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func writeReport(path string, issues []issue) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"language", "level", "word", "reason"}); err != nil {
		return err
	}
	for _, issue := range issues {
		if err := writer.Write([]string{issue.Language, issue.Level, issue.Word, issue.Reason}); err != nil {
			return err
		}
	}
	return writer.Error()
}

func normalizeWord(word string) string {
	return strings.ToUpper(strings.TrimSpace(word))
}

func validateWord(word string, letters map[string]bool) string {
	length := utf8.RuneCountInString(word)
	if length < 2 {
		return "too_short"
	}
	if length > game.MaxRackSize {
		return "too_long"
	}
	for _, letter := range word {
		if !letters[string(letter)] {
			return "unknown_letter:" + string(letter)
		}
	}
	return ""
}

func outputPath(root string, lang game.Language, lvl game.Level) string {
	return filepath.Join(root, string(lang), strings.ToLower(string(lvl))+".json")
}

func parseWordListName(name string) (game.Language, game.Level, bool) {
	name = strings.TrimSuffix(name, "-words.csv")
	name = strings.TrimPrefix(name, "lingvoberry-")
	parts := strings.Split(name, "-")
	if len(parts) != 2 {
		return "", "", false
	}
	lang := game.Language(strings.ToLower(parts[0]))
	lvl := game.Level(strings.ToUpper(parts[1]))
	if !lang.Valid() || (lvl != "A0" && !lvl.Valid()) {
		return "", "", false
	}
	return lang, lvl, true
}

func languageAllowed(lang game.Language, value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	for _, item := range strings.Split(value, ",") {
		if game.Language(strings.ToLower(strings.TrimSpace(item))) == lang {
			return true
		}
	}
	return false
}

func levelAllowed(lvl game.Level, value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	for _, item := range strings.Split(value, ",") {
		if game.Level(strings.ToUpper(strings.TrimSpace(item))) == lvl {
			return true
		}
	}
	return false
}

func readEntries(path string) ([]dictionary.WordEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "{") {
		var payload dictionary.WordFile
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		return payload.Words, nil
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	wordIndex, hintIndex, rankIndex := -1, -1, -1
	for index, field := range rows[0] {
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "word":
			wordIndex = index
		case "hint":
			hintIndex = index
		case "rank":
			rankIndex = index
		}
	}
	start := 0
	if wordIndex >= 0 {
		start = 1
	} else {
		wordIndex = 0
		if len(rows[0]) > 1 {
			hintIndex = 1
		}
	}

	entries := make([]dictionary.WordEntry, 0, len(rows)-start)
	for index, row := range rows[start:] {
		if wordIndex >= len(row) {
			continue
		}
		if rankIndex >= 0 && rankIndex < len(row) && strings.TrimSpace(row[rankIndex]) == "#" {
			continue
		}
		word := strings.TrimSpace(row[wordIndex])
		if word == "" || strings.HasPrefix(word, "#") {
			continue
		}
		if start == 0 && index == 0 && strings.EqualFold(word, "word") {
			continue
		}
		hint := ""
		if hintIndex >= 0 && hintIndex < len(row) {
			hint = strings.TrimSpace(row[hintIndex])
		}
		entries = append(entries, dictionary.WordEntry{Word: word, Hint: hint})
	}
	return entries, nil
}
