package einostructured

import (
	"context"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/agent/chat"
	einochat "github.com/sizolity/worldline/agent/eino/chat"
	"github.com/sizolity/worldline/agent/structured"
)

var lenientJSON = sonic.Config{NoValidateJSONSkip: true}.Froze()

// ToolCallAgent extracts T via forced tool calling.
type ToolCallAgent[T any] struct {
	CM       einomodel.ToolCallingChatModel
	ToolName string
	ToolDesc string
}

// NewToolCall creates a ToolCallAgent that forces a tool call to extract T.
func NewToolCall[T any](cm einomodel.ToolCallingChatModel, toolName, toolDesc string) *ToolCallAgent[T] {
	return &ToolCallAgent[T]{CM: cm, ToolName: toolName, ToolDesc: toolDesc}
}

func (a *ToolCallAgent[T]) Call(ctx context.Context, messages []chat.Message) (T, error) {
	var zero T
	toolInfo, err := utils.GoStruct2ToolInfo[T](a.ToolName, a.ToolDesc)
	if err != nil {
		return zero, fmt.Errorf("build tool schema: %w", err)
	}
	bound, err := a.CM.WithTools([]*schema.ToolInfo{toolInfo})
	if err != nil {
		return zero, fmt.Errorf("bind tool: %w", err)
	}
	resp, err := bound.Generate(ctx, einochat.ToEinoMessages(messages), einomodel.WithToolChoice(schema.ToolChoiceForced))
	if err != nil {
		return zero, fmt.Errorf("generate: %w", err)
	}
	parser := schema.NewMessageJSONParser[T](&schema.MessageJSONParseConfig{
		ParseFrom: schema.MessageParseFromToolCall,
	})
	result, err := parser.Parse(ctx, resp)
	if err != nil {
		return zero, fmt.Errorf("parse: %w", err)
	}
	return result, nil
}

var _ structured.Agent[any] = (*ToolCallAgent[any])(nil)

// JSONObjectAgent extracts T via JSON response format.
type JSONObjectAgent[T any] struct {
	CM einomodel.ToolCallingChatModel
}

// NewJSONObject creates a JSONObjectAgent that parses T from JSON content.
func NewJSONObject[T any](cm einomodel.ToolCallingChatModel) *JSONObjectAgent[T] {
	return &JSONObjectAgent[T]{CM: cm}
}

var jsonObjectResponseFormat = map[string]any{
	"response_format": map[string]any{"type": "json_object"},
}

func (a *JSONObjectAgent[T]) Call(ctx context.Context, messages []chat.Message) (T, error) {
	var zero T
	resp, err := a.CM.Generate(ctx, einochat.ToEinoMessages(messages), openai.WithExtraFields(jsonObjectResponseFormat))
	if err != nil {
		return zero, fmt.Errorf("generate: %w", err)
	}
	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return zero, fmt.Errorf("empty content")
	}
	var result T
	if err := sonic.UnmarshalString(content, &result); err == nil {
		return result, nil
	}
	if err := lenientJSON.UnmarshalFromString(content, &result); err == nil {
		return result, nil
	}
	return zero, fmt.Errorf("parse: failed to unmarshal JSON response")
}

var _ structured.Agent[any] = (*JSONObjectAgent[any])(nil)
