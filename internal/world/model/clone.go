package model

import "slices"

func (v Value) Clone() Value {
	v.Raw = CloneAny(v.Raw)
	return v
}

func (e Effect) Clone() Effect {
	e.Payload = CloneValues(e.Payload)
	return e
}

func (e WorldEvent) Clone() WorldEvent {
	e.ActorIDs = slices.Clone(e.ActorIDs)
	e.TargetIDs = slices.Clone(e.TargetIDs)
	e.Effects = CloneEffects(e.Effects)
	return e
}

func (f Fact) Clone() Fact {
	f.Value = f.Value.Clone()
	return f
}

func (m MemoryRecord) Clone() MemoryRecord {
	m.SubjectIDs = slices.Clone(m.SubjectIDs)
	m.EventIDs = slices.Clone(m.EventIDs)
	return m
}

func (e Entity) Clone() Entity {
	e.Aliases = slices.Clone(e.Aliases)
	e.Components = CloneAnyMap(e.Components)
	e.State = CloneValues(e.State)
	e.Tags = slices.Clone(e.Tags)
	return e
}

func (i EventQueueItem) Clone() EventQueueItem {
	i.Event = i.Event.Clone()
	i.NotBefore.Calendar = CloneIntMap(i.NotBefore.Calendar)
	return i
}

func (c Canon) Clone() Canon {
	c.Genre = slices.Clone(c.Genre)
	c.Tone = slices.Clone(c.Tone)
	c.StyleGuide = slices.Clone(c.StyleGuide)
	c.Laws = slices.Clone(c.Laws)
	c.Boundaries = slices.Clone(c.Boundaries)
	c.Secrets = slices.Clone(c.Secrets)
	return c
}

func (c WorldClock) Clone() WorldClock {
	c.Current.Calendar = CloneIntMap(c.Current.Calendar)
	return c
}

func (m WorldMetadata) Clone() WorldMetadata {
	m.Tags = slices.Clone(m.Tags)
	return m
}

func (r Relation) Clone() Relation {
	return r
}

func (w World) Clone() World {
	w.Canon = w.Canon.Clone()
	w.Clock = w.Clock.Clone()
	w.Entities = CloneEntities(w.Entities)
	w.Relations = slices.Clone(w.Relations)
	w.Facts = CloneFacts(w.Facts)
	w.Rules = slices.Clone(w.Rules)
	w.Threads = slices.Clone(w.Threads)
	w.EventLog = CloneEvents(w.EventLog)
	w.EventQueue = CloneEventQueue(w.EventQueue)
	w.Memory = CloneMemories(w.Memory)
	w.Metadata = w.Metadata.Clone()
	return w
}

// CloneEntities deep-copies an entity map.
func CloneEntities(entities map[EntityID]Entity) map[EntityID]Entity {
	if entities == nil {
		return nil
	}
	out := make(map[EntityID]Entity, len(entities))
	for id, entity := range entities {
		out[id] = entity.Clone()
	}
	return out
}

// CloneValues deep-copies a Value map.
func CloneValues(in map[string]Value) map[string]Value {
	if in == nil {
		return nil
	}
	out := make(map[string]Value, len(in))
	for key, value := range in {
		out[key] = value.Clone()
	}
	return out
}

// CloneEffects deep-copies a slice of effects.
func CloneEffects(effects []Effect) []Effect {
	if effects == nil {
		return nil
	}
	out := slices.Clone(effects)
	for i := range out {
		out[i] = out[i].Clone()
	}
	return out
}

// CloneEvents deep-copies a slice of world events.
func CloneEvents(events []WorldEvent) []WorldEvent {
	if events == nil {
		return nil
	}
	out := slices.Clone(events)
	for i := range out {
		out[i] = out[i].Clone()
	}
	return out
}

// CloneFacts deep-copies a slice of facts.
func CloneFacts(facts []Fact) []Fact {
	if facts == nil {
		return nil
	}
	out := slices.Clone(facts)
	for i := range out {
		out[i] = out[i].Clone()
	}
	return out
}

// CloneMemories deep-copies a slice of memory records.
func CloneMemories(memories []MemoryRecord) []MemoryRecord {
	if memories == nil {
		return nil
	}
	out := slices.Clone(memories)
	for i := range out {
		out[i] = out[i].Clone()
	}
	return out
}

// CloneEventQueue deep-copies a slice of event queue items.
func CloneEventQueue(queue []EventQueueItem) []EventQueueItem {
	if queue == nil {
		return nil
	}
	out := slices.Clone(queue)
	for i := range out {
		out[i] = out[i].Clone()
	}
	return out
}

// CloneAny deep-copies an arbitrary value that may contain maps and slices.
func CloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return CloneAnyMap(typed)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	case map[string]int:
		return CloneIntMap(typed)
	case map[string]Value:
		return CloneValues(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = CloneAny(item)
		}
		return out
	case []string:
		return slices.Clone(typed)
	case []int:
		return slices.Clone(typed)
	case []float64:
		return slices.Clone(typed)
	default:
		return value
	}
}

// CloneAnyMap deep-copies a map[string]any.
func CloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = CloneAny(value)
	}
	return out
}

// CloneIntMap copies a map[string]int.
func CloneIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
