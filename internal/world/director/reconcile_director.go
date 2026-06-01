package director

import "github.com/sizolity/worldline/internal/world/model"

type ReconcileCase struct {
	EventID          model.EventID  `json:"event_id"`
	TargetMemoryID   model.MemoryID `json:"target_memory_id"`
	WhenTruthStatus  string         `json:"when_truth_status,omitempty"`
	TruthStatus      string         `json:"truth_status,omitempty"`
	ConfidenceDelta  float64        `json:"confidence_delta,omitempty"`
	Summary          string         `json:"summary,omitempty"`
	AddMemoryID      model.MemoryID `json:"add_memory_id,omitempty"`
	AddMemoryContent string         `json:"add_memory_content,omitempty"`
}

type ReconcileDirector struct {
	id    string
	cases []ReconcileCase
}

func NewReconcileDirector(id string, cases []ReconcileCase) ReconcileDirector {
	return ReconcileDirector{
		id:    id,
		cases: append([]ReconcileCase(nil), cases...),
	}
}

func (d ReconcileDirector) ID() string {
	return d.id
}

func (d ReconcileDirector) Propose(ctx Context) ([]model.WorldEvent, error) {
	events := make([]model.WorldEvent, 0, len(d.cases))
	for _, c := range d.cases {
		memory, ok := memoryByID(ctx.World.Memory, c.TargetMemoryID)
		if !ok {
			continue
		}
		if c.WhenTruthStatus != "" && memory.TruthStatus != c.WhenTruthStatus {
			continue
		}
		events = append(events, eventFromReconcileCase(c))
	}
	return model.CloneEvents(events), nil
}

func memoryByID(memories []model.MemoryRecord, id model.MemoryID) (model.MemoryRecord, bool) {
	for _, memory := range memories {
		if memory.ID == id {
			return memory, true
		}
	}
	return model.MemoryRecord{}, false
}

func eventFromReconcileCase(c ReconcileCase) model.WorldEvent {
	payload := map[string]model.Value{
		"confidence_delta": {Kind: model.ValueKindNumber, Raw: c.ConfidenceDelta},
	}
	if c.TruthStatus != "" {
		payload["truth_status"] = model.Value{Kind: model.ValueKindString, Raw: c.TruthStatus}
	}
	if c.Summary != "" {
		payload["summary"] = model.Value{Kind: model.ValueKindString, Raw: c.Summary}
	}
	if c.AddMemoryID != "" {
		payload["add_memory_id"] = model.Value{Kind: model.ValueKindString, Raw: string(c.AddMemoryID)}
	}
	if c.AddMemoryContent != "" {
		payload["add_memory_content"] = model.Value{Kind: model.ValueKindString, Raw: c.AddMemoryContent}
	}
	return model.WorldEvent{
		ID:     c.EventID,
		Type:   model.EventTypeRemember,
		Source: model.EventSourceDirector,
		Effects: []model.Effect{{
			Kind:     model.EffectReconcileMemory,
			TargetID: string(c.TargetMemoryID),
			Payload:  payload,
		}},
	}
}
