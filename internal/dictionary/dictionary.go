package dictionary

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cookiengineer/words-with-learners-app/internal/game"
)

type AlphabetFile struct {
	SchemaVersion int             `json:"schemaVersion"`
	Language      game.Language   `json:"language"`
	Letters       []game.TileSpec `json:"letters"`
	BlankCount    int             `json:"blankCount"`
}

type WordFile struct {
	SchemaVersion int           `json:"schemaVersion"`
	Language      game.Language `json:"language"`
	Level         game.Level    `json:"level"`
	Words         []WordEntry   `json:"words"`
}

type WordEntry struct {
	Word string `json:"word"`
	Hint string `json:"hint"`
}

type Entry struct {
	Word string
	Hint string
}

type Dictionary struct {
	language   game.Language
	level      game.Level
	specs      []game.TileSpec
	letterSet  map[string]bool
	words      map[string]string
	entries    []Entry
	validation []*Dictionary
}

func (dict *Dictionary) Language() game.Language {
	return dict.language
}

func (dict *Dictionary) Level() game.Level {
	return dict.level
}

func (dict *Dictionary) Specs() []game.TileSpec {
	return append([]game.TileSpec(nil), dict.specs...)
}

func (dict *Dictionary) Entries() []Entry {
	return append([]Entry(nil), dict.entries...)
}

func (dict *Dictionary) NormalizeLetter(letter string) string {
	letter = strings.TrimSpace(letter)
	if letter == "" {
		return ""
	}
	upper := strings.ToUpper(letter)
	if utf8.RuneCountInString(upper) != 1 {
		return ""
	}
	return upper
}

func (dict *Dictionary) NormalizeWord(word string) string {
	return strings.ToUpper(strings.TrimSpace(word))
}

func (dict *Dictionary) HasLetter(letter string) bool {
	return dict.letterSet[strings.ToUpper(letter)]
}

func (dict *Dictionary) ContainsWord(word string) bool {
	word = strings.ToUpper(word)
	for _, candidate := range dict.validation {
		if _, ok := candidate.words[word]; ok {
			return true
		}
	}
	return false
}

func (dict *Dictionary) HintFor(word string) string {
	return dict.words[strings.ToUpper(word)]
}

type Library struct {
	byKey map[string]*Dictionary
}

func Key(language game.Language, level game.Level) string {
	return string(language) + "/" + string(level)
}

func (library *Library) Get(language game.Language, level game.Level) (*Dictionary, error) {
	if library == nil {
		return nil, fmt.Errorf("dictionary library is missing")
	}
	dict, ok := library.byKey[Key(language, level)]
	if !ok {
		return nil, fmt.Errorf("no dictionary for %s %s", language.Name(), level)
	}
	return dict, nil
}

func LoadLibrary(fsys fs.FS) (*Library, error) {
	library := &Library{byKey: make(map[string]*Dictionary)}
	for _, language := range game.Languages {
		alphabetPath := path.Join(string(language), "alphabet.json")
		rawAlphabet, err := fs.ReadFile(fsys, alphabetPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", alphabetPath, err)
		}
		var alphabet AlphabetFile
		if err := json.Unmarshal(rawAlphabet, &alphabet); err != nil {
			return nil, fmt.Errorf("parse %s: %w", alphabetPath, err)
		}
		if alphabet.Language != language {
			return nil, fmt.Errorf("%s declares language %q", alphabetPath, alphabet.Language)
		}
		specs, letterSet, err := validateAlphabet(alphabet)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", alphabetPath, err)
		}
		languageDictionaries := make([]*Dictionary, 0, len(game.Levels))
		for _, level := range game.Levels {
			wordPath := path.Join(string(language), strings.ToLower(string(level))+".json")
			rawWords, err := fs.ReadFile(fsys, wordPath)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", wordPath, err)
			}
			var wordFile WordFile
			if err := json.Unmarshal(rawWords, &wordFile); err != nil {
				return nil, fmt.Errorf("parse %s: %w", wordPath, err)
			}
			if wordFile.Language != language || wordFile.Level != level {
				return nil, fmt.Errorf("%s declares %s %s", wordPath, wordFile.Language, wordFile.Level)
			}
			dict := &Dictionary{
				language:  language,
				level:     level,
				specs:     append([]game.TileSpec(nil), specs...),
				letterSet: letterSet,
				words:     make(map[string]string, len(wordFile.Words)),
			}
			for _, entry := range wordFile.Words {
				word := dict.NormalizeWord(entry.Word)
				if utf8.RuneCountInString(word) < 2 {
					return nil, fmt.Errorf("%s: word %q is too short", wordPath, entry.Word)
				}
				if utf8.RuneCountInString(word) > game.MaxRackSize {
					return nil, fmt.Errorf("%s: word %q is longer than the board", wordPath, entry.Word)
				}
				for _, letter := range word {
					if !letterSet[string(letter)] {
						return nil, fmt.Errorf("%s: word %q uses unknown letter %q", wordPath, entry.Word, string(letter))
					}
				}
				if _, exists := dict.words[word]; exists {
					return nil, fmt.Errorf("%s: duplicate word %q", wordPath, entry.Word)
				}
				dict.words[word] = strings.TrimSpace(entry.Hint)
				dict.entries = append(dict.entries, Entry{Word: word, Hint: strings.TrimSpace(entry.Hint)})
			}
			if len(dict.entries) == 0 {
				return nil, fmt.Errorf("%s: dictionary is empty", wordPath)
			}
			sort.Slice(dict.entries, func(i, j int) bool {
				return dict.entries[i].Word < dict.entries[j].Word
			})
			library.byKey[Key(language, level)] = dict
			languageDictionaries = append(languageDictionaries, dict)
		}
		for _, dict := range languageDictionaries {
			dict.validation = append(dict.validation, languageDictionaries...)
		}
	}
	return library, nil
}

func validateAlphabet(alphabet AlphabetFile) ([]game.TileSpec, map[string]bool, error) {
	letterSet := make(map[string]bool)
	specs := make([]game.TileSpec, 0, len(alphabet.Letters)+1)
	for _, spec := range alphabet.Letters {
		letter := strings.ToUpper(strings.TrimSpace(spec.Letter))
		if utf8.RuneCountInString(letter) != 1 {
			return nil, nil, fmt.Errorf("invalid letter %q", spec.Letter)
		}
		if spec.Count < 0 || spec.Value < 0 {
			return nil, nil, fmt.Errorf("invalid count or value for %q", letter)
		}
		if letterSet[letter] {
			return nil, nil, fmt.Errorf("duplicate letter %q", letter)
		}
		letterSet[letter] = true
		spec.Letter = letter
		spec.Blank = false
		specs = append(specs, spec)
	}
	if alphabet.BlankCount < 0 {
		return nil, nil, fmt.Errorf("invalid blank count")
	}
	if alphabet.BlankCount > 0 {
		specs = append(specs, game.TileSpec{Letter: "", Value: 0, Count: alphabet.BlankCount, Blank: true})
	}
	if len(letterSet) == 0 {
		return nil, nil, fmt.Errorf("alphabet is empty")
	}
	return specs, letterSet, nil
}
