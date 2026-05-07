package eval

import "testing"

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   \t\n", []string{}},
		{"basic sentence", "Hello, World!", []string{"hello", "world"}},
		{"mixed punctuation", "Paris's capital, isn't it?", []string{"paris", "capital", "isn", "it"}},
		{"unicode letters", "café résumé", []string{"café", "résumé"}},
		{"drops single chars", "I am a", []string{"am"}},
		{"keeps digits", "year 2026", []string{"year", "2026"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenize(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("tokenize(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("tokenize(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestTokenizeContent_DropsStopwordsAndDeduplicates(t *testing.T) {
	in := "The cat is on the mat and the cat is happy"
	got := tokenizeContent(in)
	want := []string{"cat", "mat", "happy"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBigrams(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		want   int
	}{
		{"empty", nil, 0},
		{"single", []string{"a"}, 0},
		{"three tokens", []string{"a", "b", "c"}, 2},
		{"duplicates collapse", []string{"a", "b", "a", "b"}, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(bigrams(tc.tokens)); got != tc.want {
				t.Errorf("bigrams len = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestBigrams_Content(t *testing.T) {
	bg := bigrams([]string{"paris", "capital", "france"})
	if _, ok := bg["paris\x1fcapital"]; !ok {
		t.Errorf("missing paris-capital bigram: %v", bg)
	}
	if _, ok := bg["capital\x1ffrance"]; !ok {
		t.Errorf("missing capital-france bigram: %v", bg)
	}
}

func TestClamp01(t *testing.T) {
	tests := []struct {
		in, want float64
	}{
		{-0.5, 0}, {0, 0}, {0.5, 0.5}, {1, 1}, {1.5, 1},
	}
	for _, tc := range tests {
		if got := clamp01(tc.in); got != tc.want {
			t.Errorf("clamp01(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestToSet(t *testing.T) {
	s := toSet([]string{"a", "b", "a"})
	if len(s) != 2 {
		t.Errorf("set size = %d, want 2", len(s))
	}
	if _, ok := s["a"]; !ok {
		t.Errorf("missing a")
	}
}
