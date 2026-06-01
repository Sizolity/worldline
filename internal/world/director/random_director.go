package director

import (
	"math/rand"

	"github.com/sizolity/worldline/internal/world/model"
)

type RandomDirector struct {
	id      string
	entries []EventTableEntry
	rng     *rand.Rand
}

// NewRandomDirector creates a director that selects one event from weighted
// entries using a random number generator. If rng is nil, the global
// math/rand source is used (non-deterministic).
func NewRandomDirector(id string, entries []EventTableEntry, rng *rand.Rand) RandomDirector {
	return RandomDirector{
		id:      id,
		entries: cloneEventTableEntries(entries),
		rng:     rng,
	}
}

func (d RandomDirector) ID() string {
	return d.id
}

func (d RandomDirector) Propose(_ Context) ([]model.WorldEvent, error) {
	total := 0
	for _, entry := range d.entries {
		if entry.Weight > 0 {
			total += entry.Weight
		}
	}
	if total == 0 {
		return []model.WorldEvent{}, nil
	}
	roll := d.intn(total)
	current := 0
	for _, entry := range d.entries {
		if entry.Weight <= 0 {
			continue
		}
		current += entry.Weight
		if roll < current {
			return []model.WorldEvent{entry.Event.Clone()}, nil
		}
	}
	return []model.WorldEvent{}, nil
}

func (d RandomDirector) intn(n int) int {
	if d.rng != nil {
		return d.rng.Intn(n)
	}
	return rand.Intn(n)
}
