package eval

import (
	"strings"
	"testing"
	"time"
)

func TestHallucination_EmptyOutput(t *testing.T) {
	p := &Pipeline{}
	got := p.scoreHallucination("", []string{"some context"})
	if got != 0 {
		t.Errorf("empty output = %v, want 0", got)
	}
}

func TestHallucination_NoContext(t *testing.T) {
	p := &Pipeline{}
	got := p.scoreHallucination("Paris is the capital of France.", nil)
	if got != 0 {
		t.Errorf("no-context = %v, want 0 (cannot disprove)", got)
	}
}

func TestHallucination_GroundedAnswer(t *testing.T) {
	p := &Pipeline{}
	got := p.scoreHallucination(
		"Paris is the capital of France.",
		[]string{"France's capital is Paris.", "The Eiffel Tower is located in Paris."},
	)
	if got >= 0.3 {
		t.Errorf("grounded answer = %v, want < 0.3", got)
	}
}

func TestHallucination_UnsupportedAnswer(t *testing.T) {
	p := &Pipeline{}
	got := p.scoreHallucination(
		"Paris is on Mars and was founded in 1850 by aliens.",
		[]string{"France's capital is Paris.", "Paris has 2 million people."},
	)
	if got <= 0.5 {
		t.Errorf("unsupported answer = %v, want > 0.5", got)
	}
}

func TestHallucination_PartialOverlap(t *testing.T) {
	p := &Pipeline{}
	got := p.scoreHallucination(
		"Paris has two million people and a major airport in 1850.",
		[]string{"Paris has two million people."},
	)
	if got < 0.2 || got > 0.8 {
		t.Errorf("partial overlap = %v, want in (0.2, 0.8)", got)
	}
}

func TestHallucination_LongInputCompletesQuickly(t *testing.T) {
	p := &Pipeline{}
	long := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 2000)
	ctx := []string{strings.Repeat("brown fox lazy dog ", 500)}
	start := time.Now()
	got := p.scoreHallucination(long, ctx)
	if d := time.Since(start); d > 250*time.Millisecond {
		t.Errorf("scoring took %v, want < 250ms", d)
	}
	if got < 0 || got > 1 {
		t.Errorf("score out of range: %v", got)
	}
}

func TestHallucination_OnlyStopwords(t *testing.T) {
	p := &Pipeline{}
	got := p.scoreHallucination("the a an is", []string{"random context here"})
	if got != 0 {
		t.Errorf("all stopwords = %v, want 0", got)
	}
}
