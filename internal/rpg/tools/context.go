package tools

import (
	"math/rand/v2"
	"sync"

	"github.com/sizolity/worldline/internal/rpg/fog"
	"github.com/sizolity/worldline/internal/world/model"
)

// ToolContext holds mutable state for tool execution within a single beat.
// It is goroutine-safe for use within Eino's tool node.
type ToolContext struct {
	mu             sync.Mutex
	World          model.World
	PendingEffects []model.Effect
	Rng            *rand.Rand
	Disclosure     *fog.DisclosureState // nil = fog disabled
}

// GetPendingEffects returns a copy of accumulated effects (goroutine-safe).
func (tc *ToolContext) GetPendingEffects() []model.Effect {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	out := make([]model.Effect, len(tc.PendingEffects))
	copy(out, tc.PendingEffects)
	return out
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
