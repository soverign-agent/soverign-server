package eval

import (
	"strings"
	"unicode"
)

var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"of": {}, "in": {}, "on": {}, "at": {}, "to": {}, "for": {}, "and": {},
	"or": {}, "but": {}, "with": {}, "as": {}, "by": {}, "this": {}, "that": {},
	"it": {}, "be": {}, "been": {},
}

// tokenize lowercases s, splits on non-letter/digit boundaries, and returns the
// tokens in source order. Tokens shorter than two runes are dropped.
func tokenize(s string) []string {
	if s == "" {
		return nil
	}
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 2 {
			continue
		}
		out = append(out, f)
	}
	return out
}

// tokenizeContent strips stopwords and deduplicates. Used for set-based
// overlap math where word order is irrelevant.
func tokenizeContent(s string) []string {
	toks := tokenize(s)
	seen := make(map[string]struct{}, len(toks))
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		if _, drop := stopwords[t]; drop {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// bigrams returns the set of consecutive token pairs joined by '\x1f'. The
// separator is a control character that cannot appear in tokens.
func bigrams(tokens []string) map[string]struct{} {
	if len(tokens) < 2 {
		return map[string]struct{}{}
	}
	out := make(map[string]struct{}, len(tokens)-1)
	for i := 0; i < len(tokens)-1; i++ {
		out[tokens[i]+"\x1f"+tokens[i+1]] = struct{}{}
	}
	return out
}

func toSet(tokens []string) map[string]struct{} {
	out := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		out[t] = struct{}{}
	}
	return out
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
