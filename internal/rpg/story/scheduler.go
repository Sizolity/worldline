package story

import (
	"fmt"
	"math/rand/v2"

	"github.com/sizolity/worldline/internal/world/model"
)

// TimeDelta is how much in-fiction time elapsed this beat, per scale.
// Scenes is the per-beat baseline (normally 1); Days/Chapters come from
// narrative-declared time skips (the advance_time tool).
type TimeDelta struct {
	Scenes   int
	Days     int
	Chapters int
}

// TickInput is the per-beat input to the scheduler.
type TickInput struct {
	World model.World
	Lines []WorldLine
	Delta TimeDelta
}

// TickOutput is what the scheduler returns. Events should be applied via
// worldruntime.ApplyEvent in order; UpdatedLines should be persisted in place
// of the input lines. World is NOT mutated by the scheduler — applying the
// returned events is the session's responsibility.
type TickOutput struct {
	Events       []model.WorldEvent
	UpdatedLines []WorldLine
}

// Tick advances every non-resolved WorldLine by one beat:
//
//  1. Apply drift to thread tension (clamped to [0, 1]); emit
//     EventTypeThreadChanged with EffectUpdateThread if tension changes.
//  2. Evaluate each not-yet-triggered Milestone's Condition; on first true,
//     emit EventTypeNote carrying the milestone's Effects and mark Triggered.
//
// The function is pure-ish: it does not mutate the input world, and given the
// same input world + lines + rng it returns identical output. It MUST NOT
// call an LLM (spec §2.4 LLM Boundary).
//
// `rng` is reserved for future probabilistic conditions; the MVP doesn't use
// it yet but accepting it now keeps the API stable.
func Tick(in TickInput, _ *rand.Rand) (TickOutput, error) {
	threadByID := indexThreads(in.World.Threads)
	out := TickOutput{
		UpdatedLines: make([]WorldLine, 0, len(in.Lines)),
	}

	// We track per-thread tension as the scheduler progresses so multiple
	// lines pointing at the same thread compose correctly (rare but possible).
	tensionByThread := make(map[model.ThreadID]float64, len(threadByID))
	for id, th := range threadByID {
		tensionByThread[id] = th.Tension
	}

	for _, line := range in.Lines {
		updated, events, err := tickOne(line, in.World, in.Delta, tensionByThread)
		if err != nil {
			return TickOutput{}, fmt.Errorf("worldline %q: %w", line.ID, err)
		}
		out.UpdatedLines = append(out.UpdatedLines, updated)
		out.Events = append(out.Events, events...)
	}
	return out, nil
}

func tickOne(
	line WorldLine,
	world model.World,
	timeDelta TimeDelta,
	tensionByThread map[model.ThreadID]float64,
) (WorldLine, []model.WorldEvent, error) {
	var events []model.WorldEvent

	// Resolved lines neither drift nor fire milestones.
	if line.Visibility == VisibilityResolved {
		return line, nil, nil
	}

	// 1. Drift advancement.
	delta := driftFor(line.Drift, timeDelta)
	if delta != 0 {
		current, ok := tensionByThread[line.ThreadID]
		if ok {
			newTension := clamp01(current + delta)
			if newTension != current {
				tensionByThread[line.ThreadID] = newTension
				events = append(events, model.WorldEvent{
					ID:          model.EventID(fmt.Sprintf("wl_%s_drift_%d", line.ID, world.Clock.Sequence)),
					Type:        model.EventTypeThreadChanged,
					Source:      model.EventSourceRuntime,
					Description: fmt.Sprintf("WorldLine %s drifts (Δtension=%+.3f)", line.ID, newTension-current),
					Effects: []model.Effect{{
						Kind:     model.EffectUpdateThread,
						TargetID: string(line.ThreadID),
						Payload: map[string]model.Value{
							"tension": {Kind: model.ValueKindNumber, Raw: newTension},
						},
					}},
				})
			}
		}
	}

	// 2. Milestone evaluation. We evaluate against a synthetic projected world
	// where the thread tension reflects the just-applied drift, so a milestone
	// gated on the same line's tension can fire in the same tick.
	projected := projectWorld(world, tensionByThread)

	milestones := make([]Milestone, len(line.Milestones))
	copy(milestones, line.Milestones)
	for i := range milestones {
		m := &milestones[i]
		if m.Triggered {
			continue
		}
		ok, err := evalCondition(m.Condition, projected)
		if err != nil {
			return line, nil, fmt.Errorf("milestone %q: %w", m.ID, err)
		}
		if !ok {
			continue
		}
		m.Triggered = true
		if len(m.Effects) > 0 {
			effectsCopy := make([]model.Effect, len(m.Effects))
			copy(effectsCopy, m.Effects)
			events = append(events, model.WorldEvent{
				ID:          model.EventID(fmt.Sprintf("wl_%s_milestone_%s_%d", line.ID, m.ID, world.Clock.Sequence)),
				Type:        model.EventTypeNote,
				Source:      model.EventSourceRuntime,
				Description: fmt.Sprintf("WorldLine %s milestone %s fired", line.ID, m.ID),
				Effects:     effectsCopy,
			})
		}
	}

	line.Milestones = milestones
	return line, events, nil
}

// driftFor returns the tension delta accrued this beat by summing each
// drift scale weighted by how much in-fiction time elapsed at that scale.
// All three scales are live simultaneously: a beat that advances both a
// scene and several days accrues scene drift + day drift together. Given a
// deterministic TimeDelta the result is deterministic.
func driftFor(d Drift, delta TimeDelta) float64 {
	return d.Scene*float64(delta.Scenes) +
		d.Day*float64(delta.Days) +
		d.Chapter*float64(delta.Chapters)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func indexThreads(threads []model.WorldThread) map[model.ThreadID]model.WorldThread {
	out := make(map[model.ThreadID]model.WorldThread, len(threads))
	for _, th := range threads {
		out[th.ID] = th
	}
	return out
}

// projectWorld returns a copy of world with thread tensions replaced from the
// given map. Only Threads are touched; entities/facts/etc. are shared.
func projectWorld(world model.World, tensionByThread map[model.ThreadID]float64) model.World {
	if len(world.Threads) == 0 {
		return world
	}
	threads := make([]model.WorldThread, len(world.Threads))
	for i, th := range world.Threads {
		threads[i] = th
		if t, ok := tensionByThread[th.ID]; ok {
			threads[i].Tension = t
		}
	}
	out := world
	out.Threads = threads
	return out
}
