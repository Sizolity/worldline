package view

import (
	"strings"
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func TestFormatHistoryEmpty(t *testing.T) {
	t.Parallel()
	out := FormatHistory(nil, nil)
	if out != "no events\n" {
		t.Errorf("got: %q", out)
	}
}

func TestFormatHistoryBasicEvent(t *testing.T) {
	t.Parallel()
	events := []model.WorldEvent{
		{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "The sky darkened."},
	}
	out := FormatHistory(events, nil)
	if !strings.Contains(out, "[note]") {
		t.Errorf("missing type:\n%s", out)
	}
	if !strings.Contains(out, "director") {
		t.Errorf("missing source:\n%s", out)
	}
	if !strings.Contains(out, "The sky darkened.") {
		t.Errorf("missing description:\n%s", out)
	}
}

func TestFormatHistoryResolvesNames(t *testing.T) {
	t.Parallel()
	events := []model.WorldEvent{
		{
			ID: "ev1", Type: model.EventTypeMove, Source: model.EventSourceRuntime,
			ActorIDs: []model.EntityID{"char_a"}, TargetIDs: []model.EntityID{"loc_1"},
			LocationID: "loc_1",
		},
	}
	names := map[model.EntityID]string{
		"char_a": "Alice",
		"loc_1":  "Market",
	}
	out := FormatHistory(events, names)
	if !strings.Contains(out, "by Alice") {
		t.Errorf("missing actor name:\n%s", out)
	}
	if !strings.Contains(out, "→ Market") {
		t.Errorf("missing target name:\n%s", out)
	}
	if !strings.Contains(out, "@ Market") {
		t.Errorf("missing location name:\n%s", out)
	}
}

func TestFormatHistoryFallsBackToID(t *testing.T) {
	t.Parallel()
	events := []model.WorldEvent{
		{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector, ActorIDs: []model.EntityID{"unknown_id"}},
	}
	out := FormatHistory(events, map[model.EntityID]string{})
	if !strings.Contains(out, "by unknown_id") {
		t.Errorf("expected fallback to ID:\n%s", out)
	}
}

func TestFormatHistoryUsesIntentFallback(t *testing.T) {
	t.Parallel()
	events := []model.WorldEvent{
		{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceUser, Intent: "explore the cave"},
	}
	out := FormatHistory(events, nil)
	if !strings.Contains(out, "explore the cave") {
		t.Errorf("missing intent fallback:\n%s", out)
	}
}

func TestFormatHistoryShowsEffects(t *testing.T) {
	t.Parallel()
	events := []model.WorldEvent{
		{
			ID: "ev1", Type: model.EventTypeWorldFactChanged, Source: model.EventSourceDirector,
			Effects: []model.Effect{
				{Kind: model.EffectSetFact, TargetID: "f1"},
				{Kind: model.EffectAddMemory, TargetID: "m1"},
			},
		},
	}
	out := FormatHistory(events, nil)
	if !strings.Contains(out, "2 effect(s):") {
		t.Errorf("missing effect count:\n%s", out)
	}
	if !strings.Contains(out, "set_fact") {
		t.Errorf("missing effect kind:\n%s", out)
	}
}

func TestFormatHistoryMultipleEvents(t *testing.T) {
	t.Parallel()
	events := []model.WorldEvent{
		{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "First."},
		{ID: "ev2", Type: model.EventTypeMove, Source: model.EventSourceRuntime, Description: "Second."},
		{ID: "ev3", Type: model.EventTypeNote, Source: model.EventSourceUser, Description: "Third."},
	}
	out := FormatHistory(events, nil)
	if !strings.Contains(out, "  1.") {
		t.Errorf("missing event 1:\n%s", out)
	}
	if !strings.Contains(out, "  3.") {
		t.Errorf("missing event 3:\n%s", out)
	}
}

func TestBuildHistoryStructured(t *testing.T) {
	t.Parallel()
	events := []model.WorldEvent{
		{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "Something happened.",
			Effects: []model.Effect{{Kind: model.EffectSetFact, TargetID: "f1"}}},
	}
	entries := BuildHistory(events)
	if len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	if entries[0].Index != 1 {
		t.Errorf("index = %d", entries[0].Index)
	}
	if entries[0].Summary != "Something happened." {
		t.Errorf("summary = %q", entries[0].Summary)
	}
	if entries[0].EffectCount != 1 {
		t.Errorf("effect_count = %d", entries[0].EffectCount)
	}
}
