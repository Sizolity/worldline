package runtime

import (
	"fmt"

	"github.com/sizolity/worldline/internal/world/model"
)

func (r Runtime) ApplyEvent(world model.World, event model.WorldEvent) (model.World, error) {
	if err := event.Validate(); err != nil {
		return model.World{}, err
	}
	if err := r.evaluateRules(world, event); err != nil {
		return model.World{}, err
	}
	world = cloneWorldForMutation(world)
	for i, effect := range event.Effects {
		var err error
		world, err = applyEffect(world, effect)
		if err != nil {
			return model.World{}, fmt.Errorf("effect %d: %w", i, err)
		}
	}
	world.EventLog = append(world.EventLog, event)
	return world, nil
}

func applyEffect(world model.World, effect model.Effect) (model.World, error) {
	switch effect.Kind {
	case model.EffectSetFact:
		return applySetFact(world, effect)
	case model.EffectUpdateEntityState:
		return applyUpdateEntityState(world, effect)
	case model.EffectSetEntityComponent:
		return applySetEntityComponent(world, effect)
	case model.EffectAddRelation:
		return applyAddRelation(world, effect)
	case model.EffectRemoveRelation:
		return applyRemoveRelation(world, effect)
	case model.EffectAddMemory:
		return applyAddMemory(world, effect)
	case model.EffectReviseMemory:
		return applyReviseMemory(world, effect)
	case model.EffectReconcileMemory:
		return applyReconcileMemory(world, effect)
	case model.EffectRemoveMemory:
		return applyRemoveMemory(world, effect)
	case model.EffectRemoveFact:
		return applyRemoveFact(world, effect)
	case model.EffectEnqueueEvent:
		return applyEnqueueEvent(world, effect)
	case model.EffectOpenThread:
		return applyOpenThread(world, effect)
	case model.EffectUpdateThread:
		return applyUpdateThread(world, effect)
	case model.EffectCloseThread:
		return applyCloseThread(world, effect)
	case model.EffectAddEntity:
		return applyAddEntity(world, effect)
	case model.EffectRemoveEntity:
		return applyRemoveEntity(world, effect)
	default:
		return model.World{}, fmt.Errorf("unsupported effect kind %q", effect.Kind)
	}
}

func applySetFact(world model.World, effect model.Effect) (model.World, error) {
	subjectID, err := payloadEntityID(effect, "subject_id")
	if err != nil {
		return model.World{}, err
	}
	predicate, err := payloadString(effect, "predicate")
	if err != nil {
		return model.World{}, err
	}
	value, ok := effect.Payload["value"]
	if !ok {
		return model.World{}, fmt.Errorf("payload.value is required")
	}
	world.Facts = append(world.Facts, model.Fact{
		ID:        model.FactID(effect.TargetID),
		SubjectID: subjectID,
		Predicate: predicate,
		Value:     value,
	})
	return world, nil
}

func applyUpdateEntityState(world model.World, effect model.Effect) (model.World, error) {
	entityID := model.EntityID(effect.TargetID)
	entity, ok := world.Entities[entityID]
	if !ok {
		return model.World{}, fmt.Errorf("entity %q not found", effect.TargetID)
	}
	if entity.State == nil {
		entity.State = map[string]model.Value{}
	}
	for key, value := range effect.Payload {
		entity.State[key] = value
	}
	if world.Entities == nil {
		world.Entities = map[model.EntityID]model.Entity{}
	}
	world.Entities[entityID] = entity
	return world, nil
}

func applySetEntityComponent(world model.World, effect model.Effect) (model.World, error) {
	entityID := model.EntityID(effect.TargetID)
	entity, ok := world.Entities[entityID]
	if !ok {
		return model.World{}, fmt.Errorf("entity %q not found", effect.TargetID)
	}
	component, err := payloadString(effect, "component")
	if err != nil {
		return model.World{}, err
	}
	data, err := payloadObject(effect, "data")
	if err != nil {
		return model.World{}, err
	}
	if entity.Components == nil {
		entity.Components = map[string]any{}
	}
	entity.Components[component] = data
	if err := entity.Validate(); err != nil {
		return model.World{}, err
	}
	if world.Entities == nil {
		world.Entities = map[model.EntityID]model.Entity{}
	}
	world.Entities[entityID] = entity
	return world, nil
}

func applyAddRelation(world model.World, effect model.Effect) (model.World, error) {
	relationType, err := payloadString(effect, "type")
	if err != nil {
		return model.World{}, err
	}
	sourceID, err := payloadEntityID(effect, "source_id")
	if err != nil {
		return model.World{}, err
	}
	targetID, err := payloadEntityID(effect, "target_id")
	if err != nil {
		return model.World{}, err
	}
	world.Relations = append(world.Relations, model.Relation{
		ID:       model.RelationID(effect.TargetID),
		Type:     relationType,
		SourceID: sourceID,
		TargetID: targetID,
	})
	return world, nil
}

func applyRemoveRelation(world model.World, effect model.Effect) (model.World, error) {
	for i, rel := range world.Relations {
		if string(rel.ID) == effect.TargetID {
			world.Relations = append(world.Relations[:i], world.Relations[i+1:]...)
			return world, nil
		}
	}
	return model.World{}, fmt.Errorf("relation %q not found", effect.TargetID)
}

func applyRemoveFact(world model.World, effect model.Effect) (model.World, error) {
	for i, fact := range world.Facts {
		if string(fact.ID) == effect.TargetID {
			world.Facts = append(world.Facts[:i], world.Facts[i+1:]...)
			return world, nil
		}
	}
	return model.World{}, fmt.Errorf("fact %q not found", effect.TargetID)
}

func applyEnqueueEvent(world model.World, effect model.Effect) (model.World, error) {
	event, err := payloadWorldEvent(effect, "event")
	if err != nil {
		return model.World{}, err
	}
	item := model.EventQueueItem{
		Event:     event,
		Priority:  int(payloadOptionalFloat(effect, "priority")),
		CreatedBy: payloadOptionalString(effect, "created_by"),
	}
	if _, ok := effect.Payload["not_before"]; ok {
		notBefore, err := payloadWorldTime(effect, "not_before")
		if err != nil {
			return model.World{}, err
		}
		item.NotBefore = notBefore
	}
	if err := item.Validate(); err != nil {
		return model.World{}, fmt.Errorf("payload.event_queue_item: %w", err)
	}
	world.EventQueue = append(world.EventQueue, item)
	return world, nil
}

func applyAddEntity(world model.World, effect model.Effect) (model.World, error) {
	entityID := model.EntityID(effect.TargetID)
	if world.Entities != nil {
		if _, ok := world.Entities[entityID]; ok {
			return model.World{}, fmt.Errorf("entity %q already exists", effect.TargetID)
		}
	}
	entityType, err := payloadString(effect, "type")
	if err != nil {
		return model.World{}, err
	}
	name, err := payloadString(effect, "name")
	if err != nil {
		return model.World{}, err
	}
	entity := model.Entity{
		ID:          entityID,
		Type:        entityType,
		Name:        name,
		Description: payloadOptionalString(effect, "description"),
	}
	if err := entity.Validate(); err != nil {
		return model.World{}, err
	}
	if world.Entities == nil {
		world.Entities = map[model.EntityID]model.Entity{}
	}
	world.Entities[entityID] = entity
	return world, nil
}

func applyRemoveEntity(world model.World, effect model.Effect) (model.World, error) {
	entityID := model.EntityID(effect.TargetID)
	if _, ok := world.Entities[entityID]; !ok {
		return model.World{}, fmt.Errorf("entity %q not found", effect.TargetID)
	}
	delete(world.Entities, entityID)
	return world, nil
}
