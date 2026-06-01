package beat_test

import (
	"context"
	"testing"

	"github.com/sizolity/worldline/agent/beat"
)

var _ beat.Agent = (*mockBeatAgent)(nil)

type mockBeatAgent struct{}

func (m *mockBeatAgent) Run(_ context.Context, req beat.Request) *beat.Stream {
	narrativeCh := make(chan string, 1)
	doneCh := make(chan beat.Result, 1)
	narrativeCh <- "Once upon a time..."
	close(narrativeCh)
	doneCh <- beat.Result{Narrative: "Once upon a time..."}
	return &beat.Stream{Narrative: narrativeCh, Done: doneCh}
}

func TestStreamWait(t *testing.T) {
	agent := &mockBeatAgent{}
	stream := agent.Run(context.Background(), beat.Request{
		SystemPrompt: "test",
		UserMessage:  "go",
	})
	result := stream.Wait()
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Narrative != "Once upon a time..." {
		t.Fatalf("expected narrative, got %q", result.Narrative)
	}
}
