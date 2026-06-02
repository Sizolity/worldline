package tools

import "testing"

func TestRegistryReturnsAllTools(t *testing.T) {
	infos := Registry()
	const wantCount = 8
	if len(infos) != wantCount {
		t.Fatalf("expected %d tools, got %d", wantCount, len(infos))
	}
	names := map[string]bool{}
	for _, info := range infos {
		names[info.Name] = true
	}
	for _, required := range []string{
		"lookup_rules", "update_state", "advance_time", "get_entity_state",
		"explore_knowledge", "random", "chance", "weighted_choice",
	} {
		if !names[required] {
			t.Errorf("missing tool %q", required)
		}
	}
	// roll is intentionally shelved in v1 — it must NOT be registered.
	if names["roll"] {
		t.Errorf("roll should be shelved (not registered) in v1")
	}
}

func TestRegistryToolsHaveDescriptions(t *testing.T) {
	infos := Registry()
	for _, info := range infos {
		if info.Desc == "" {
			t.Errorf("tool %q has no description", info.Name)
		}
	}
}
