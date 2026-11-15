package wiktionary

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const clarificationMarker = "clarification of this definition is needed"

var maintenanceMarkers = []string{
	"this term needs a translation",
	"this term needs a definition",
	"please help out and add a",
	"then remove the text",
	"{{rfdef}}",
}

// danglingHintWords are words that almost always indicate the hint was cut off
// mid-sentence. Prepositions are intentionally excluded because short, valid
// hints such as "in view of" end with one.
var danglingHintWords = map[string]bool{
	"a": true, "an": true, "the": true,
	"and": true, "or": true,
	"also": true, "especially": true, "particularly": true,
	"usually": true, "often": true, "typically": true,
	"such": true, "including": true, "generally": true,
	"normally": true, "mainly": true, "mostly": true,
	"e.g.": true, "i.e.": true, "eg": true, "ie": true,
	"this": true, "these": true, "those": true,
	"that": true, "which": true, "who": true, "whom": true, "whose": true,
}

var danglingAdverbWords = map[string]bool{
	"and": true, "or": true,
	"also": true, "especially": true, "particularly": true,
	"usually": true, "often": true, "typically": true,
	"such": true, "including": true, "generally": true,
	"normally": true, "mainly": true, "mostly": true,
	"e.g.": true, "i.e.": true, "eg": true, "ie": true,
}

// NormalizeHint cleans up hints extracted from Wiktionary. It removes
// editorial maintenance notes and trims incomplete trailing clauses that were
// introduced when a definition was cut to maxHintLength.
func NormalizeHint(hint string) string {
	text := normalizeHintOnce(hint)
	for attempt := 0; attempt < 10; attempt++ {
		next := normalizeHintOnce(text)
		if next == text {
			break
		}
		text = next
	}
	return text
}

func normalizeHintOnce(hint string) string {
	text := tidyHint(hint)
	if text == "" {
		return ""
	}
	text = stripClarifications(text)
	text = stripMaintenance(text)
	text = tidyHint(text)
	text = trimBrokenTail(text)
	return tidyHint(text)
}

func tidyHint(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	replacer := strings.NewReplacer(
		" ,", ",",
		" ;", ";",
		" :", ":",
		" .", ".",
		"( ", "(",
		" )", ")",
	)
	return strings.TrimSpace(replacer.Replace(text))
}

func stripClarifications(text string) string {
	for {
		lower := strings.ToLower(text)
		index := strings.Index(lower, clarificationMarker)
		if index < 0 {
			break
		}
		open := lastUnclosedOpen(text, index)
		if open < 0 {
			text = strings.TrimSpace(text[:index] + text[index+len(clarificationMarker):])
			continue
		}
		close := matchingClose(text, open)
		if close < 0 {
			text = text[:open]
			break
		}
		text = text[:open] + text[close+1:]
	}
	return tidyHint(text)
}

func lastUnclosedOpen(text string, limit int) int {
	stack := make([]int, 0, 4)
	for index := 0; index < limit && index < len(text); index++ {
		switch text[index] {
		case '(', '[', '{':
			stack = append(stack, index)
		case ')', ']', '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) == 0 {
		return -1
	}
	return stack[len(stack)-1]
}

func matchingClose(text string, open int) int {
	depth := 0
	for index := open; index < len(text); index++ {
		switch text[index] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func stripMaintenance(text string) string {
	segments := strings.Split(text, "; ")
	kept := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = stripMaintenanceSegment(segment)
		if segment != "" {
			kept = append(kept, segment)
		}
	}
	return strings.Join(kept, "; ")
}

func stripMaintenanceSegment(segment string) string {
	lower := strings.ToLower(segment)
	cut := -1
	for _, marker := range maintenanceMarkers {
		if index := strings.Index(lower, marker); index >= 0 && (cut < 0 || index < cut) {
			cut = index
		}
	}
	if cut >= 0 {
		segment = segment[:cut]
	}
	return strings.TrimRight(strings.TrimSpace(segment), " \t\r\n,;:-–—")
}

func trimBrokenTail(text string) string {
	if text == "" || !needsInitialCleanup(text) {
		return text
	}
	removed := false
	for attempt := 0; attempt < 20; attempt++ {
		if open := unbalancedOpenIndex(text); open >= 0 {
			text = trimHintCut(text[:open])
			removed = true
			continue
		}
		if trimmed := trimHintCut(text); trimmed != text {
			text = trimmed
			removed = true
			continue
		}
		if isDanglingTailWord(lastWord(text)) {
			next := removeLastWord(text)
			if next == text {
				break
			}
			text = trimHintCut(next)
			removed = true
			continue
		}
		break
	}
	if removed {
		text = trimShortCommaTail(text)
	}
	return tidyHint(text)
}

func needsInitialCleanup(text string) bool {
	if text == "" {
		return false
	}
	if unbalancedOpenIndex(text) >= 0 {
		return true
	}
	last := lastRune(text)
	switch last {
	case ',', ';', ':':
		return true
	case '.', '!', '?', '…', ')', ']', '}':
		return false
	}
	length := utf8.RuneCountInString(text)
	if length >= 165 {
		return true
	}
	if length >= 100 && danglingAdverbWords[lastWord(text)] {
		return true
	}
	return length >= 150 && danglingHintWords[lastWord(text)]
}

func isDanglingTailWord(word string) bool {
	if danglingHintWords[word] {
		return true
	}
	switch word {
	case "of", "for", "to", "in", "with", "on", "by", "from", "as", "at",
		"into", "onto", "upon", "over", "under", "between", "among", "about",
		"against", "within", "without", "through", "during", "before", "after",
		"above", "below", "than", "via", "per":
		return true
	}
	return false
}

func removeLastWord(text string) string {
	index := strings.LastIndexFunc(text, unicode.IsSpace)
	if index < 0 {
		return ""
	}
	return text[:index]
}

func trimShortCommaTail(text string) string {
	index := lastTopLevelIndex(text, ',')
	if index < 0 {
		return text
	}
	tail := strings.TrimSpace(text[index+1:])
	if utf8.RuneCountInString(tail) >= 40 {
		return text
	}
	return trimHintCut(text[:index])
}

func lastTopLevelIndex(text string, target byte) int {
	depth := 0
	last := -1
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && text[index] == target {
			last = index
		}
	}
	return last
}

func trimHintCut(text string) string {
	return strings.TrimRight(strings.TrimSpace(text), " \t\r\n,;:-–—")
}

func unbalancedOpenIndex(text string) int {
	stack := make([]int, 0, 4)
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '(', '[', '{':
			stack = append(stack, index)
		case ')', ']', '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) == 0 {
		return -1
	}
	return stack[len(stack)-1]
}

func lastRune(text string) rune {
	value, _ := utf8.DecodeLastRuneInString(text)
	return value
}

func lastWord(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	word := fields[len(fields)-1]
	word = strings.TrimFunc(word, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.'
	})
	return strings.ToLower(word)
}
