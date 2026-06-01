package director

import (
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func TestScriptDirectorProposesScriptedEvents(t *testing.T) {
	t.Parallel()

	d := NewScriptDirector("script_1", []model.WorldEvent{
		{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		{ID: "event_2", Type: model.EventTypeRemember, Source: model.EventSourceDirector},
	})

	got, err := d.Propose(Context{World: model.World{ID: "world_1", Name: "World"}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if d.ID() != "script_1" {
		t.Fatalf("ID = %q, want script_1", d.ID())
	}
	if len(got) != 2 {
		t.Fatalf("events count = %d, want 2: %#v", len(got), got)
	}
	if got[0].ID != "event_1" || got[1].ID != "event_2" {
		t.Fatalf("events = %#v, want event_1 then event_2", got)
	}
}

func TestScriptDirectorReturnsNonNilEmptySlice(t *testing.T) {
	t.Parallel()

	got, err := NewScriptDirector("script_1", nil).Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if got == nil {
		t.Fatal("events is nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0: %#v", len(got), got)
	}
}

func TestScriptDirectorDoesNotValidateProposals(t *testing.T) {
	t.Parallel()

	got, err := NewScriptDirector("script_1", []model.WorldEvent{
		{ID: "event_1"},
	}).Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "event_1" {
		t.Fatalf("events = %#v, want invalid proposal returned", got)
	}
}

func TestScriptDirectorDoesNotAliasScriptedEvents(t *testing.T) {
	t.Parallel()

	events := []model.WorldEvent{{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceDirector,
		ActorIDs: []model.EntityID{"char_a"},
		Effects: []model.Effect{{
			Kind:     model.EffectUpdateEntityState,
			TargetID: "char_a",
			Payload: map[string]model.Value{
				"mood": {Kind: model.ValueKindString, Raw: "calm"},
			},
		}},
	}}
	d := NewScriptDirector("script_1", events)

	got, err := d.Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	got[0].ActorIDs[0] = "char_b"
	got[0].Effects[0].Payload["mood"] = model.Value{Kind: model.ValueKindString, Raw: "angry"}

	second, err := d.Propose(Context{})
	if err != nil {
		t.Fatalf("second Propose returned error: %v", err)
	}
	if second[0].ActorIDs[0] != "char_a" {
		t.Fatalf("scripted event actors were mutated: %#v", second[0].ActorIDs)
	}
	if second[0].Effects[0].Payload["mood"].Raw != "calm" {
		t.Fatalf("scripted event payload was mutated: %#v", second[0].Effects[0].Payload)
	}
	if events[0].ActorIDs[0] != "char_a" {
		t.Fatalf("input event actors were mutated: %#v", events[0].ActorIDs)
	}
	if events[0].Effects[0].Payload["mood"].Raw != "calm" {
		t.Fatalf("input event payload was mutated: %#v", events[0].Effects[0].Payload)
	}
}

func TestScriptDirectorDoesNotAliasObjectValuePayloads(t *testing.T) {
	t.Parallel()

	events := []model.WorldEvent{{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceDirector,
		Effects: []model.Effect{{
			Kind:     model.EffectUpdateEntityState,
			TargetID: "char_a",
			Payload: map[string]model.Value{
				"state": {
					Kind: model.ValueKindObject,
					Raw: map[string]any{
						"label": "calm",
						"nested": map[string]any{
							"score": float64(1),
						},
					},
				},
			},
		}},
	}}
	d := NewScriptDirector("script_1", events)

	got, err := d.Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	raw := got[0].Effects[0].Payload["state"].Raw.(map[string]any)
	raw["label"] = "angry"
	raw["nested"].(map[string]any)["score"] = float64(99)

	second, err := d.Propose(Context{})
	if err != nil {
		t.Fatalf("second Propose returned error: %v", err)
	}
	secondRaw := second[0].Effects[0].Payload["state"].Raw.(map[string]any)
	if secondRaw["label"] != "calm" {
		t.Fatalf("scripted object payload was mutated: %#v", secondRaw)
	}
	if secondRaw["nested"].(map[string]any)["score"] != float64(1) {
		t.Fatalf("scripted nested object payload was mutated: %#v", secondRaw)
	}
	inputRaw := events[0].Effects[0].Payload["state"].Raw.(map[string]any)
	if inputRaw["label"] != "calm" {
		t.Fatalf("input object payload was mutated: %#v", inputRaw)
	}
	if inputRaw["nested"].(map[string]any)["score"] != float64(1) {
		t.Fatalf("input nested object payload was mutated: %#v", inputRaw)
	}
}
