package tools

import (
	"math/rand/v2"
	"sync"

	"github.com/sizolity/worldline/internal/rpg/fog"
	"github.com/sizolity/worldline/internal/world/model"
)

// timeAdvance accumulates in-fiction time skips declared via the
// advance_time tool over the course of a single beat. It is local to the
// tools package on purpose: the session maps it onto story.TimeDelta so the
// tools package never needs to import the story package.
type timeAdvance struct {
	Scenes   int
	Days     int
	Chapters int
}

// ToolContext holds mutable state for tool execution within a single beat.
// It is goroutine-safe for use within Eino's tool node.
type ToolContext struct {
	mu             sync.Mutex
	World          model.World
	PendingEffects []model.Effect
	Rng            *rand.Rand
	Disclosure     *fog.DisclosureState // nil = fog disabled
	pendingTime    timeAdvance          // accumulated advance_time signals this beat
}

// GetPendingEffects returns a copy of accumulated effects (goroutine-safe).
func (tc *ToolContext) GetPendingEffects() []model.Effect {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	out := make([]model.Effect, len(tc.PendingEffects))
	copy(out, tc.PendingEffects)
	return out
}

// GetPendingTimeAdvance returns the accumulated in-fiction time advance
// declared via advance_time this beat, as (scenes, days, chapters). It is
// goroutine-safe. The session adds the per-beat baseline (one scene) and
// maps the result onto story.TimeDelta — these counts are the additional
// narrative-declared skips only.
func (tc *ToolContext) GetPendingTimeAdvance() (scenes, days, chapters int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.pendingTime.Scenes, tc.pendingTime.Days, tc.pendingTime.Chapters
}

// rngLocked returns the bound RNG, lazily allocating a deterministic
// PCG seed when the context did not get one. Must be called with mu
// held.
func (tc *ToolContext) rngLocked() *rand.Rand {
	if tc.Rng == nil {
		tc.Rng = rand.New(rand.NewPCG(0, 0))
	}
	return tc.Rng
}

func inferValueKind(v any) string {
	switch v.(type) {
	case float64, float32, int:
		return model.ValueKindNumber
	case bool:
		return model.ValueKindBoolean
	default:
		return model.ValueKindString
	}
}

func hasMutableEntities(w model.World) bool {
	for _, e := range w.Entities {
		if len(e.State) > 0 {
			return true
		}
		if _, ok := e.StatsComponent(); ok {
			return true
		}
	}
	return false
}
