package director

import (
	"slices"

	"github.com/sizolity/worldline/world/model"
)

type ScriptDirector struct {
	id     string
	events []model.WorldEvent
}

func NewScriptDirector(id string, events []model.WorldEvent) ScriptDirector {
	return ScriptDirector{
		id:     id,
		events: cloneEvents(events),
	}
}

func (d ScriptDirector) ID() string {
	return d.id
}

func (d ScriptDirector) Propose(_ Context) ([]model.WorldEvent, error) {
	return cloneEvents(d.events), nil
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
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value model.Value) model.Value {
	value.Raw = cloneAny(value.Raw)
	return value
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = cloneAny(value)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	case map[string]int:
		out := make(map[string]int, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneAny(item)
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
