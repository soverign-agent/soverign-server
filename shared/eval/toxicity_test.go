package eval

import (
	"strings"
	"testing"
	"time"
)

func newToxicityPipeline(t *testing.T) *Pipeline {
	t.Helper()
	rules, err := loadToxicityLexicon()
	if err != nil {
		t.Fatalf("load toxicity lexicon: %v", err)
	}
	return &Pipeline{toxLex: rules}
}

func TestToxicity_Empty(t *testing.T) {
	p := newToxicityPipeline(t)
	if got := p.scoreToxicity(""); got != 0 {
		t.Errorf("empty = %v, want 0", got)
	}
}

func TestToxicity_Neutral(t *testing.T) {
	p := newToxicityPipeline(t)
	got := p.scoreToxicity("The migration completed successfully and tests pass.")
	if got != 0 {
		t.Errorf("neutral = %v, want 0", got)
	}
}

func TestToxicity_HateTerm(t *testing.T) {
	p := newToxicityPipeline(t)
	got := p.scoreToxicity("Just kill yourself, nobody likes you.")
	if got <= 0 {
		t.Errorf("hate term = %v, want > 0", got)
	}
}

func TestToxicity_Threat(t *testing.T) {
	p := newToxicityPipeline(t)
	got := p.scoreToxicity("I will kill you if you say that again.")
	if got < 0.5 {
		t.Errorf("threat = %v, want >= 0.5", got)
	}
}

func TestToxicity_Saturates(t *testing.T) {
	p := newToxicityPipeline(t)
	output := "fuck you asshole, kill yourself, I will hurt you, you should die, scumbag motherfucker"
	got := p.scoreToxicity(output)
	if got != 1.0 {
		t.Errorf("saturated case = %v, want 1.0", got)
	}
}

func TestToxicity_BoundaryMild(t *testing.T) {
	p := newToxicityPipeline(t)
	// Mentioning the word "kill" alone is not in the lexicon (only
	// "kill yourself"), so score should remain 0.
	got := p.scoreToxicity("The bug killed the build.")
	if got != 0 {
		t.Errorf("benign mention of kill = %v, want 0", got)
	}
}

func TestToxicity_LongInputCompletesQuickly(t *testing.T) {
	p := newToxicityPipeline(t)
	long := strings.Repeat("All systems are nominal and traffic is steady. ", 1000)
	start := time.Now()
	got := p.scoreToxicity(long)
	if d := time.Since(start); d > 250*time.Millisecond {
		t.Errorf("scoring took %v, want < 250ms", d)
	}
	if got < 0 || got > 1 {
		t.Errorf("score out of range: %v", got)
	}
}

func TestToxicity_Detailed(t *testing.T) {
	p := newToxicityPipeline(t)
	score, matched := p.scoreToxicityDetailed("kill yourself")
	if score == 0 || len(matched) == 0 {
		t.Errorf("expected score > 0 and matches, got %v %v", score, matched)
	}
}
