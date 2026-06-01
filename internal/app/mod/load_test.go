package mod

import (
	"path/filepath"
	"strings"
	"testing"
)

// modRoot resolves the repo's mod/ folder relative to this test file
// (internal/app/mod/load_test.go) so tests do not depend on the
// caller's CWD or environment.
func modRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "..", "mod"))
	if err != nil {
		t.Fatalf("resolve mod root: %v", err)
	}
	return abs
}

func TestLoadScenario_XiyouChangan(t *testing.T) {
	sc, err := LoadScenario(modRoot(t), "xiyou-changan")
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	if sc.ID != "xiyou-changan" {
		t.Errorf("scenario ID = %q, want xiyou-changan", sc.ID)
	}
	if sc.World == nil {
		t.Fatal("World doc not loaded")
	}
	if sc.World.StartLocation != "changan_gate" {
		t.Errorf("start_location = %q, want changan_gate", sc.World.StartLocation)
	}
	if !strings.Contains(sc.World.Title, "西游") {
		t.Errorf("world title missing 西游: %q", sc.World.Title)
	}

	// Player must be present exactly once.
	if sc.PlayerIndex < 0 {
		t.Fatal("no player character")
	}
	playerCount := 0
	for _, ch := range sc.Characters {
		if ch.Role == RolePlayer {
			playerCount++
		}
	}
	if playerCount != 1 {
		t.Errorf("expected exactly 1 player, found %d", playerCount)
	}

	// xiyou-changan ships with 6 characters / 3 locations / 1 event /
	// 4 rules / 2 threads. Pin these counts so a stray file addition
	// surfaces at load-time.
	if got := len(sc.Characters); got != 6 {
		t.Errorf("characters = %d, want 6", got)
	}
	if got := len(sc.Locations); got != 3 {
		t.Errorf("locations = %d, want 3", got)
	}
	if got := len(sc.Events); got != 1 {
		t.Errorf("events = %d, want 1", got)
	}
	if got := len(sc.Rules); got != 4 {
		t.Errorf("rules = %d, want 4", got)
	}
	if got := len(sc.Threads); got != 2 {
		t.Errorf("threads = %d, want 2", got)
	}
}

func TestLoadScenario_MissingScenario(t *testing.T) {
	if _, err := LoadScenario(modRoot(t), "does-not-exist"); err == nil {
		t.Fatal("expected error for missing scenario")
	}
}

func TestLoadStyle_Default(t *testing.T) {
	st, err := LoadStyle(modRoot(t), "default")
	if err != nil {
		t.Fatalf("LoadStyle: %v", err)
	}
	if st.ID != "default" {
		t.Errorf("style ID = %q", st.ID)
	}
	if st.NarratorPersona == nil {
		t.Fatal("NarratorPersona is nil")
	}
	if st.LorekeeperPersona == nil {
		t.Fatal("LorekeeperPersona is nil")
	}
	if st.SuggesterPersona == nil {
		t.Fatal("SuggesterPersona is nil")
	}
	if st.IntentPersona == nil {
		t.Fatal("IntentPersona is nil")
	}
	if st.ProloguePrompt == "" {
		t.Error("prologue body is empty")
	}
	if st.RecapPrompt == "" {
		t.Error("recap body is empty")
	}

	// Narrator persona must include all reserved H2 placeholders.
	for _, h2 := range []string{
		ReservedWorld, ReservedRules, ReservedCharacters,
		ReservedLocations, ReservedNPCMemory, ReservedRecentEvents,
		ReservedActiveThreads, ReservedDiscovery,
	} {
		if st.NarratorPersona.SectionByTitle(h2) == nil {
			t.Errorf("narrator persona missing reserved H2 %q", h2)
		}
	}
}
