package director

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/sizolity/worldline/world/model"
)

const DefaultMaxRepairAttempts = 2

// DefaultSystemPrompt documents the WorldEvent schema for LLM generators.
const DefaultSystemPrompt = `You are a world director for a narrative simulation engine.
Given the current world state as JSON, propose one or more world events as a JSON array.

Each event MUST have:
- "id": unique snake_case identifier (e.g. "event_merchant_arrives")
- "type": one of "note", "world_fact_changed", "remember", "move", "thread_changed"
- "source": always "director"
- "description": one-sentence narrative description

Events MAY include "effects" to mutate world state. Each effect has:
- "kind": the mutation type
- "target_id": the target entity/fact/memory/thread ID
- "payload": key-value pairs where values are {"kind":"string|number|boolean|entity_ref","raw":<value>}

Supported effect kinds:
- "set_fact": payload needs "subject_id" (entity_ref), "predicate" (string), "value" (any)
- "update_entity_state": target_id is entity, payload keys become state entries
- "add_memory": target_id is new memory ID, payload needs "owner_kind" (string: "world"|"character"), "owner_id" (string, if character), "scope" (string: "factual"|"subjective"), "memory_kind" (string: "observation"|"belief"|"rumor"), "content" (string), "truth_status" (string: "true"|"unknown"|"disputed"|"secret")
- "open_thread": target_id is new thread ID, payload needs "title" (string), "kind" (string: "mystery"|"quest"|"conflict"|"theme")
- "close_thread": target_id is existing thread ID
- "add_relation": target_id is new relation ID, payload needs "type" (string), "source_id" (entity_ref), "relation_target_id" (entity_ref)
- "add_entity": target_id is new entity ID, payload needs "type" (string: "character"|"location"|"item"), "name" (string)

A simple narrative event with no world mutation:
[{"id":"event_dawn","type":"note","source":"director","description":"Dawn breaks."}]

An event that also sets a world fact:
[{"id":"event_gate_sealed","type":"world_fact_changed","source":"director","description":"The city gate is sealed.","effects":[{"kind":"set_fact","target_id":"fact_gate","payload":{"subject_id":{"kind":"entity_ref","raw":"city_gate"},"predicate":{"kind":"string","raw":"status"},"value":{"kind":"string","raw":"sealed"}}}]}]

Return ONLY a valid JSON array. No markdown, no explanation.`

func buildWorldPrompt(w model.World) string {
	data, err := json.Marshal(worldPromptContext{
		WorldID:     string(w.ID),
		Name:        w.Name,
		Description: w.Description,
		Clock:       w.Clock.Sequence,
		Entities:    entitySummaries(w.Entities),
		Facts:       factSummaries(w.Facts),
		Relations:   relationSummaries(w.Relations),
		Memories:    memorySummaries(w.Memory),
		Threads:     threadSummaries(w.Threads),
	})
	if err != nil {
		return fmt.Sprintf(`{"world_id":%q,"name":%q}`, w.ID, w.Name)
	}
	return string(data)
}

func parseEventResponse(response string) ([]model.WorldEvent, error) {
	cleaned := stripMarkdownFences(response)
	var events []model.WorldEvent
	if err := json.Unmarshal([]byte(cleaned), &events); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("event[%d]: %w", i, err)
		}
	}
	return cloneEvents(events), nil
}

func stripMarkdownFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	idx := strings.Index(trimmed, "\n")
	if idx < 0 {
		return trimmed
	}
	inner := trimmed[idx+1:]
	if last := strings.LastIndex(inner, "```"); last >= 0 {
		inner = inner[:last]
	}
	return strings.TrimSpace(inner)
}

type worldPromptContext struct {
	WorldID     string            `json:"world_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Clock       int64             `json:"clock"`
	Entities    []entitySummary   `json:"entities,omitempty"`
	Facts       []factSummary     `json:"facts,omitempty"`
	Relations   []relationSummary `json:"relations,omitempty"`
	Memories    []memorySummary   `json:"memories,omitempty"`
	Threads     []threadSummary   `json:"threads,omitempty"`
}

type entitySummary struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	State       map[string]any `json:"state,omitempty"`
}

type factSummary struct {
	ID        string `json:"id"`
	SubjectID string `json:"subject_id"`
	Predicate string `json:"predicate"`
	Value     any    `json:"value"`
}

type relationSummary struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
}

type memorySummary struct {
	ID          string `json:"id"`
	OwnerID     string `json:"owner_id,omitempty"`
	Content     string `json:"content"`
	TruthStatus string `json:"truth_status"`
}

type threadSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func entitySummaries(entities map[model.EntityID]model.Entity) []entitySummary {
	out := make([]entitySummary, 0, len(entities))
	for _, e := range entities {
		var state map[string]any
		if len(e.State) > 0 {
			state = make(map[string]any, len(e.State))
			for k, v := range e.State {
				state[k] = v.Raw
			}
		}
		out = append(out, entitySummary{
			ID:          string(e.ID),
			Type:        e.Type,
			Name:        e.Name,
			Description: e.Description,
			State:       state,
		})
	}
	return out
}

func factSummaries(facts []model.Fact) []factSummary {
	out := make([]factSummary, 0, len(facts))
	for _, f := range facts {
		out = append(out, factSummary{
			ID:        string(f.ID),
			SubjectID: string(f.SubjectID),
			Predicate: f.Predicate,
			Value:     f.Value.Raw,
		})
	}
	return out
}

func relationSummaries(relations []model.Relation) []relationSummary {
	out := make([]relationSummary, 0, len(relations))
	for _, r := range relations {
		out = append(out, relationSummary{
			ID:       string(r.ID),
			Type:     r.Type,
			SourceID: string(r.SourceID),
			TargetID: string(r.TargetID),
		})
	}
	return out
}

func memorySummaries(memories []model.MemoryRecord) []memorySummary {
	out := make([]memorySummary, 0, len(memories))
	for _, m := range memories {
		out = append(out, memorySummary{
			ID:          string(m.ID),
			OwnerID:     m.Owner.ID,
			Content:     m.Content,
			TruthStatus: m.TruthStatus,
		})
	}
	return out
}

func threadSummaries(threads []model.WorldThread) []threadSummary {
	out := make([]threadSummary, 0, len(threads))
	for _, th := range threads {
		out = append(out, threadSummary{
			ID:     string(th.ID),
			Title:  th.Title,
			Status: th.Status,
		})
	}
	return out
}

func cloneEvents(events []model.WorldEvent) []model.WorldEvent {
	if events == nil {
		return []model.WorldEvent{}
	}
	out := slices.Clone(events)
	for i := range out {
		out[i].ActorIDs = slices.Clone(out[i].ActorIDs)
		out[i].TargetIDs = slices.Clone(out[i].TargetIDs)
		out[i].Effects = cloneEffects(out[i].Effects)
	}
	return out
}

func cloneEffects(effects []model.Effect) []model.Effect {
	if effects == nil {
		return nil
	}
	out := slices.Clone(effects)
	for i := range out {
		out[i].Payload = cloneValueMap(out[i].Payload)
	}
	return out
}

func cloneValueMap(in map[string]model.Value) map[string]model.Value {
	if in == nil {
		return nil
	}
	out := make(map[string]model.Value, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
