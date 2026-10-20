package wiktionary

import "testing"

func TestExtractHintChoosesLanguageSectionAndSkipsExamples(t *testing.T) {
	raw := []byte(`<html><body>
<div><h2 id="English">English</h2><ol><li>English definition</li></ol></div>
<div><h2 id="German">German</h2>
<div><h3 id="Noun_1">Noun</h3><ol>
<li>(transitive) to <a href="go">go</a><span class="h-usage-example">Example sentence</span></li>
<li>to walk<div class="citation-whole">A citation</div></li>
</ol></div></div>
<div><h2 id="Russian">Russian</h2><ol><li>Russian definition</li></ol></div>
</body></html>`)
	hint := ExtractHint(raw, "German")
	if hint != "to go; to walk" {
		t.Fatalf("unexpected hint: %q", hint)
	}
}

func TestExtractHintFallsBackToFirstDefinitionList(t *testing.T) {
	raw := []byte(`<html><body>
<div><h2 id="Dutch">Dutch</h2>
<h3 id="Etymology_1">Etymology 1</h3>
<ol><li>a house or building</li><li>a household</li></ol>
</div>
</body></html>`)
	hint := ExtractHint(raw, "Dutch")
	if hint != "a house or building; a household" {
		t.Fatalf("unexpected hint: %q", hint)
	}
}

func TestExtractHintReturnsEmptyWithoutLanguage(t *testing.T) {
	raw := []byte(`<html><body><h2 id="English">English</h2><ol><li>word</li></ol></body></html>`)
	if hint := ExtractHint(raw, "Italian"); hint != "" {
		t.Fatalf("expected no hint, got %q", hint)
	}
}

func TestTitleVariants(t *testing.T) {
	variants := titleVariants("haus")
	expected := map[string]bool{"haus": true, "Haus": true, "HAUS": true}
	for _, variant := range variants {
		delete(expected, variant)
	}
	if len(expected) != 0 {
		t.Fatalf("missing variants: %#v", expected)
	}
}
