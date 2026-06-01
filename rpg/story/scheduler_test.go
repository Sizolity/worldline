package story

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func newRNG() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

// helper world with one thread + one entity + one fact
func sampleWorld(threadTension float64) model.World {
	return model.World{
		ID:   "w1",
		Name: "W",
		Clock: model.WorldClock{
			Current:  model.WorldTime{Kind: model.WorldTimeScene, Tick: 5},
			Sequence: 5,
		},
		Entities: map[model.EntityID]model.Entity{
			"e_door": {ID: "e_door", Type: "object", Name: "Door",
				State: map[string]model.Value{
					"locked": {Kind: model.ValueKindBoolean, Raw: true},
					"opens":  {Kind: model.ValueKindNumber, Raw: float64(3)},
				}},
		},
		Threads: []model.WorldThread{
			{ID: "th_seal", Kind: model.ThreadKindMystery, Title: "Seal", Status: model.ThreadStatusActive, Tension: threadTension},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "e_door", Predicate: "guarded_by",
				Value: model.Value{Kind: model.ValueKindString, Raw: "ghost"}},
		},
	}
}

func TestTick_DriftAdvancesTension(t *testing.T) {
	world := sampleWorld(0.30)
	lines := []WorldLine{
		{ID: "wl_seal", ThreadID: "th_seal", Visibility: VisibilityHinted,
			Drift: Drift{Scene: 0.10}},
	}
	out, err := Tick(TickInput{World: world, Lines: lines, TimeScale: model.WorldTimeScene}, newRNG())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(out.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(out.Events))
	}
	ev := out.Events[0]
	if ev.Type != model.EventTypeThreadChanged {
		t.Errorf("expected ThreadChanged, got %s", ev.Type)
	}
	if len(ev.Effects) != 1 || ev.Effects[0].Kind != model.EffectUpdateThread {
		t.Fatalf("expected single EffectUpdateThread, got %+v", ev.Effects)
	}
	if got := ev.Effects[0].Payload["tension"].Raw.(float64); got != 0.40 {
		t.Errorf("expected tension 0.40, got %v", got)
	}
	if ev.Effects[0].TargetID != "th_seal" {
		t.Errorf("wrong target: %s", ev.Effects[0].TargetID)
	}
}

func TestTick_DriftClampsAtOne(t *testing.T) {
	world := sampleWorld(0.95)
	lines := []WorldLine{
		{ID: "wl", ThreadID: "th_seal", Drift: Drift{Scene: 0.50}},
	}
	out, _ := Tick(TickInput{World: world, Lines: lines, TimeScale: model.WorldTimeScene}, newRNG())
	if got := out.Events[0].Effects[0].Payload["tension"].Raw.(float64); got != 1.0 {
		t.Errorf("expected tension clamped at 1.0, got %v", got)
	}
}

func TestTick_DriftClampsAtZero(t *testing.T) {
	world := sampleWorld(0.10)
	lines := []WorldLine{
		{ID: "wl", ThreadID: "th_seal", Drift: Drift{Scene: -0.50}},
	}
	out, _ := Tick(TickInput{World: world, Lines: lines, TimeScale: model.WorldTimeScene}, newRNG())
	if got := out.Events[0].Effects[0].Payload["tension"].Raw.(float64); got != 0.0 {
		t.Errorf("expected tension clamped at 0.0, got %v", got)
	}
}

func TestTick_NoDriftAtSaturation(t *testing.T) {
	// Already at 1.0; drift +0.1 → clamped to 1.0 → no change → no event.
	world := sampleWorld(1.0)
	lines := []WorldLine{
		{ID: "wl", ThreadID: "th_seal", Drift: Drift{Scene: 0.10}},
	}
	out, _ := Tick(TickInput{World: world, Lines: lines, TimeScale: model.WorldTimeScene}, newRNG())
	if len(out.Events) != 0 {
		t.Errorf("expected 0 events at saturation, got %d", len(out.Events))
	}
}

func TestTick_TimeScaleSelection(t *testing.T) {
	tests := []struct {
		scale   model.WorldTimeKind
		drift   Drift
		wantTen float64
	}{
		{model.WorldTimeScene, Drift{Scene: 0.02, Day: 0.15}, 0.32},
		{model.WorldTimeDay, Drift{Scene: 0.02, Day: 0.15}, 0.45},
		{model.WorldTimeChapter, Drift{Chapter: 0.35}, 0.65},
		{model.WorldTimeTurn, Drift{Scene: 0.99}, 0.30}, // unknown scale → no drift → no event
	}
	for _, tt := range tests {
		t.Run(string(tt.scale), func(t *testing.T) {
			world := sampleWorld(0.30)
			lines := []WorldLine{{ID: "wl", ThreadID: "th_seal", Drift: tt.drift}}
			out, _ := Tick(TickInput{World: world, Lines: lines, TimeScale: tt.scale}, newRNG())
			if tt.wantTen == 0.30 {
				if len(out.Events) != 0 {
					t.Errorf("expected no event for unknown scale, got %d", len(out.Events))
				}
				return
			}
			got := out.Events[0].Effects[0].Payload["tension"].Raw.(float64)
			if !approxEqual(got, tt.wantTen) {
				t.Errorf("scale %s: want %v, got %v", tt.scale, tt.wantTen, got)
			}
		})
	}
}

func TestTick_ResolvedLineNotAdvanced(t *testing.T) {
	world := sampleWorld(0.30)
	lines := []WorldLine{
		{ID: "wl", ThreadID: "th_seal", Visibility: VisibilityResolved, Drift: Drift{Scene: 0.50}},
	}
	out, _ := Tick(TickInput{World: world, Lines: lines, TimeScale: model.WorldTimeScene}, newRNG())
	if len(out.Events) != 0 {
		t.Errorf("resolved line should not emit events, got %d", len(out.Events))
	}
}

func TestTick_MilestoneFires_OnThreadTension(t *testing.T) {
	// Drift bumps tension 0.55 → 0.65; milestone gated at >= 0.60 should fire.
	world := sampleWorld(0.55)
	lines := []WorldLine{{
		ID: "wl", ThreadID: "th_seal",
		Drift: Drift{Scene: 0.10},
		Milestones: []Milestone{{
			ID: "m1",
			Condition: MilestoneCondition{
				Kind: CondThreadTensionGTE,
				Args: map[string]any{"thread_id": "th_seal", "threshold": 0.60},
			},
			Effects: []model.Effect{
				{Kind: model.EffectUpdateThread, TargetID: "th_seal",
					Payload: map[string]model.Value{
						"summary": {Kind: model.ValueKindString, Raw: "The tower trembles"},
					}},
			},
		}},
	}}
	out, err := Tick(TickInput{World: world, Lines: lines, TimeScale: model.WorldTimeScene}, newRNG())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(out.Events) != 2 {
		t.Fatalf("expected drift+milestone (2 events), got %d", len(out.Events))
	}
	if out.UpdatedLines[0].Milestones[0].Triggered != true {
		t.Errorf("milestone should be Triggered=true")
	}
	// milestone event is the second one
	if out.Events[1].Description != "WorldLine wl milestone m1 fired" {
		t.Errorf("unexpected event description: %s", out.Events[1].Description)
	}
}

func TestTick_MilestoneFiresOnce(t *testing.T) {
	world := sampleWorld(0.80)
	lines := []WorldLine{{
		ID: "wl", ThreadID: "th_seal",
		Milestones: []Milestone{{
			ID: "m1", Triggered: true, // already fired
			Condition: MilestoneCondition{
				Kind: CondThreadTensionGTE,
				Args: map[string]any{"thread_id": "th_seal", "threshold": 0.50},
			},
			Effects: []model.Effect{
				{Kind: model.EffectUpdateThread, TargetID: "th_seal"},
			},
		}},
	}}
	out, _ := Tick(TickInput{World: world, Lines: lines, TimeScale: model.WorldTimeScene}, newRNG())
	if len(out.Events) != 0 {
		t.Errorf("already-triggered milestone must not refire, got %d events", len(out.Events))
	}
}

func TestTick_Condition_EntityStateEq(t *testing.T) {
	world := sampleWorld(0.30)
	tests := []struct {
		name string
		args map[string]any
		want bool
	}{
		{"bool match", map[string]any{"entity_id": "e_door", "key": "locked", "value": true}, true},
		{"bool mismatch", map[string]any{"entity_id": "e_door", "key": "locked", "value": false}, false},
		{"numeric match (int vs float64)", map[string]any{"entity_id": "e_door", "key": "opens", "value": 3}, true},
		{"missing entity", map[string]any{"entity_id": "e_missing", "key": "x", "value": "y"}, false},
		{"missing key", map[string]any{"entity_id": "e_door", "key": "missing", "value": 1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evalCondition(MilestoneCondition{Kind: CondEntityStateEq, Args: tt.args}, world)
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			if got != tt.want {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}

func TestTick_Condition_FactExists(t *testing.T) {
	world := sampleWorld(0.30)
	got, err := evalCondition(MilestoneCondition{
		Kind: CondFactExists,
		Args: map[string]any{"subject_id": "e_door", "predicate": "guarded_by"},
	}, world)
	if err != nil || !got {
		t.Errorf("expected true, got %v err=%v", got, err)
	}
	got, _ = evalCondition(MilestoneCondition{
		Kind: CondFactExists,
		Args: map[string]any{"subject_id": "e_door", "predicate": "absent"},
	}, world)
	if got {
		t.Errorf("expected false for missing predicate")
	}
}

func TestTick_Condition_UnknownKind(t *testing.T) {
	_, err := evalCondition(MilestoneCondition{Kind: "bogus_kind"}, model.World{})
	if err == nil {
		t.Errorf("expected error for unknown kind")
	}
}

func TestTick_Condition_MissingArg(t *testing.T) {
	_, err := evalCondition(MilestoneCondition{
		Kind: CondThreadTensionGTE,
		Args: map[string]any{"thread_id": "x"}, // missing threshold
	}, model.World{})
	if err == nil {
		t.Errorf("expected error for missing arg")
	}
}

func TestTick_EmptyInputs(t *testing.T) {
	out, err := Tick(TickInput{
		World:     sampleWorld(0.30),
		TimeScale: model.WorldTimeScene,
	}, newRNG())
	if err != nil {
		t.Fatalf("Tick empty: %v", err)
	}
	if len(out.Events) != 0 || len(out.UpdatedLines) != 0 {
		t.Errorf("empty input should produce empty output, got %+v", out)
	}
}

func TestTick_MissingThreadIsNoOp(t *testing.T) {
	world := sampleWorld(0.30)
	lines := []WorldLine{
		{ID: "wl", ThreadID: "th_does_not_exist", Drift: Drift{Scene: 0.10}},
	}
	out, err := Tick(TickInput{World: world, Lines: lines, TimeScale: model.WorldTimeScene}, newRNG())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(out.Events) != 0 {
		t.Errorf("missing thread should produce no events, got %d", len(out.Events))
	}
}
