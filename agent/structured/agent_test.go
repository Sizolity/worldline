package structured_test

import (
	"context"

	"github.com/sizolity/worldline/agent/chat"
	"github.com/sizolity/worldline/agent/structured"
)

// Compile-time interface check for a concrete generic instantiation.
var _ structured.Agent[testOutput] = (*mockStructuredAgent)(nil)

type testOutput struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

type mockStructuredAgent struct{}

func (m *mockStructuredAgent) Call(_ context.Context, _ []chat.Message) (testOutput, error) {
	return testOutput{Name: "test", Score: 42}, nil
}
