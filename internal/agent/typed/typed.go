// Package typed implements single-shot, type-safe extraction of a Go value T
// from an LLM conversation. Messages are passed as the native eino
// [*schema.Message] type and the underlying model is the native eino
// [model.ToolCallingChatModel] — no intermediate translation layer.
//
// Two extraction strategies are provided:
//
//   - [ToolCallAgent] forces the model to invoke a synthetic tool whose
//     parameter schema is derived from T's struct tags. The tool arguments
//     are then parsed into T.
//   - [JSONObjectAgent] requests `response_format=json_object` from the
//     underlying OpenAI-compatible endpoint and unmarshals the assistant
//     message body into T.
//
// Use [NewToolCall] when the schema is rich (nested objects, enums, arrays)
// and the model needs guidance; use [NewJSONObject] for simple, flat shapes
// or when the model's tool-calling fidelity is uneven.
package typed

import (
	"context"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// lenientJSON tolerates trailing commas / loose number formatting that some
// LLMs occasionally emit even in strict json_object mode. We try the strict
// parser first and only fall back if it rejects the payload, so the lenient
// branch is a recovery path rather than the default.
var lenientJSON = sonic.Config{NoValidateJSONSkip: true}.Froze()

// Agent extracts a typed value from an LLM conversation. Implementations
// in this package are [*ToolCallAgent] and [*JSONObjectAgent].
type Agent[T any] interface {
	Call(ctx context.Context, messages []*schema.Message) (T, error)
}

// ToolCallAgent extracts T by binding a synthetic tool to the model and
// forcing a single tool call. The tool's parameter schema is derived from
// T's struct tags via eino's [utils.GoStruct2ToolInfo].
type ToolCallAgent[T any] struct {
	CM       model.ToolCallingChatModel
	ToolName string
	ToolDesc string
}

// NewToolCall constructs a [ToolCallAgent]. ToolName and ToolDesc are
// surfaced to the LLM verbatim, so they should be meaningful.
func NewToolCall[T any](cm model.ToolCallingChatModel, toolName, toolDesc string) *ToolCallAgent[T] {
	return &ToolCallAgent[T]{CM: cm, ToolName: toolName, ToolDesc: toolDesc}
}

// Call binds the synthetic tool, forces a tool call, and parses the
// resulting arguments into T.
func (a *ToolCallAgent[T]) Call(ctx context.Context, messages []*schema.Message) (T, error) {
	var zero T
	toolInfo, err := utils.GoStruct2ToolInfo[T](a.ToolName, a.ToolDesc)
	if err != nil {
		return zero, fmt.Errorf("build tool schema: %w", err)
	}
	bound, err := a.CM.WithTools([]*schema.ToolInfo{toolInfo})
	if err != nil {
		return zero, fmt.Errorf("bind tool: %w", err)
	}
	resp, err := bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForced))
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

var _ Agent[any] = (*ToolCallAgent[any])(nil)

// JSONObjectAgent extracts T by requesting `response_format=json_object`
// and unmarshaling the assistant message body into T. This is the cheaper
// path for flat shapes where the cost of building a synthetic tool schema
// outweighs its benefit.
type JSONObjectAgent[T any] struct {
	CM model.ToolCallingChatModel
}

// NewJSONObject constructs a [JSONObjectAgent].
func NewJSONObject[T any](cm model.ToolCallingChatModel) *JSONObjectAgent[T] {
	return &JSONObjectAgent[T]{CM: cm}
}

// jsonObjectResponseFormat is the OpenAI-compatible extra-field payload
// that pins the assistant response to a single JSON object. We pass this
// through eino-ext's openai-specific extra-fields hook because the field
// is not part of the generic eino options surface.
var jsonObjectResponseFormat = map[string]any{
	"response_format": map[string]any{"type": "json_object"},
}

// Call generates a single assistant message under json_object mode and
// unmarshals the content into T. The strict parser is tried first; the
// lenient fallback is only invoked when the strict parser refuses the
// payload.
func (a *JSONObjectAgent[T]) Call(ctx context.Context, messages []*schema.Message) (T, error) {
	var zero T
	resp, err := a.CM.Generate(ctx, messages, openai.WithExtraFields(jsonObjectResponseFormat))
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

var _ Agent[any] = (*JSONObjectAgent[any])(nil)
