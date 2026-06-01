package director_test

import (
	"testing"

	"github.com/sizolity/worldline/agent/director"
	worlddirector "github.com/sizolity/worldline/world/director"
)

// Compile-time interface check.
var _ worlddirector.Director = (*director.LLMDirector)(nil)

func TestNew(t *testing.T) {
	d := director.New("test-director", nil, director.Config{})
	if d.ID() != "test-director" {
		t.Fatalf("expected ID 'test-director', got %q", d.ID())
	}
}
