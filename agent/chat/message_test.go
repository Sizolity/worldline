package chat_test

import (
	"testing"

	"github.com/sizolity/worldline/agent/chat"
)

func TestSystemMessage(t *testing.T) {
	m := chat.SystemMessage("hello")
	if m.Role != chat.RoleSystem {
		t.Fatalf("expected RoleSystem, got %q", m.Role)
	}
	if m.Content != "hello" {
		t.Fatalf("expected content 'hello', got %q", m.Content)
	}
}

func TestUserMessage(t *testing.T) {
	m := chat.UserMessage("world")
	if m.Role != chat.RoleUser {
		t.Fatalf("expected RoleUser, got %q", m.Role)
	}
	if m.Content != "world" {
		t.Fatalf("expected content 'world', got %q", m.Content)
	}
}

func TestAssistantMessage(t *testing.T) {
	m := chat.AssistantMessage("reply")
	if m.Role != chat.RoleAssistant {
		t.Fatalf("expected RoleAssistant, got %q", m.Role)
	}
	if m.Content != "reply" {
		t.Fatalf("expected content 'reply', got %q", m.Content)
	}
}

func TestOptions(t *testing.T) {
	opts := chat.ApplyOptions(
		chat.WithExtra("temperature", 0.7),
		chat.WithExtra("model", "gpt-4"),
	)
	if opts.Extra["temperature"] != 0.7 {
		t.Fatalf("expected temperature 0.7, got %v", opts.Extra["temperature"])
	}
	if opts.Extra["model"] != "gpt-4" {
		t.Fatalf("expected model 'gpt-4', got %v", opts.Extra["model"])
	}
}
