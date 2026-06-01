package tool_test

import (
	"context"
	"testing"

	"github.com/sizolity/worldline/agent/tool"
)

var _ tool.Tool = (*mockTool)(nil)

type mockTool struct{}

func (m *mockTool) Info() tool.Info {
	return tool.Info{Name: "test", Description: "A test tool"}
}

func (m *mockTool) Invoke(_ context.Context, arguments string) (string, error) {
	return "result:" + arguments, nil
}

func TestToolInterface(t *testing.T) {
	var tt tool.Tool = &mockTool{}
	info := tt.Info()
	if info.Name != "test" {
		t.Fatalf("expected name 'test', got %q", info.Name)
	}
	result, err := tt.Invoke(context.Background(), `{"key":"val"}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != `result:{"key":"val"}` {
		t.Fatalf("unexpected result: %q", result)
	}
}
