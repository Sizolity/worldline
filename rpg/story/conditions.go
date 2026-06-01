package story

import (
	"fmt"

	"github.com/sizolity/worldline/world/model"
)

// Condition kinds supported by the MVP scheduler.
const (
	CondThreadTensionGTE = "thread_tension_gte"
	CondEntityStateEq    = "entity_state_eq"
	CondFactExists       = "fact_exists"
)

// evalCondition evaluates a milestone condition against a world snapshot.
// Returns (true, nil) if the condition holds.
func evalCondition(c MilestoneCondition, world model.World) (bool, error) {
	switch c.Kind {
	case CondThreadTensionGTE:
		return evalThreadTensionGTE(c.Args, world)
	case CondEntityStateEq:
		return evalEntityStateEq(c.Args, world)
	case CondFactExists:
		return evalFactExists(c.Args, world)
	default:
		return false, fmt.Errorf("unknown condition kind %q", c.Kind)
	}
}

func evalThreadTensionGTE(args map[string]any, world model.World) (bool, error) {
	threadID, err := stringArg(args, "thread_id")
	if err != nil {
		return false, err
	}
	threshold, err := floatArg(args, "threshold")
	if err != nil {
		return false, err
	}
	for _, th := range world.Threads {
		if string(th.ID) == threadID {
			return th.Tension >= threshold, nil
		}
	}
	return false, nil
}

func evalEntityStateEq(args map[string]any, world model.World) (bool, error) {
	entityID, err := stringArg(args, "entity_id")
	if err != nil {
		return false, err
	}
	key, err := stringArg(args, "key")
	if err != nil {
		return false, err
	}
	want, ok := args["value"]
	if !ok {
		return false, fmt.Errorf("entity_state_eq: missing arg %q", "value")
	}
	e, ok := world.Entities[model.EntityID(entityID)]
	if !ok {
		return false, nil
	}
	v, ok := e.State[key]
	if !ok {
		return false, nil
	}
	return valuesEqual(v.Raw, want), nil
}

func evalFactExists(args map[string]any, world model.World) (bool, error) {
	subjectID, err := stringArg(args, "subject_id")
	if err != nil {
		return false, err
	}
	predicate, err := stringArg(args, "predicate")
	if err != nil {
		return false, err
	}
	for _, f := range world.Facts {
		if string(f.SubjectID) == subjectID && f.Predicate == predicate {
			return true, nil
		}
	}
	return false, nil
}

// --- arg coercion ---

func stringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing arg %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("arg %q must be string, got %T", key, v)
	}
	return s, nil
}

func floatArg(args map[string]any, key string) (float64, error) {
	v, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("missing arg %q", key)
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("arg %q must be numeric, got %T", key, v)
	}
}

// valuesEqual compares two values with simple JSON-friendly semantics:
// numerics compared as float64, strings/bools by ==. Other types fall back to
// interface equality, which is safe for primitives but won't recurse maps.
func valuesEqual(a, b any) bool {
	if af, aOK := toFloat(a); aOK {
		if bf, bOK := toFloat(b); bOK {
			return af == bf
		}
	}
	return a == b
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
