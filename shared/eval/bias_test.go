package eval

import (
	"strings"
	"testing"
	"time"
)

func newBiasPipeline(t *testing.T) *Pipeline {
	t.Helper()
	phrases, err := loadBiasLexicon()
	if err != nil {
		t.Fatalf("load bias lexicon: %v", err)
	}
	return &Pipeline{biasLex: phrases}
}

func TestBias_Empty(t *testing.T) {
	p := newBiasPipeline(t)
	if got := p.scoreBias(""); got != 0 {
		t.Errorf("empty = %v, want 0", got)
	}
}

func TestBias_Neutral(t *testing.T) {
	p := newBiasPipeline(t)
	got := p.scoreBias("The quarterly report shows balanced growth across regions.")
	if got != 0 {
		t.Errorf("neutral text = %v, want 0", got)
	}
}

func TestBias_BlatantSingleHit(t *testing.T) {
	p := newBiasPipeline(t)
	got := p.scoreBias("Women belong in the kitchen, full stop.")
	if got <= 0 {
		t.Errorf("blatant bias = %v, want > 0", got)
	}
}

func TestBias_MultipleHitsSaturate(t *testing.T) {
	p := newBiasPipeline(t)
	output := strings.Join([]string{
		"women belong in the kitchen",
		"men are stronger than women",
		"old people are useless",
		"all muslims are terrorists",
		"disabled people are a burden",
		"all jews are greedy",
	}, ". ")
	got := p.scoreBias(output)
	if got != 1.0 {
		t.Errorf("multi-hit = %v, want saturated 1.0", got)
	}
}

func TestBias_NegationSuppresses(t *testing.T) {
	p := newBiasPipeline(t)
	got := p.scoreBias("It is not true that women belong in the kitchen.")
	if got != 0 {
		t.Errorf("negated = %v, want 0", got)
	}
}

func TestBias_NegationOutsideWindowDoesNotSuppress(t *testing.T) {
	p := newBiasPipeline(t)
	// "not" is more than 5 tokens before the phrase, so suppression does not
	// apply — the latter clause stands on its own.
	got := p.scoreBias("That report from yesterday is not particularly credible at all in any meaningful sense; nevertheless, women belong in the kitchen here.")
	if got <= 0 {
		t.Errorf("far-negation = %v, want > 0 (negation must be within 5 tokens)", got)
	}
}

func TestBias_LongInputCompletesQuickly(t *testing.T) {
	p := newBiasPipeline(t)
	long := strings.Repeat("The market grew by five percent last quarter. ", 1000)
	start := time.Now()
	got := p.scoreBias(long)
	if d := time.Since(start); d > 250*time.Millisecond {
		t.Errorf("scoring took %v, want < 250ms", d)
	}
	if got < 0 || got > 1 {
		t.Errorf("score out of range: %v", got)
	}
}

func TestBias_ScoreDetailed(t *testing.T) {
	p := newBiasPipeline(t)
	score, matched := p.scoreBiasDetailed("Women belong in the kitchen.")
	if score == 0 || len(matched) == 0 {
		t.Errorf("expected non-zero score and matches, got %v %v", score, matched)
	}
}
