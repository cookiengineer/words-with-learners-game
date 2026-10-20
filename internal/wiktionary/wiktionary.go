package wiktionary

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const maxHintLength = 180

type Lookup struct {
	ZimPath    string
	Tool       string
	SearchTool string
}

func (lookup Lookup) Hint(ctx context.Context, word string, language string) (string, error) {
	if lookup.Tool == "" {
		lookup.Tool = "zimdump"
	}
	if lookup.SearchTool == "" {
		lookup.SearchTool = "zimsearch"
	}
	fallback := ""
	for _, candidate := range lookupVariants(word, language) {
		requestContext, cancel := context.WithTimeout(ctx, 30*time.Second)
		raw, err := lookup.show(requestContext, candidate)
		cancel()
		if err != nil {
			return "", err
		}
		if len(raw) == 0 {
			continue
		}
		hint := ExtractHint(raw, language)
		if hint == "" {
			continue
		}
		if !isFormOf(hint) {
			return hint, nil
		}
		if fallback == "" {
			fallback = hint
		}
	}
	if hint := lookup.search(ctx, word, language); hint != "" {
		if !isFormOf(hint) {
			return hint, nil
		}
		if fallback == "" {
			fallback = hint
		}
	}
	return fallback, nil
}

func (lookup Lookup) search(ctx context.Context, word string, language string) string {
	requestContext, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	command := exec.CommandContext(requestContext, lookup.SearchTool, lookup.ZimPath, word)
	output, err := command.Output()
	if err != nil {
		return ""
	}
	fallback := ""
	attempts := 0
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 3 || !strings.HasPrefix(parts[0], "score ") {
			continue
		}
		score, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(parts[0], "score ")))
		if err != nil || score < 50 {
			continue
		}
		title := strings.TrimSpace(parts[2])
		if title == "" || strings.ContainsAny(title, " \t") || !similarWord(word, title) {
			continue
		}
		attempts++
		if attempts > 6 {
			break
		}
		requestContext, cancel := context.WithTimeout(ctx, 30*time.Second)
		raw, err := lookup.show(requestContext, title)
		cancel()
		if err != nil || len(raw) == 0 {
			continue
		}
		hint := ExtractHint(raw, language)
		if hint == "" {
			continue
		}
		if !isFormOf(hint) {
			return hint
		}
		if fallback == "" {
			fallback = hint
		}
	}
	return fallback
}

func similarWord(word string, title string) bool {
	wordRunes := []rune(strings.ToLower(word))
	titleRunes := []rune(strings.ToLower(title))
	if len(wordRunes) == 0 || len(titleRunes) == 0 {
		return false
	}
	minimum := 4
	if len(wordRunes) < minimum {
		minimum = len(wordRunes)
	}
	common := 0
	for common < len(wordRunes) && common < len(titleRunes) && wordRunes[common] == titleRunes[common] {
		common++
	}
	return common >= minimum
}

func lookupVariants(word string, language string) []string {
	variants := titleVariants(word)
	if strings.EqualFold(language, "Russian") {
		extra := make([]string, 0, len(variants))
		for _, variant := range variants {
			swapped := strings.Map(func(r rune) rune {
				switch r {
				case 'е':
					return 'ё'
				case 'ё':
					return 'е'
				case 'Е':
					return 'Ё'
				case 'Ё':
					return 'Е'
				default:
					return r
				}
			}, variant)
			if swapped != variant {
				extra = append(extra, swapped)
			}
		}
		variants = append(variants, extra...)
	}
	seen := make(map[string]bool, len(variants))
	result := make([]string, 0, len(variants))
	for _, variant := range variants {
		if variant == "" || seen[variant] {
			continue
		}
		seen[variant] = true
		result = append(result, variant)
	}
	return result
}

func isFormOf(hint string) bool {
	lower := strings.ToLower(strings.TrimSpace(hint))
	for _, prefix := range []string{
		"form of", "inflection of", "plural of", "singular of", "variant of",
		"alternative form", "alternative spelling", "archaic form", "obsolete form",
		"misspelling of", "abbreviation of", "initialism of", "acronym of",
		"past participle", "present participle", "first-person", "second-person",
		"third-person", "singular imperative", "imperative of", "dative", "accusative", "genitive",
		"nominative", "comparative of", "superlative of", "synonym of",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func (lookup Lookup) show(ctx context.Context, title string) ([]byte, error) {
	command := exec.CommandContext(ctx, lookup.Tool, "show", "--url="+title, lookup.ZimPath)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, nil
	}
	return nil, err
}

func titleVariants(word string) []string {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil
	}
	variants := []string{word, strings.ToLower(word)}
	lowered := []rune(strings.ToLower(word))
	if len(lowered) > 0 {
		lowered[0] = unicode.ToUpper(lowered[0])
		variants = append(variants, string(lowered))
	}
	runes := []rune(word)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		variants = append(variants, string(runes))
	}
	variants = append(variants, strings.ToUpper(word))
	seen := make(map[string]bool, len(variants))
	result := make([]string, 0, len(variants))
	for _, variant := range variants {
		if variant == "" || seen[variant] {
			continue
		}
		seen[variant] = true
		result = append(result, variant)
	}
	return result
}

func ExtractHint(raw []byte, language string) string {
	document, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return ""
	}
	nodes := make([]*html.Node, 0, 2048)
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		nodes = append(nodes, node)
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(document)

	start := -1
	for index, node := range nodes {
		if node.Type != html.ElementNode || node.Data != "h2" {
			continue
		}
		if strings.EqualFold(attribute(node, "id"), language) {
			start = index
			break
		}
	}
	if start < 0 {
		return ""
	}
	currentHeading := ""
	for index := start + 1; index < len(nodes); index++ {
		node := nodes[index]
		if node.Type != html.ElementNode {
			continue
		}
		if node.Data == "h2" {
			break
		}
		if node.Data == "h3" || node.Data == "h4" {
			currentHeading = attribute(node, "id")
			continue
		}
		if strings.HasPrefix(currentHeading, "Proper_noun") {
			continue
		}
		if node.Data != "ol" || skipClasses(node) {
			continue
		}
		if hint := extractList(node); hint != "" {
			return hint
		}
	}
	return ""
}

func extractList(list *html.Node) string {
	definitions := make([]string, 0, 2)
	seen := make(map[string]bool, 2)
	for item := list.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != html.ElementNode || item.Data != "li" || skipNode(item) {
			continue
		}
		text := cleanDefinition(textContent(item))
		if !usefulDefinition(text) || seen[text] {
			continue
		}
		seen[text] = true
		definitions = append(definitions, text)
		if len(definitions) == 2 {
			break
		}
	}
	if len(definitions) == 0 {
		return ""
	}
	return truncate(strings.Join(definitions, "; "), maxHintLength)
}

func textContent(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	if node.Type != html.ElementNode {
		var builder strings.Builder
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			builder.WriteString(textContent(child))
		}
		return builder.String()
	}
	if skipNode(node) {
		return ""
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(textContent(child))
	}
	return builder.String()
}

func skipNode(node *html.Node) bool {
	switch node.Data {
	case "style", "script", "table", "ul", "dl", "ol", "sup", "audio", "figure", "nav":
		return true
	}
	return skipClasses(node)
}

func skipClasses(node *html.Node) bool {
	classes := strings.Fields(attribute(node, "class"))
	for _, class := range classes {
		switch class {
		case "h-usage-example", "e-example", "e-quotation", "e-translation",
			"citation-whole", "cited-source", "nyms", "synonym", "antonym",
			"reflist", "references", "reference", "mw-references-wrap",
			"translations", "translation", "sister-project", "navbox",
			"thumb", "hatnote", "disambig-see-also", "quotations":
			return true
		}
	}
	return false
}

func attribute(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func cleanDefinition(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	for {
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "(") {
			if end := strings.Index(trimmed, ")"); end > 1 && end < 80 {
				text = strings.TrimSpace(trimmed[end+1:])
				continue
			}
		}
		if strings.HasPrefix(trimmed, "[") {
			if end := strings.Index(trimmed, "]"); end > 1 && end < 80 {
				text = strings.TrimSpace(trimmed[end+1:])
				continue
			}
		}
		break
	}
	text = strings.TrimSpace(text)
	text = strings.Trim(text, " :;,-–—")
	return text
}

func usefulDefinition(text string) bool {
	if utf8.RuneCountInString(text) < 2 {
		return false
	}
	lower := strings.ToLower(text)
	for _, prefix := range []string{
		"alternative forms", "see also", "synonyms", "antonyms", "references",
		"further reading", "anagrams", "quotations", "descendants",
	} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cut := runes[:limit]
	for index := len(cut) - 1; index > limit/2; index-- {
		if unicode.IsSpace(cut[index]) {
			return strings.TrimSpace(string(cut[:index]))
		}
	}
	return strings.TrimSpace(string(cut))
}
