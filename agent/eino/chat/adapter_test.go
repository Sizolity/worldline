package einochat_test

import (
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/agent/chat"
	einochat "github.com/sizolity/worldline/agent/eino/chat"
)

func TestToEinoMessages(t *testing.T) {
	msgs := []chat.Message{
		chat.SystemMessage("sys"),
		chat.UserMessage("hello"),
		{
			Role:    chat.RoleAssistant,
			Content: "reply",
			ToolCalls: []chat.ToolCall{
				{ID: "tc1", Name: "search", Arguments: `{"q":"test"}`},
			},
		},
		{
			Role:       chat.RoleTool,
			Content:    "result",
			ToolCallID: "tc1",
		},
	}

	out := einochat.ToEinoMessages(msgs)
	if len(out) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(out))
	}
	if out[0].Role != schema.System || out[0].Content != "sys" {
		t.Errorf("message[0] mismatch: %+v", out[0])
	}
	if out[1].Role != schema.User || out[1].Content != "hello" {
		t.Errorf("message[1] mismatch: %+v", out[1])
	}
	if out[2].Role != schema.Assistant || len(out[2].ToolCalls) != 1 {
		t.Errorf("message[2] mismatch: %+v", out[2])
	}
	if out[2].ToolCalls[0].Function.Name != "search" {
		t.Errorf("tool call name mismatch: %s", out[2].ToolCalls[0].Function.Name)
	}
	if out[3].Role != schema.Tool || out[3].ToolCallID != "tc1" {
		t.Errorf("message[3] mismatch: %+v", out[3])
	}
}

func TestFromEinoMessage(t *testing.T) {
	em := &schema.Message{
		Role:    schema.Assistant,
		Content: "hello",
		ToolCalls: []schema.ToolCall{
			{ID: "t1", Type: "function", Function: schema.FunctionCall{Name: "fn", Arguments: "{}"}},
		},
	}
	msg := einochat.FromEinoMessage(em)
	if msg.Role != chat.RoleAssistant {
		t.Errorf("expected RoleAssistant, got %q", msg.Role)
	}
	if msg.Content != "hello" {
		t.Errorf("expected content 'hello', got %q", msg.Content)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Name != "fn" {
		t.Errorf("tool calls mismatch: %+v", msg.ToolCalls)
	}
}

func TestRoundTripRoles(t *testing.T) {
	roles := []chat.Role{chat.RoleSystem, chat.RoleUser, chat.RoleAssistant, chat.RoleTool}
	for _, r := range roles {
		msg := chat.Message{Role: r, Content: "x"}
		eino := einochat.ToEinoMessages([]chat.Message{msg})
		back := einochat.FromEinoMessage(eino[0])
		if back.Role != r {
			t.Errorf("role roundtrip failed: %q -> %q", r, back.Role)
		}
	}
}
