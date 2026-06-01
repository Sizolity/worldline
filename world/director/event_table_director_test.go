package director

import (
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func TestEventTableDirectorSelectsWeightedEventDeterministically(t *testing.T) {
	t.Parallel()

	d := NewEventTableDirector("table_1", []EventTableEntry{
		{
			Weight: 1,
			Event:  model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		},
		{
			Weight: 3,
			Event:  model.WorldEvent{ID: "event_2", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		},
	})

	got, err := d.Propose(Context{World: model.World{Clock: model.WorldClock{Sequence: 0}}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "event_1" {
		t.Fatalf("sequence 0 proposal mismatch: %#v", got)
	}

	got, err = d.Propose(Context{World: model.World{Clock: model.WorldClock{Sequence: 1}}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "event_2" {
		t.Fatalf("sequence 1 proposal mismatch: %#v", got)
	}

	again, err := d.Propose(Context{World: model.World{Clock: model.WorldClock{Sequence: 1}}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(again) != 1 || again[0].ID != got[0].ID {
		t.Fatalf("same sequence should choose same event: got %#v again %#v", got, again)
	}
}

func TestEventTableDirectorReturnsEmptyForEmptyOrZeroWeightTable(t *testing.T) {
	t.Parallel()

	got, err := NewEventTableDirector("table_1", nil).Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty table proposal = %#v, want empty", got)
	}

	got, err = NewEventTableDirector("table_1", []EventTableEntry{{
		Weight: 0,
		Event:  model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
	}}).Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("zero weight table proposal = %#v, want empty", got)
	}
}

func TestEventTableDirectorDoesNotAliasConfiguredEvents(t *testing.T) {
	t.Parallel()

	entries := []EventTableEntry{{
		Weight: 1,
		Event: model.WorldEvent{
			ID:       "event_1",
			Type:     model.EventTypeNote,
			Source:   model.EventSourceDirector,
			ActorIDs: []model.EntityID{"char_alice"},
			Effects: []model.Effect{{
				Kind:     model.EffectUpdateEntityState,
				TargetID: "char_alice",
				Payload: map[string]model.Value{
					"mood": {Kind: model.ValueKindObject, Raw: map[string]any{"label": "calm"}},
				},
			}},
		},
	}}
	d := NewEventTableDirector("table_1", entries)
	entries[0].Event.ActorIDs[0] = "char_bob"
	entries[0].Event.Effects[0].Payload["mood"] = model.Value{Kind: model.ValueKindString, Raw: "angry"}

	got, err := d.Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	got[0].ActorIDs[0] = "char_bob"
	got[0].Effects[0].Payload["mood"].Raw.(map[string]any)["label"] = "angry"

	again, err := d.Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if again[0].ActorIDs[0] != "char_alice" {
		t.Fatalf("event actor ids were aliased: %#v", again[0].ActorIDs)
	}
	if again[0].Effects[0].Payload["mood"].Raw.(map[string]any)["label"] != "calm" {
		t.Fatalf("event payload was aliased: %#v", again[0].Effects[0].Payload)
	}
}
