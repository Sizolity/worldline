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
