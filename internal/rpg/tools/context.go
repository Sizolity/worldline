package tools

import (
	"fmt"
	"math/rand/v2"
	"os"
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
	// BeatID, when set, is folded into debug-log lines emitted by the
	// internal randomness tools. Optional; tools work without it.
	BeatID string
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

// logDice emits a single-line stderr trace for each internal-randomness
// call, gated by the WORLDLINE_DEBUG_DICE environment variable per
// directive 2.3. Output is **stderr only** — the player-facing stdout
// stream never sees these so the "zero visible judgment" red line
// stays intact.
func logDice(beatID, tool, detail string) {
	if os.Getenv("WORLDLINE_DEBUG_DICE") == "" {
		return
	}
	if beatID != "" {
		fmt.Fprintf(os.Stderr, "dice[%s]: %s(%s)\n", beatID, tool, detail)
		return
	}
	fmt.Fprintf(os.Stderr, "dice: %s(%s)\n", tool, detail)
}
