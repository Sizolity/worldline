package view

import (
	"strings"
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func healthyWorld() model.World {
	return model.World{
		ID: "w1", Name: "Test World",
		Canon: model.Canon{Premise: "A fantasy realm", Genre: []string{"fantasy"}},
		Entities: map[model.EntityID]model.Entity{
			"hero": {
				ID: "hero", Type: "character", Name: "Hero",
				Components: map[string]any{
					"actor": map[string]any{"can_act": true, "goals": []any{"save the world"}},
				},
			},
			"town": {ID: "town", Type: "location", Name: "Town"},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Main Quest", Status: model.ThreadStatusActive},
		},
	}
}

func TestPreflightHealthyWorld(t *testing.T) {
	t.Parallel()
	r := Preflight(healthyWorld(), 0)
	if !r.Pass {
		t.Errorf("healthy world should pass preflight")
	}
	for _, c := range r.Readiness {
		if c.Status == ReadinessFail {
			t.Errorf("unexpected fail: %s — %s", c.Name, c.Message)
		}
	}
}

func TestPreflightNoEntities(t *testing.T) {
	t.Parallel()
	w := model.World{ID: "w1", Name: "Empty"}
	r := Preflight(w, 0)
	if r.Pass {
		t.Error("should fail with no entities")
	}
	found := false
	for _, c := range r.Readiness {
		if c.Name == "entities" && c.Status == ReadinessFail {
			found = true
		}
	}
	if !found {
		t.Error("expected entities fail check")
	}
}

func TestPreflightNoActors(t *testing.T) {
	t.Parallel()
	w := model.World{
		ID: "w1", Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "item", Name: "Sword"},
		},
	}
	r := Preflight(w, 0)
	found := false
	for _, c := range r.Readiness {
		if c.Name == "actors" && c.Status == ReadinessWarn {
			found = true
		}
	}
	if !found {
		t.Error("expected actors warning")
	}
}

func TestPreflightNoThreads(t *testing.T) {
	t.Parallel()
	w := healthyWorld()
	w.Threads = nil
	r := Preflight(w, 0)
	found := false
	for _, c := range r.Readiness {
		if c.Name == "threads" && c.Status == ReadinessWarn {
			found = true
		}
	}
	if !found {
		t.Error("expected threads warning")
	}
}

func TestPreflightNoCanon(t *testing.T) {
	t.Parallel()
	w := healthyWorld()
	w.Canon = model.Canon{}
	r := Preflight(w, 0)
	found := false
	for _, c := range r.Readiness {
		if c.Name == "canon" && c.Status == ReadinessWarn {
			found = true
		}
	}
	if !found {
		t.Error("expected canon warning")
	}
}

func TestPreflightBudgetExceeded(t *testing.T) {
	t.Parallel()
	w := healthyWorld()
	r := Preflight(w, 1)
	if r.Pass {
		t.Error("should fail when budget exceeds limit of 1 token")
	}
	found := false
	for _, c := range r.Readiness {
		if c.Name == "budget" && c.Status == ReadinessFail {
			found = true
		}
	}
	if !found {
		t.Error("expected budget fail check")
	}
}

func TestPreflightValidationErrors(t *testing.T) {
	t.Parallel()
	w := healthyWorld()
	w.Entities["bad"] = model.Entity{ID: "bad", Type: "", Name: "Bad"}
	r := Preflight(w, 0)
	if r.Pass {
		t.Error("should fail with validation errors")
	}
	found := false
	for _, c := range r.Readiness {
		if c.Name == "validation" && c.Status == ReadinessFail {
			found = true
		}
	}
	if !found {
		t.Error("expected validation fail check")
	}
}

func TestFormatPreflightPass(t *testing.T) {
	t.Parallel()
	r := Preflight(healthyWorld(), 0)
	out := FormatPreflight(r)
	if !strings.Contains(out, "PASS") {
		t.Errorf("expected PASS:\n%s", out)
	}
	if !strings.Contains(out, "Preflight:") {
		t.Errorf("missing header:\n%s", out)
	}
}

func TestFormatPreflightFail(t *testing.T) {
	t.Parallel()
	w := model.World{ID: "w1", Name: "Empty"}
	r := Preflight(w, 0)
	out := FormatPreflight(r)
	if !strings.Contains(out, "FAIL") {
		t.Errorf("expected FAIL:\n%s", out)
	}
}
