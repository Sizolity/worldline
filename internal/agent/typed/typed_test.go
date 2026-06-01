package typed_test

import (
	"context"

	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/internal/agent/typed"
)

// Compile-time interface check for a concrete generic instantiation.
var _ typed.Agent[testOutput] = (*mockTypedAgent)(nil)

type testOutput struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

type mockTypedAgent struct{}

func (m *mockTypedAgent) Call(_ context.Context, _ []*schema.Message) (testOutput, error) {
	return testOutput{Name: "test", Score: 42}, nil
}
