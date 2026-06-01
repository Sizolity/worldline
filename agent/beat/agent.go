package beat

import (
	"context"

	agenttool "github.com/sizolity/worldline/agent/tool"
)

// Request describes what the beat agent should execute.
type Request struct {
	SystemPrompt string
	UserMessage  string
	Tools        []agenttool.Tool
	MaxStep      int
}

// Result is the final outcome of a beat agent run.
type Result struct {
	Narrative string
	ToolCalls []ToolCallRecord
	Err       error
}

// ToolCallRecord logs a single tool invocation.
type ToolCallRecord struct {
	Name      string
	Arguments string
	Result    string
}

// Stream provides incremental narrative output and a final result.
type Stream struct {
	Narrative <-chan string
	Done      <-chan Result
}

// Wait blocks until the stream completes and returns the final result.
func (s *Stream) Wait() Result {
	for range s.Narrative {
	}
	return <-s.Done
}

// Agent drives a multi-step LLM loop with tool calling.
type Agent interface {
	Run(ctx context.Context, req Request) *Stream
}
