package structured

import (
	"context"

	"github.com/sizolity/worldline/agent/chat"
)

// Mode controls how structured output is extracted from the LLM.
type Mode int

const (
	ModeToolCall   Mode = iota // forced tool-call + parse from tool arguments
	ModeJSONObject             // response_format=json_object + parse from content
)

// Config holds settings for a structured extraction agent.
type Config struct {
	Mode        Mode
	ToolName    string // used when Mode=ModeToolCall
	Description string // used when Mode=ModeToolCall
}

// Agent extracts a typed value from an LLM conversation.
type Agent[T any] interface {
	Call(ctx context.Context, messages []chat.Message) (T, error)
}
