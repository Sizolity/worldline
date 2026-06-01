package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/sizolity/worldline/world/director"
	"github.com/sizolity/worldline/world/model"
)

var bg = context.Background()

func TestRuntimeStepWithNoDirectorsReturnsNonNilEmptyResult(t *testing.T) {
	t.Parallel()

	world := model.World{ID: "world_1", Name: "World"}
	got, err := NewRuntime(WithoutRules()).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if got.World.ID != "world_1" {
		t.Fatalf("world mismatch: %#v", got.World)
	}
	if got.Proposals == nil {
		t.Fatal("Proposals is nil, want non-nil empty slice")
	}
	if got.AppliedEvents == nil {
		t.Fatal("AppliedEvents is nil, want non-nil empty slice")
	}
	if len(got.Proposals) != 0 || len(got.AppliedEvents) != 0 {
		t.Fatalf("unexpected step result: %#v", got)
	}
}

func TestRuntimeStepAppliesDirectorProposals(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(director.NewScriptDirector("script_1", []model.WorldEvent{{
			ID:     "event_1",
			Type:   model.EventTypeWorldFactChanged,
			Source: model.EventSourceDirector,
			Effects: []model.Effect{{
				Kind:     model.EffectSetFact,
				TargetID: "fact_1",
				Payload: map[string]model.Value{
					"subject_id": {Kind: model.ValueKindEntityRef, Raw: "tower"},
					"predicate":  {Kind: model.ValueKindString, Raw: "status"},
					"value":      {Kind: model.ValueKindString, Raw: "sealed"},
				},
			}},
		}})),
	)
	got, err := rt.Step(bg, model.World{ID: "world_1", Name: "World"})
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.Proposals) != 1 || got.Proposals[0].ID != "event_1" {
		t.Fatalf("Proposals mismatch: %#v", got.Proposals)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_1" {
		t.Fatalf("AppliedEvents mismatch: %#v", got.AppliedEvents)
	}
	if len(got.World.Facts) != 1 || got.World.Facts[0].ID != "fact_1" {
		t.Fatalf("fact was not applied: %#v", got.World.Facts)
	}
	if len(got.World.EventLog) != 1 || got.World.EventLog[0].ID != "event_1" {
		t.Fatalf("event was not logged: %#v", got.World.EventLog)
	}
}

func TestRuntimeStepDoesNotConsumeEventQueueByDefault(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{{
			Event: model.WorldEvent{
				ID:     "event_queued",
				Type:   model.EventTypeNote,
				Source: model.EventSourceRuntime,
			},
		}},
	}

	got, err := NewRuntime(WithoutRules()).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.World.EventQueue) != 1 || got.World.EventQueue[0].Event.ID != "event_queued" {
		t.Fatalf("queue should remain untouched by default: %#v", got.World.EventQueue)
	}
	if len(got.AppliedEvents) != 0 {
		t.Fatalf("AppliedEvents = %#v, want empty", got.AppliedEvents)
	}
}

func TestRuntimeStepConsumesEventQueueWithLimit(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "event_queued_1", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
			{Event: model.WorldEvent{ID: "event_queued_2", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
		},
	}

	got, err := NewRuntime(WithoutRules(), WithEventQueueLimit(1)).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_queued_1" {
		t.Fatalf("AppliedEvents mismatch: %#v", got.AppliedEvents)
	}
	if len(got.World.EventLog) != 1 || got.World.EventLog[0].ID != "event_queued_1" {
		t.Fatalf("queued event was not logged: %#v", got.World.EventLog)
	}
	if len(got.World.EventQueue) != 1 || got.World.EventQueue[0].Event.ID != "event_queued_2" {
		t.Fatalf("queue was not consumed correctly: %#v", got.World.EventQueue)
	}
}

func TestRuntimeStepConsumesHighestPriorityReadyQueueItems(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		Clock: model.WorldClock{
			Current: model.WorldTime{Kind: model.WorldTimeTick, Tick: 5},
		},
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "event_low", Type: model.EventTypeNote, Source: model.EventSourceRuntime}, Priority: 1},
			{Event: model.WorldEvent{ID: "event_future", Type: model.EventTypeNote, Source: model.EventSourceRuntime}, Priority: 100, NotBefore: model.WorldTime{Kind: model.WorldTimeTick, Tick: 6}},
			{Event: model.WorldEvent{ID: "event_high", Type: model.EventTypeNote, Source: model.EventSourceRuntime}, Priority: 10},
		},
	}

	got, err := NewRuntime(WithoutRules(), WithEventQueueLimit(2)).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.AppliedEvents) != 2 {
		t.Fatalf("AppliedEvents count = %d, want 2: %#v", len(got.AppliedEvents), got.AppliedEvents)
	}
	if got.AppliedEvents[0].ID != "event_high" || got.AppliedEvents[1].ID != "event_low" {
		t.Fatalf("queued events consumed in wrong order: %#v", got.AppliedEvents)
	}
	if len(got.World.EventQueue) != 1 || got.World.EventQueue[0].Event.ID != "event_future" {
		t.Fatalf("future event should remain queued: %#v", got.World.EventQueue)
	}
}

func TestRuntimeStepKeepsEqualPriorityQueueOrder(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceRuntime}, Priority: 5},
			{Event: model.WorldEvent{ID: "event_2", Type: model.EventTypeNote, Source: model.EventSourceRuntime}, Priority: 5},
		},
	}

	got, err := NewRuntime(WithoutRules(), WithEventQueueLimit(2)).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.AppliedEvents) != 2 || got.AppliedEvents[0].ID != "event_1" || got.AppliedEvents[1].ID != "event_2" {
		t.Fatalf("equal priority events should keep queue order: %#v", got.AppliedEvents)
	}
}

func TestRuntimeStepAppliesReconcileDirectorProposals(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(director.NewReconcileDirector("reconcile_1", []director.ReconcileCase{{
			EventID:          "event_reconcile_1",
			TargetMemoryID:   "memory_1",
			WhenTruthStatus:  model.TruthStatusUnknown,
			TruthStatus:      model.TruthStatusDisputed,
			ConfidenceDelta:  -0.5,
			Summary:          "New evidence disputes this belief.",
			AddMemoryID:      "memory_2",
			AddMemoryContent: "I may have been wrong.",
		}})),
	)
	world := model.World{
		ID:   "world_1",
		Name: "World",
		Memory: []model.MemoryRecord{{
			ID:          "memory_1",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
			Scope:       model.MemoryScopeSubjective,
			Kind:        model.MemoryKindBelief,
			Content:     "A killed the king.",
			TruthStatus: model.TruthStatusUnknown,
			Confidence:  0.8,
		}},
	}

	got, err := rt.Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.Proposals) != 1 || got.Proposals[0].ID != "event_reconcile_1" {
		t.Fatalf("Proposals mismatch: %#v", got.Proposals)
	}
	if got.World.Memory[0].TruthStatus != model.TruthStatusDisputed {
		t.Fatalf("memory was not reconciled: %#v", got.World.Memory[0])
	}
	if len(got.World.Memory) != 2 || got.World.Memory[1].ID != "memory_2" {
		t.Fatalf("follow-up memory missing: %#v", got.World.Memory)
	}
}

func TestRuntimeStepReturnsDirectorErrorsWithoutMutatingWorld(t *testing.T) {
	t.Parallel()

	world := model.World{ID: "world_1", Name: "World"}
	rt := NewRuntime(WithoutRules(), WithDirectors(errorDirector{err: errors.New("boom")}))

	got, err := rt.Step(bg, world)
	if err == nil {
		t.Fatal("Step returned nil error")
	}
	if got.World.ID != "world_1" || len(got.World.EventLog) != 0 {
		t.Fatalf("world was mutated after director error: %#v", got.World)
	}
}

func TestRuntimeStepDoesNotLetDirectorsMutateWorldThroughContext(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		Entities: map[model.EntityID]model.Entity{
			"actor_1": {
				ID:   "actor_1",
				Type: "character",
				Name: "Actor",
				State: map[string]model.Value{
					"mood": {
						Kind: model.ValueKindObject,
						Raw:  map[string]any{"label": "calm"},
					},
				},
			},
		},
		Facts: []model.Fact{{
			ID:        "fact_1",
			SubjectID: "actor_1",
			Predicate: "status",
			Value:     model.Value{Kind: model.ValueKindObject, Raw: map[string]any{"label": "safe"}},
		}},
	}
	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(mutatingDirector{}),
	)

	got, err := rt.Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if got.World.Entities["actor_1"].State["mood"].Raw.(map[string]any)["label"] != "calm" {
		t.Fatalf("director mutated result world entity state: %#v", got.World.Entities["actor_1"].State)
	}
	if got.World.Facts[0].Value.Raw.(map[string]any)["label"] != "safe" {
		t.Fatalf("director mutated result world facts: %#v", got.World.Facts)
	}
	if world.Entities["actor_1"].State["mood"].Raw.(map[string]any)["label"] != "calm" {
		t.Fatalf("director mutated input world entity state: %#v", world.Entities["actor_1"].State)
	}
	if world.Facts[0].Value.Raw.(map[string]any)["label"] != "safe" {
		t.Fatalf("director mutated input world facts: %#v", world.Facts)
	}
}

func TestRuntimeStepReturnsApplyErrorsWithPriorAppliedEvents(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(director.NewScriptDirector("script_1", []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
			{ID: "event_2"},
		})),
	)

	got, err := rt.Step(bg, model.World{ID: "world_1", Name: "World"})
	if err == nil {
		t.Fatal("Step returned nil error")
	}
	if len(got.Proposals) != 2 {
		t.Fatalf("Proposals count = %d, want 2: %#v", len(got.Proposals), got.Proposals)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_1" {
		t.Fatalf("AppliedEvents mismatch: %#v", got.AppliedEvents)
	}
	if len(got.World.EventLog) != 1 || got.World.EventLog[0].ID != "event_1" {
		t.Fatalf("world should include only prior applied event: %#v", got.World.EventLog)
	}
}

func TestRuntimeStepSkipsFailedQueuedEventWithSkipPolicy(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{
			{
				Event:       model.WorldEvent{ID: "event_bad", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Effects: []model.Effect{{Kind: model.EffectReviseMemory, TargetID: "missing"}}},
				ErrorPolicy: model.QueueErrorPolicySkip,
			},
			{
				Event: model.WorldEvent{ID: "event_good", Type: model.EventTypeNote, Source: model.EventSourceRuntime},
			},
		},
	}

	got, err := NewRuntime(WithoutRules(), WithEventQueueLimit(10)).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_good" {
		t.Fatalf("AppliedEvents = %#v, want only event_good", got.AppliedEvents)
	}
	if len(got.World.EventQueue) != 0 {
		t.Fatalf("EventQueue should be empty: %#v", got.World.EventQueue)
	}
	if len(got.SkippedEvents) != 1 || got.SkippedEvents[0].ID != "event_bad" {
		t.Fatalf("SkippedEvents = %#v, want event_bad", got.SkippedEvents)
	}
}

func TestRuntimeStepRetriesFailedQueuedEventWithRetryPolicy(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{
			{
				Event:       model.WorldEvent{ID: "event_retry", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Effects: []model.Effect{{Kind: model.EffectReviseMemory, TargetID: "missing"}}},
				ErrorPolicy: model.QueueErrorPolicyRetry,
				MaxAttempts: 3,
			},
		},
	}

	got, err := NewRuntime(WithoutRules(), WithEventQueueLimit(10)).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.AppliedEvents) != 0 {
		t.Fatalf("AppliedEvents should be empty: %#v", got.AppliedEvents)
	}
	if len(got.World.EventQueue) != 1 {
		t.Fatalf("EventQueue should still contain retry item: %#v", got.World.EventQueue)
	}
	if got.World.EventQueue[0].Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1", got.World.EventQueue[0].Attempts)
	}
}

func TestRuntimeStepDropsRetryEventAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{
			{
				Event:       model.WorldEvent{ID: "event_exhausted", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Effects: []model.Effect{{Kind: model.EffectReviseMemory, TargetID: "missing"}}},
				ErrorPolicy: model.QueueErrorPolicyRetry,
				MaxAttempts: 2,
				Attempts:    1,
			},
		},
	}

	got, err := NewRuntime(WithoutRules(), WithEventQueueLimit(10)).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.World.EventQueue) != 0 {
		t.Fatalf("exhausted retry event should be dropped: %#v", got.World.EventQueue)
	}
	if len(got.SkippedEvents) != 1 || got.SkippedEvents[0].ID != "event_exhausted" {
		t.Fatalf("SkippedEvents = %#v, want event_exhausted", got.SkippedEvents)
	}
}

func TestRuntimeStepFailsPolicyStillFailsStep(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{
			{
				Event:       model.WorldEvent{ID: "event_fail", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Effects: []model.Effect{{Kind: model.EffectReviseMemory, TargetID: "missing"}}},
				ErrorPolicy: model.QueueErrorPolicyFail,
			},
		},
	}

	_, err := NewRuntime(WithoutRules(), WithEventQueueLimit(10)).Step(bg, world)
	if err == nil {
		t.Fatal("Step should return error for fail policy")
	}
}

func TestRuntimeStepAdvancesClockSequence(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:    "world_1",
		Name:  "World",
		Clock: model.WorldClock{Sequence: 10},
	}
	got, err := NewRuntime(WithoutRules()).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if got.World.Clock.Sequence != 11 {
		t.Fatalf("Clock.Sequence = %d, want 11", got.World.Clock.Sequence)
	}
	if world.Clock.Sequence != 10 {
		t.Fatalf("input world clock was mutated: %d", world.Clock.Sequence)
	}
}

func TestRuntimeStepAdvancesTickClock(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		Clock: model.WorldClock{
			Current:  model.WorldTime{Kind: model.WorldTimeTick, Tick: 5},
			Sequence: 0,
		},
	}
	got, err := NewRuntime(WithoutRules()).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if got.World.Clock.Current.Tick != 6 {
		t.Fatalf("Clock.Current.Tick = %d, want 6", got.World.Clock.Current.Tick)
	}
	if got.World.Clock.Sequence != 1 {
		t.Fatalf("Clock.Sequence = %d, want 1", got.World.Clock.Sequence)
	}
}

func TestRuntimeStepDoesNotAdvanceTickForNonTickClock(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		Clock: model.WorldClock{
			Current:  model.WorldTime{Kind: model.WorldTimeScene, Tick: 3},
			Sequence: 0,
		},
	}
	got, err := NewRuntime(WithoutRules()).Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if got.World.Clock.Current.Tick != 3 {
		t.Fatalf("Clock.Current.Tick = %d, want 3 (unchanged)", got.World.Clock.Current.Tick)
	}
	if got.World.Clock.Sequence != 1 {
		t.Fatalf("Clock.Sequence = %d, want 1", got.World.Clock.Sequence)
	}
}

type errorDirector struct {
	err error
}

func (d errorDirector) ID() string {
	return "error_director"
}

func (d errorDirector) Propose(director.Context) ([]model.WorldEvent, error) {
	return nil, d.err
}

type mutatingDirector struct{}

func (d mutatingDirector) ID() string {
	return "mutating_director"
}

func (d mutatingDirector) Propose(ctx director.Context) ([]model.WorldEvent, error) {
	ctx.World.Entities["actor_1"].State["mood"].Raw.(map[string]any)["label"] = "angry"
	ctx.World.Facts[0].Value.Raw.(map[string]any)["label"] = "corrupted"
	return []model.WorldEvent{}, nil
}
