package runtime

import (
	"context"
	"testing"

	"github.com/sizolity/worldline/internal/world/director"
	"github.com/sizolity/worldline/internal/world/model"
)

var runBg = context.Background()

func TestRuntimeRunExecutesMultipleSteps(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(counterDirector{prefix: "evt"}),
	)
	world := model.World{
		ID:    "world_1",
		Name:  "World",
		Clock: model.WorldClock{Sequence: 0},
	}

	got, err := rt.Run(runBg, world, 3)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got.StepsCompleted != 3 {
		t.Fatalf("StepsCompleted = %d, want 3", got.StepsCompleted)
	}
	if got.World.Clock.Sequence != 3 {
		t.Fatalf("Clock.Sequence = %d, want 3", got.World.Clock.Sequence)
	}
	if len(got.World.EventLog) != 3 {
		t.Fatalf("EventLog length = %d, want 3", len(got.World.EventLog))
	}
}

func TestRuntimeRunAccumulatesAppliedEvents(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(counterDirector{prefix: "evt"}),
	)
	world := model.World{
		ID:    "world_1",
		Name:  "World",
		Clock: model.WorldClock{Sequence: 0},
	}

	got, err := rt.Run(runBg, world, 2)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(got.AllAppliedEvents) != 2 {
		t.Fatalf("AllAppliedEvents length = %d, want 2", len(got.AllAppliedEvents))
	}
}

func TestRuntimeRunStopsOnError(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(director.NewScriptDirector("script_1", []model.WorldEvent{
			{ID: "event_1"},
		})),
	)
	world := model.World{ID: "world_1", Name: "World"}

	got, err := rt.Run(runBg, world, 5)
	if err == nil {
		t.Fatal("Run returned nil error for invalid event")
	}
	if got.StepsCompleted != 0 {
		t.Fatalf("StepsCompleted = %d, want 0 (failed on first step)", got.StepsCompleted)
	}
}

func TestRuntimeRunWithZeroStepsReturnsUnchangedWorld(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:    "world_1",
		Name:  "World",
		Clock: model.WorldClock{Sequence: 5},
	}
	got, err := NewRuntime(WithoutRules()).Run(runBg, world, 0)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got.StepsCompleted != 0 {
		t.Fatalf("StepsCompleted = %d, want 0", got.StepsCompleted)
	}
	if got.World.Clock.Sequence != 5 {
		t.Fatalf("Clock.Sequence = %d, want 5 (unchanged)", got.World.Clock.Sequence)
	}
}

func TestRuntimeRunDoesNotMutateInputWorld(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(counterDirector{prefix: "evt"}),
	)
	world := model.World{
		ID:    "world_1",
		Name:  "World",
		Clock: model.WorldClock{Sequence: 0},
	}

	_, err := rt.Run(runBg, world, 3)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if world.Clock.Sequence != 0 {
		t.Fatalf("input world clock was mutated: %d", world.Clock.Sequence)
	}
	if len(world.EventLog) != 0 {
		t.Fatalf("input world event log was mutated: %d", len(world.EventLog))
	}
}

func TestRuntimeRunRotatesEventTableDirectorAcrossSteps(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(director.NewEventTableDirector("table_1", []director.EventTableEntry{
			{Weight: 1, Event: model.WorldEvent{ID: "event_a", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
			{Weight: 1, Event: model.WorldEvent{ID: "event_b", Type: model.EventTypeNote, Source: model.EventSourceDirector}},
		})),
	)
	world := model.World{
		ID:    "world_1",
		Name:  "World",
		Clock: model.WorldClock{Sequence: 0},
	}

	got, err := rt.Run(runBg, world, 2)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(got.AllAppliedEvents) != 2 {
		t.Fatalf("AllAppliedEvents = %d, want 2", len(got.AllAppliedEvents))
	}
	if got.AllAppliedEvents[0].ID == got.AllAppliedEvents[1].ID {
		t.Fatalf("EventTableDirector selected same event both steps: %q — clock rotation is broken",
			got.AllAppliedEvents[0].ID)
	}
}

// counterDirector proposes a single note event each step, using the world
// clock sequence to generate unique IDs.
type counterDirector struct {
	prefix string
}

func (d counterDirector) ID() string { return "counter_director" }

func (d counterDirector) Propose(ctx director.Context) ([]model.WorldEvent, error) {
	id := model.EventID(d.prefix + "_" + itoa(ctx.World.Clock.Sequence))
	return []model.WorldEvent{{
		ID:     id,
		Type:   model.EventTypeNote,
		Source: model.EventSourceDirector,
	}}, nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
