package director

import (
	"math/rand"
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func TestRandomDirectorSelectsFromWeightedEntries(t *testing.T) {
	t.Parallel()

	d := NewRandomDirector("random_1", []EventTableEntry{
		{Weight: 1, Event: model.WorldEvent{ID: "event_rare", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
		{Weight: 3, Event: model.WorldEvent{ID: "event_common", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
	}, rand.New(rand.NewSource(42)))

	counts := map[model.EventID]int{}
	for i := 0; i < 400; i++ {
		got, err := d.Propose(Context{})
		if err != nil {
			t.Fatalf("Propose returned error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("proposal length = %d, want 1", len(got))
		}
		counts[got[0].ID]++
	}
	if counts["event_common"] < 200 {
		t.Fatalf("event_common selected %d/400 times, expected >> 200 for weight 3/4", counts["event_common"])
	}
	if counts["event_rare"] < 50 {
		t.Fatalf("event_rare selected %d/400 times, expected ~100 for weight 1/4", counts["event_rare"])
	}
}

func TestRandomDirectorWithSameSeedIsReproducible(t *testing.T) {
	t.Parallel()

	entries := []EventTableEntry{
		{Weight: 1, Event: model.WorldEvent{ID: "event_a", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
		{Weight: 1, Event: model.WorldEvent{ID: "event_b", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
		{Weight: 1, Event: model.WorldEvent{ID: "event_c", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
	}

	d1 := NewRandomDirector("random_1", entries, rand.New(rand.NewSource(7)))
	d2 := NewRandomDirector("random_1", entries, rand.New(rand.NewSource(7)))

	for i := 0; i < 20; i++ {
		got1, _ := d1.Propose(Context{})
		got2, _ := d2.Propose(Context{})
		if got1[0].ID != got2[0].ID {
			t.Fatalf("step %d: same seed produced different events: %q vs %q", i, got1[0].ID, got2[0].ID)
		}
	}
}

func TestRandomDirectorReturnsEmptyForEmptyOrZeroWeightTable(t *testing.T) {
	t.Parallel()

	got, err := NewRandomDirector("random_1", nil, nil).Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty table proposal = %#v, want empty", got)
	}

	got, err = NewRandomDirector("random_1", []EventTableEntry{{
		Weight: 0,
		Event:  model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
	}}, nil).Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("zero weight table proposal = %#v, want empty", got)
	}
}

func TestRandomDirectorDoesNotAliasConfiguredEvents(t *testing.T) {
	t.Parallel()

	entries := []EventTableEntry{{
		Weight: 1,
		Event: model.WorldEvent{
			ID:       "event_1",
			Type:     model.EventTypeNote,
			Source:   model.EventSourceDirector,
			ActorIDs: []model.EntityID{"char_alice"},
		},
	}}
	d := NewRandomDirector("random_1", entries, rand.New(rand.NewSource(0)))
	entries[0].Event.ActorIDs[0] = "char_bob"

	got, err := d.Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if got[0].ActorIDs[0] != "char_alice" {
		t.Fatalf("event actor ids were aliased: %#v", got[0].ActorIDs)
	}

	got[0].ActorIDs[0] = "char_bob"
	again, err := d.Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if again[0].ActorIDs[0] != "char_alice" {
		t.Fatalf("proposal output was aliased: %#v", again[0].ActorIDs)
	}
}

func TestRandomDirectorWithNilRandUsesGlobalRand(t *testing.T) {
	t.Parallel()

	d := NewRandomDirector("random_1", []EventTableEntry{
		{Weight: 1, Event: model.WorldEvent{ID: "event_a", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
	}, nil)

	got, err := d.Propose(Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "event_a" {
		t.Fatalf("single entry proposal mismatch: %#v", got)
	}
}
