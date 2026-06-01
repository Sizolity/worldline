package runtime

import (
	"encoding/json"
	"fmt"

	"github.com/sizolity/worldline/internal/world/model"
)

func payloadString(effect model.Effect, key string) (string, error) {
	value, ok := effect.Payload[key]
	if !ok {
		return "", fmt.Errorf("payload.%s is required", key)
	}
	raw, ok := value.Raw.(string)
	if !ok || raw == "" {
		return "", fmt.Errorf("payload.%s must be a non-empty string", key)
	}
	return raw, nil
}

func payloadObject(effect model.Effect, key string) (map[string]any, error) {
	value, ok := effect.Payload[key]
	if !ok {
		return nil, fmt.Errorf("payload.%s is required", key)
	}
	raw, ok := value.Raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("payload.%s must be an object", key)
	}
	return model.CloneAnyMap(raw), nil
}

func payloadOptionalString(effect model.Effect, key string) string {
	value, ok := effect.Payload[key]
	if !ok {
		return ""
	}
	raw, _ := value.Raw.(string)
	return raw
}

func payloadOptionalFloat(effect model.Effect, key string) float64 {
	value, ok := effect.Payload[key]
	if !ok {
		return 0
	}
	switch raw := value.Raw.(type) {
	case float64:
		return raw
	case float32:
		return float64(raw)
	case int:
		return float64(raw)
	default:
		return 0
	}
}

func payloadEntityID(effect model.Effect, key string) (model.EntityID, error) {
	raw, err := payloadString(effect, key)
	if err != nil {
		return "", err
	}
	if err := model.ValidateID(raw); err != nil {
		return "", fmt.Errorf("payload.%s: %w", key, err)
	}
	return model.EntityID(raw), nil
}

func payloadWorldEvent(effect model.Effect, key string) (model.WorldEvent, error) {
	value, ok := effect.Payload[key]
	if !ok {
		return model.WorldEvent{}, fmt.Errorf("payload.%s is required", key)
	}
	switch raw := value.Raw.(type) {
	case model.WorldEvent:
		return raw, nil
	case map[string]any:
		data, err := json.Marshal(raw)
		if err != nil {
			return model.WorldEvent{}, fmt.Errorf("payload.%s: %w", key, err)
		}
		var event model.WorldEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return model.WorldEvent{}, fmt.Errorf("payload.%s: %w", key, err)
		}
		return event, nil
	default:
		return model.WorldEvent{}, fmt.Errorf("payload.%s must be a world event", key)
	}
}

func payloadWorldTime(effect model.Effect, key string) (model.WorldTime, error) {
	value, ok := effect.Payload[key]
	if !ok {
		return model.WorldTime{}, fmt.Errorf("payload.%s is required", key)
	}
	switch raw := value.Raw.(type) {
	case model.WorldTime:
		return raw, nil
	case map[string]any:
		data, err := json.Marshal(raw)
		if err != nil {
			return model.WorldTime{}, fmt.Errorf("payload.%s: %w", key, err)
		}
		var worldTime model.WorldTime
		if err := json.Unmarshal(data, &worldTime); err != nil {
			return model.WorldTime{}, fmt.Errorf("payload.%s: %w", key, err)
		}
		return worldTime, nil
	default:
		return model.WorldTime{}, fmt.Errorf("payload.%s must be a world time", key)
	}
}
