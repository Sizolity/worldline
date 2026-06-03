package react_test

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/agent/react"
)

var _ react.Agent = (*mockReactAgent)(nil)

type mockReactAgent struct{}

func (m *mockReactAgent) Run(_ context.Context, req react.Request) *react.Stream {
	narrativeCh := make(chan string, 1)
	doneCh := make(chan react.Result, 1)
	narrativeCh <- "Once upon a time..."
	close(narrativeCh)
	doneCh <- react.Result{Narrative: "Once upon a time..."}
	return &react.Stream{Narrative: narrativeCh, Done: doneCh}
}

func TestStreamWait(t *testing.T) {
	agent := &mockReactAgent{}
	stream := agent.Run(context.Background(), react.Request{
		Messages: []*schema.Message{
			schema.SystemMessage("test"),
			schema.UserMessage("go"),
		},
	})
	result := stream.Wait()
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Narrative != "Once upon a time..." {
		t.Fatalf("expected narrative, got %q", result.Narrative)
	}
}

// mockReactAgentWithToolCall verifies that the Agent interface surface
// supports the inline-schema pattern: callers can populate Result.ToolCalls
// alongside Narrative, and Stream.Wait carries them through unchanged.
// Mirrors the shape of a real beat where the model emits text + a final
// set_choices tool_call in the same completion.
type mockReactAgentWithToolCall struct{}

func (m *mockReactAgentWithToolCall) Run(_ context.Context, _ react.Request) *react.Stream {
	narrativeCh := make(chan string, 1)
	doneCh := make(chan react.Result, 1)
	narrativeCh <- "The door swings open."
	close(narrativeCh)
	doneCh <- react.Result{
		Narrative: "The door swings open.",
		ToolCalls: []schema.ToolCall{{
			ID: "call_test_1",
			Function: schema.FunctionCall{
				Name:      "set_choices",
				Arguments: `{"choices":[{"label":"step inside","type":"explore"}]}`,
			},
		}},
	}
	return &react.Stream{Narrative: narrativeCh, Done: doneCh}
}

func TestStreamWait_CarriesToolCalls(t *testing.T) {
	agent := &mockReactAgentWithToolCall{}
	stream := agent.Run(context.Background(), react.Request{
		Messages: []*schema.Message{schema.UserMessage("open the door")},
	})
	result := stream.Wait()
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Narrative != "The door swings open." {
		t.Errorf("narrative = %q", result.Narrative)
	}
	if got := len(result.ToolCalls); got != 1 {
		t.Fatalf("expected 1 tool call, got %d", got)
	}
	if got := result.ToolCalls[0].Function.Name; got != "set_choices" {
		t.Errorf("tool name = %q, want set_choices", got)
	}
}
