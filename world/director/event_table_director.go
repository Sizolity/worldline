package director

import "github.com/sizolity/worldline/world/model"

type EventTableEntry struct {
	Weight int              `json:"weight,omitempty"`
	Event  model.WorldEvent `json:"event"`
}

type EventTableDirector struct {
	id      string
	entries []EventTableEntry
}

func NewEventTableDirector(id string, entries []EventTableEntry) EventTableDirector {
	return EventTableDirector{
		id:      id,
		entries: cloneEventTableEntries(entries),
	}
}

func (d EventTableDirector) ID() string {
	return d.id
}

func (d EventTableDirector) Propose(ctx Context) ([]model.WorldEvent, error) {
	total := 0
	for _, entry := range d.entries {
		if entry.Weight > 0 {
			total += entry.Weight
		}
	}
	if total == 0 {
		return []model.WorldEvent{}, nil
	}
	roll := int(ctx.World.Clock.Sequence % int64(total))
	if roll < 0 {
		roll += total
	}
	current := 0
	for _, entry := range d.entries {
		if entry.Weight <= 0 {
			continue
		}
		current += entry.Weight
		if roll < current {
			return cloneEvents([]model.WorldEvent{entry.Event}), nil
		}
	}
	return []model.WorldEvent{}, nil
}

func cloneEventTableEntries(entries []EventTableEntry) []EventTableEntry {
	out := append([]EventTableEntry(nil), entries...)
	for i := range out {
		out[i].Event = cloneEvents([]model.WorldEvent{out[i].Event})[0]
	}
	return out
}
