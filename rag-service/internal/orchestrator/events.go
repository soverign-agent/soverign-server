// Package orchestrator implements the agentic RAG pipeline for compliance chat.
package orchestrator

import (
	"time"
)

// AddProgress appends a progress event to the pipeline state.
func (s *PipelineState) AddProgress(stage ProgressStage, message string, subQueries []string, resultCount int) {
	evt := ProgressEvent{
		Stage:       stage,
		Message:     message,
		Timestamp:   time.Now().UTC(),
		SubQueries:  subQueries,
		ResultCount: resultCount,
	}
	s.Progress = append(s.Progress, evt)
}

// ProgressChanReader turns a channel of progress events into a slice once the channel is closed.
func ProgressChanReader(ch <-chan ProgressEvent) []ProgressEvent {
	var events []ProgressEvent
	for evt := range ch {
		events = append(events, evt)
	}
	return events
}
