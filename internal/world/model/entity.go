package model

import "fmt"

type Entity struct {
	ID          EntityID `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	// Aliases is the set of additional human-readable names this entity
	// is known by (nicknames, epithets, translations, code-names).
	// Distinct from Name (the canonical display name) and Tags (taxonomy).
	// The ingest pipeline populates this from extracted drafts; alias
	// resolvers may also push their findings here.
	Aliases    []string         `json:"aliases,omitempty"`
	Components map[string]any   `json:"components,omitempty"`
	State      map[string]Value `json:"state,omitempty"`
	Tags       []string         `json:"tags,omitempty"`
}

const (
	ComponentProfile   = "profile"
	ComponentActor     = "actor"
	ComponentSpatial   = "spatial"
	ComponentInventory = "inventory"
	ComponentStats     = "stats"
)

type ProfileComponent struct {
	Name        string
	Description string
}

type ActorComponent struct {
	CanAct bool
	Goals  []string
}

type SpatialComponent struct {
	LocationID EntityID
}

type InventoryComponent struct {
	ItemIDs []EntityID
}

type StatsComponent struct {
	Values map[string]Value
}

func (e Entity) Validate() error {
	if err := ValidateID(string(e.ID)); err != nil {
		return fmt.Errorf("entity.id: %w", err)
	}
	if e.Type == "" {
		return fmt.Errorf("entity.type is required")
	}
	if e.Name == "" {
		return fmt.Errorf("entity.name is required")
	}
	for i, alias := range e.Aliases {
		if alias == "" {
			return fmt.Errorf("entity.aliases[%d] must not be empty", i)
		}
	}
	for key, component := range e.Components {
		if err := validateComponent(key, component); err != nil {
			return fmt.Errorf("entity.components[%s]: %w", key, err)
		}
	}
	return nil
}

func (e Entity) ProfileComponent() (ProfileComponent, bool) {
	data, ok := componentObject(e.Components, ComponentProfile)
	if !ok {
		return ProfileComponent{}, false
	}
	if err := validateProfileComponent(data); err != nil {
		return ProfileComponent{}, false
	}
	return ProfileComponent{
		Name:        stringValue(data, "name"),
		Description: stringValue(data, "description"),
	}, true
}

func (e Entity) ActorComponent() (ActorComponent, bool) {
	data, ok := componentObject(e.Components, ComponentActor)
	if !ok {
		return ActorComponent{}, false
	}
	if err := validateActorComponent(data); err != nil {
		return ActorComponent{}, false
	}
	goals := []string{}
	if value, ok := data["goals"]; ok {
		goals, _ = stringList(value)
	}
	return ActorComponent{
		CanAct: boolValue(data, "can_act"),
		Goals:  append([]string(nil), goals...),
	}, true
}

func (e Entity) SpatialComponent() (SpatialComponent, bool) {
	data, ok := componentObject(e.Components, ComponentSpatial)
	if !ok {
		return SpatialComponent{}, false
	}
	if err := validateSpatialComponent(data); err != nil {
		return SpatialComponent{}, false
	}
	return SpatialComponent{
		LocationID: EntityID(stringValue(data, "location_id")),
	}, true
}

func (e Entity) InventoryComponent() (InventoryComponent, bool) {
	data, ok := componentObject(e.Components, ComponentInventory)
	if !ok {
		return InventoryComponent{}, false
	}
	if err := validateInventoryComponent(data); err != nil {
		return InventoryComponent{}, false
	}
	items := []string{}
	if value, ok := data["item_ids"]; ok {
		items, _ = stringList(value)
	}
	itemIDs := make([]EntityID, len(items))
	for i, item := range items {
		itemIDs[i] = EntityID(item)
	}
	return InventoryComponent{ItemIDs: itemIDs}, true
}

func (e Entity) StatsComponent() (StatsComponent, bool) {
	data, ok := componentObject(e.Components, ComponentStats)
	if !ok {
		return StatsComponent{}, false
	}
	if err := validateStatsComponent(data); err != nil {
		return StatsComponent{}, false
	}
	return StatsComponent{Values: valueMap(data["values"])}, true
}

func validateComponent(key string, component any) error {
	data, ok := component.(map[string]any)
	if !ok {
		return fmt.Errorf("component must be an object")
	}
	switch key {
	case ComponentProfile:
		return validateProfileComponent(data)
	case ComponentActor:
		return validateActorComponent(data)
	case ComponentSpatial:
		return validateSpatialComponent(data)
	case ComponentInventory:
		return validateInventoryComponent(data)
	case ComponentStats:
		return validateStatsComponent(data)
	default:
		return fmt.Errorf("unsupported component %q", key)
	}
}

func validateProfileComponent(data map[string]any) error {
	if err := optionalStringField(data, "name"); err != nil {
		return err
	}
	return optionalStringField(data, "description")
}

func validateActorComponent(data map[string]any) error {
	if value, ok := data["can_act"]; ok {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("can_act must be a boolean")
		}
	}
	return optionalStringListField(data, "goals")
}

func validateSpatialComponent(data map[string]any) error {
	if value, ok := data["location_id"]; ok {
		locationID, ok := value.(string)
		if !ok {
			return fmt.Errorf("location_id must be a string")
		}
		if err := ValidateID(locationID); err != nil {
			return fmt.Errorf("location_id: %w", err)
		}
	}
	return nil
}

func validateInventoryComponent(data map[string]any) error {
	if value, ok := data["item_ids"]; ok {
		items, err := stringList(value)
		if err != nil {
			return fmt.Errorf("item_ids: %w", err)
		}
		for i, id := range items {
			if err := ValidateID(id); err != nil {
				return fmt.Errorf("item_ids[%d]: %w", i, err)
			}
		}
	}
	return nil
}

func validateStatsComponent(data map[string]any) error {
	if value, ok := data["values"]; ok {
		switch value.(type) {
		case map[string]any, map[string]Value:
		default:
			return fmt.Errorf("values must be an object")
		}
	}
	return nil
}

func optionalStringField(data map[string]any, key string) error {
	if value, ok := data[key]; ok {
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s must be a string", key)
		}
	}
	return nil
}

func optionalStringListField(data map[string]any, key string) error {
	if value, ok := data[key]; ok {
		if _, err := stringList(value); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}

func stringList(value any) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []any:
		out := make([]string, len(typed))
		for i, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("[%d] must be a string", i)
			}
			out[i] = text
		}
		return out, nil
	default:
		return nil, fmt.Errorf("must be a string list")
	}
}

func componentObject(components map[string]any, key string) (map[string]any, bool) {
	component, ok := components[key]
	if !ok {
		return nil, false
	}
	data, ok := component.(map[string]any)
	if !ok {
		return nil, false
	}
	return data, true
}

func stringValue(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return value
}

func boolValue(data map[string]any, key string) bool {
	value, _ := data[key].(bool)
	return value
}

func valueMap(value any) map[string]Value {
	switch typed := value.(type) {
	case map[string]Value:
		out := make(map[string]Value, len(typed))
		for key, value := range typed {
			out[key] = value.Clone()
		}
		return out
	case map[string]any:
		out := make(map[string]Value, len(typed))
		for key, value := range typed {
			if typedValue, ok := value.(Value); ok {
				out[key] = typedValue.Clone()
				continue
			}
			if typedValue, ok := valueFromObject(value); ok {
				out[key] = typedValue
			}
		}
		return out
	default:
		return map[string]Value{}
	}
}

func valueFromObject(value any) (Value, bool) {
	data, ok := value.(map[string]any)
	if !ok {
		return Value{}, false
	}
	kind, _ := data["kind"].(string)
	out := Value{
		Kind: kind,
		Raw:  CloneAny(data["raw"]),
	}
	if unit, ok := data["unit"].(string); ok {
		out.Unit = unit
	}
	if source, ok := data["source"].(string); ok {
		out.Source = source
	}
	return out, true
}

func NewProfileComponent(name, description string) map[string]any {
	component := map[string]any{}
	if name != "" {
		component["name"] = name
	}
	if description != "" {
		component["description"] = description
	}
	return component
}

func NewActorComponent(canAct bool, goals []string) map[string]any {
	return map[string]any{
		"can_act": canAct,
		"goals":   append([]string(nil), goals...),
	}
}

func NewSpatialComponent(locationID EntityID) map[string]any {
	component := map[string]any{}
	if locationID != "" {
		component["location_id"] = string(locationID)
	}
	return component
}

func NewInventoryComponent(itemIDs ...EntityID) map[string]any {
	ids := make([]string, len(itemIDs))
	for i, id := range itemIDs {
		ids[i] = string(id)
	}
	return map[string]any{"item_ids": ids}
}

func NewStatsComponent(values map[string]Value) map[string]any {
	out := make(map[string]Value, len(values))
	for key, value := range values {
		out[key] = value
	}
	return map[string]any{"values": out}
}
