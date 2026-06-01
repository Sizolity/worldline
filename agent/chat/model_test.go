package chat_test

import (
	"context"

	"github.com/sizolity/worldline/agent/chat"
)

// Compile-time interface checks.
var (
	_ chat.Model       = (*mockModel)(nil)
	_ chat.StreamModel = (*mockStreamModel)(nil)
)

type mockModel struct{}

func (m *mockModel) Generate(_ context.Context, _ []chat.Message, _ ...chat.Option) (chat.Message, error) {
	return chat.AssistantMessage("ok"), nil
}

type mockStreamModel struct{ mockModel }

func (m *mockStreamModel) Stream(_ context.Context, _ []chat.Message, _ ...chat.Option) (*chat.StreamReader, error) {
	ch := make(chan chat.Message)
	close(ch)
	return &chat.StreamReader{Ch: ch}, nil
}
