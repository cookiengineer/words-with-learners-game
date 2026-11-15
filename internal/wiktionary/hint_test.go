package wiktionary

import "testing"

func TestNormalizeHintTrimsBrokenTail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "unclosed parenthesis after semicolon",
			in:   "to have; (auxiliary, the non-perfect forms followed by the masculine singular past participle, used to form composite (perfect) tenses in the active and dative conjugations of the",
			want: "to have",
		},
		{
			name: "dangling article after comma",
			in:   "Any of many minute arachnids which, along with the ticks, comprise subclass Acari (syn. Acarina).; A small coin formerly circulated in England, rated at about a third of a",
			want: "Any of many minute arachnids which, along with the ticks, comprise subclass Acari (syn. Acarina).; A small coin formerly circulated in England",
		},
		{
			name: "dangling article after semicolon",
			in:   "Reference book (a book, usually alphabetically ordered, in which definitions and facts can be looked up, such as an encyclopaedia, dictionary, etc.); lexicon (the vocabulary of a",
			want: "Reference book (a book, usually alphabetically ordered, in which definitions and facts can be looked up, such as an encyclopaedia, dictionary, etc.); lexicon",
		},
		{
			name: "unclosed parenthesis",
			in:   "to understand, to comprehend (a line of thought, a development, a motivation; with a focus on how something derives from previous steps or conditions); to retrace (the steps of a",
			want: "to understand, to comprehend (a line of thought, a development, a motivation; with a focus on how something derives from previous steps or conditions); to retrace",
		},
		{
			name: "unclosed bracket",
			in:   "to think [with an accusative object]; to remember, to be reminded of [with an accusative or (higher register) genitive",
			want: "to think [with an accusative object]; to remember",
		},
		{
			name: "complete leading sentence",
			in:   "The policy of racial segregation in South Africa from 1948 to 1994.; Any similar policy of racial segregation or separation and discrimination, particularly when in favor of a",
			want: "The policy of racial segregation in South Africa from 1948 to 1994.; Any similar policy of racial segregation or separation and discrimination",
		},
		{
			name: "short valid hint",
			in:   "alternative spelling of a",
			want: "alternative spelling of a",
		},
		{
			name: "short phrasal hint",
			in:   "thanks to, because of, courtesy of",
			want: "thanks to, because of, courtesy of",
		},
		{
			name: "curated city hint",
			in:   "capital city of the United Kingdom",
			want: "capital city of the United Kingdom",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeHint(test.in); got != test.want {
				t.Fatalf("NormalizeHint() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeHintIsIdempotent(t *testing.T) {
	hints := []string{
		"Any of many minute arachnids which, along with the ticks, comprise subclass Acari (syn. Acarina).; A small coin formerly circulated in England, rated at about a third of a",
		"Reference book (a book, usually alphabetically ordered, in which definitions and facts can be looked up, such as an encyclopaedia, dictionary, etc.); lexicon (the vocabulary of a",
		"This term needs a translation to English. Please help out and add a translation, then remove the text {{rfdef}}.; a milanesa made with chicken",
	}
	for _, hint := range hints {
		once := NormalizeHint(hint)
		twice := NormalizeHint(once)
		if once != twice {
			t.Fatalf("NormalizeHint is not idempotent: first %q, second %q", once, twice)
		}
	}
}

func TestNormalizeHintRemovesEditorialText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "maintenance-only hint",
			in:   "This term needs a translation to English. Please help out and add a translation, then remove the text {{rfdef}}.",
			want: "",
		},
		{
			name: "maintenance after valid hint",
			in:   "to state, assert; This term needs a translation to English. Please help out and add a translation, then remove the text {{rfdef}}.",
			want: "to state, assert",
		},
		{
			name: "maintenance before valid hint",
			in:   "This term needs a translation to English. Please help out and add a translation, then remove the text {{rfdef}}.; a milanesa made with chicken",
			want: "a milanesa made with chicken",
		},
		{
			name: "clarification note",
			in:   "sheriff (clarification of this definition is needed)",
			want: "sheriff",
		},
		{
			name: "nested clarification note",
			in:   "balance (clarification of this definition is needed (explain the difference)); scales",
			want: "balance; scales",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeHint(test.in); got != test.want {
				t.Fatalf("NormalizeHint() = %q, want %q", got, test.want)
			}
		})
	}
}
