package eval

import "strings"

// scoreHallucination returns the share of OutputText not grounded in the
// supplied RAG chunks, blended from unigram coverage gap (80%) and bigram
// unsupported-rate (20%). Coverage carries the primary signal because
// short, paraphrased answers preserve content tokens but not n-gram order
// (e.g. "X is the capital of Y" vs "Y's capital is X"). Bigrams provide a
// secondary penalty for wholly fabricated multi-word claims. Coverage uses
// the content-only token set so trivial stopword agreement does not
// inflate it; bigrams use raw tokens so n-gram statistics are stable.
func (p *Pipeline) scoreHallucination(output string, ragContext []string) float64 {
	if strings.TrimSpace(output) == "" {
		return 0
	}
	if len(ragContext) == 0 {
		return 0
	}

	outputContent := tokenizeContent(output)
	if len(outputContent) == 0 {
		return 0
	}

	rawOutput := tokenize(output)
	rawContext := make([]string, 0, 64)
	contextContent := make([]string, 0, 64)
	for _, chunk := range ragContext {
		rawContext = append(rawContext, tokenize(chunk)...)
		contextContent = append(contextContent, tokenizeContent(chunk)...)
	}
	contextSet := toSet(contextContent)

	outBigrams := bigrams(rawOutput)
	ctxBigrams := bigrams(rawContext)

	var unsupported float64
	if len(outBigrams) == 0 {
		unsupported = 0
	} else {
		missing := 0
		for bg := range outBigrams {
			if _, ok := ctxBigrams[bg]; !ok {
				missing++
			}
		}
		unsupported = float64(missing) / float64(len(outBigrams))
	}

	matched := 0
	for _, t := range outputContent {
		if _, ok := contextSet[t]; ok {
			matched++
		}
	}
	coverage := float64(matched) / float64(len(outputContent))

	return clamp01((1-coverage)*0.8 + unsupported*0.2)
}
