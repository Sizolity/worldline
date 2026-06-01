package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/sizolity/worldline/world/director"
	"github.com/sizolity/worldline/world/model"
)

type Runtime struct {
	Rules           []Rule
	Directors       []director.Director
	EventQueueLimit int
}

type StepResult struct {
	World         model.World        `json:"world"`
	Proposals     []model.WorldEvent `json:"proposals"`
	AppliedEvents []model.WorldEvent `json:"applied_events"`
	SkippedEvents []model.WorldEvent `json:"skipped_events,omitempty"`
}

func (r Runtime) Step(ctx context.Context, world model.World) (StepResult, error) {
	result := StepResult{
		World:         world,
		Proposals:     []model.WorldEvent{},
		AppliedEvents: []model.WorldEvent{},
		SkippedEvents: []model.WorldEvent{},
	}
	for _, d := range r.Directors {
		proposals, err := d.Propose(director.Context{Ctx: ctx, World: cloneWorldForMutation(result.World)})
		if err != nil {
			return result, fmt.Errorf("director %q: %w", d.ID(), err)
		}
		result.Proposals = append(result.Proposals, proposals...)
	}
	for i, proposal := range result.Proposals {
		next, err := r.ApplyEvent(result.World, proposal)
		if err != nil {
			return result, fmt.Errorf("proposal %d: %w", i, err)
		}
		result.World = next
		result.AppliedEvents = append(result.AppliedEvents, proposal)
	}
	retriedThisStep := map[model.EventID]bool{}
	for i := 0; i < r.EventQueueLimit && len(result.World.EventQueue) > 0; i++ {
		queueIndex, ok := nextReadyQueueIndexExcluding(result.World, retriedThisStep)
		if !ok {
			break
		}
		item := result.World.EventQueue[queueIndex]
		result.World.EventQueue = removeQueueItem(result.World.EventQueue, queueIndex)
		next, err := r.ApplyEvent(result.World, item.Event)
		if err != nil {
			switch item.ErrorPolicy {
			case model.QueueErrorPolicySkip:
				result.SkippedEvents = append(result.SkippedEvents, item.Event)
				continue
			case model.QueueErrorPolicyRetry:
				item.Attempts++
				if item.MaxAttempts > 0 && item.Attempts >= item.MaxAttempts {
					result.SkippedEvents = append(result.SkippedEvents, item.Event)
					continue
				}
				retriedThisStep[item.Event.ID] = true
				result.World.EventQueue = append(result.World.EventQueue, item)
				continue
			default:
				return result, fmt.Errorf("queued event %d: %w", i, err)
			}
		}
		result.World = next
		result.AppliedEvents = append(result.AppliedEvents, item.Event)
	}
	result.World.Clock = advanceClock(result.World.Clock)
	return result, nil
}

type RunResult struct {
	World            model.World        `json:"world"`
	StepsCompleted   int                `json:"steps_completed"`
	AllAppliedEvents []model.WorldEvent `json:"all_applied_events"`
}

func (r Runtime) Run(ctx context.Context, world model.World, steps int) (RunResult, error) {
	result := RunResult{
		World:            world,
		AllAppliedEvents: []model.WorldEvent{},
	}
	for i := 0; i < steps; i++ {
		step, err := r.Step(ctx, result.World)
		if err != nil {
			return result, fmt.Errorf("step %d: %w", i, err)
		}
		result.World = step.World
		result.AllAppliedEvents = append(result.AllAppliedEvents, step.AppliedEvents...)
		result.StepsCompleted++
	}
	return result, nil
}

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

func cloneWorldForMutation(world model.World) model.World {
	world.Canon.Genre = slices.Clone(world.Canon.Genre)
	world.Canon.Tone = slices.Clone(world.Canon.Tone)
	world.Canon.StyleGuide = slices.Clone(world.Canon.StyleGuide)
	world.Canon.Laws = slices.Clone(world.Canon.Laws)
	world.Canon.Boundaries = slices.Clone(world.Canon.Boundaries)
	world.Canon.Secrets = slices.Clone(world.Canon.Secrets)
	world.Clock.Current.Calendar = cloneIntMap(world.Clock.Current.Calendar)
	world.Entities = cloneEntities(world.Entities)
	world.Relations = slices.Clone(world.Relations)
	world.Facts = cloneFacts(world.Facts)
	world.Rules = slices.Clone(world.Rules)
	world.Threads = slices.Clone(world.Threads)
	world.EventLog = cloneEvents(world.EventLog)
	world.EventQueue = cloneEventQueue(world.EventQueue)
	world.Memory = cloneMemories(world.Memory)
	world.Metadata.Tags = slices.Clone(world.Metadata.Tags)
	return world
}

func cloneEntities(entities map[model.EntityID]model.Entity) map[model.EntityID]model.Entity {
	if entities == nil {
		return nil
	}
	out := make(map[model.EntityID]model.Entity, len(entities))
	for id, entity := range entities {
		entity.Components = cloneAnyMap(entity.Components)
		entity.State = cloneValueMap(entity.State)
		entity.Tags = slices.Clone(entity.Tags)
		out[id] = entity
	}
	return out
}

func cloneFacts(facts []model.Fact) []model.Fact {
	out := slices.Clone(facts)
	for i := range out {
		out[i].Value = cloneValue(out[i].Value)
	}
	return out
}

func cloneMemories(memories []model.MemoryRecord) []model.MemoryRecord {
	out := slices.Clone(memories)
	for i := range out {
		out[i].SubjectIDs = slices.Clone(out[i].SubjectIDs)
		out[i].EventIDs = slices.Clone(out[i].EventIDs)
	}
	return out
}

func cloneEvents(events []model.WorldEvent) []model.WorldEvent {
	out := slices.Clone(events)
	for i := range out {
		out[i].ActorIDs = slices.Clone(out[i].ActorIDs)
		out[i].TargetIDs = slices.Clone(out[i].TargetIDs)
		out[i].Effects = cloneEffects(out[i].Effects)
	}
	return out
}

func cloneEventQueue(queue []model.EventQueueItem) []model.EventQueueItem {
	out := slices.Clone(queue)
	for i := range out {
		out[i].Event.ActorIDs = slices.Clone(out[i].Event.ActorIDs)
		out[i].Event.TargetIDs = slices.Clone(out[i].Event.TargetIDs)
		out[i].Event.Effects = cloneEffects(out[i].Event.Effects)
		out[i].NotBefore.Calendar = cloneIntMap(out[i].NotBefore.Calendar)
	}
	return out
}

func cloneEffects(effects []model.Effect) []model.Effect {
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

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneAny(value)
	}
	return out
}

func cloneIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	case map[string]int:
		return cloneIntMap(typed)
	case map[string]model.Value:
		return cloneValueMap(typed)
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

func nextReadyQueueIndexExcluding(world model.World, exclude map[model.EventID]bool) (int, bool) {
	bestIndex := -1
	for i, item := range world.EventQueue {
		if exclude[item.Event.ID] {
			continue
		}
		if !queueItemReady(world.Clock.Current, item) {
			continue
		}
		if bestIndex == -1 || item.Priority > world.EventQueue[bestIndex].Priority {
			bestIndex = i
		}
	}
	return bestIndex, bestIndex != -1
}

func queueItemReady(now model.WorldTime, item model.EventQueueItem) bool {
	if item.NotBefore.Kind == "" {
		return true
	}
	if item.NotBefore.Kind != now.Kind {
		return false
	}
	switch now.Kind {
	case model.WorldTimeTick, model.WorldTimeTurn, model.WorldTimeScene, model.WorldTimeChapter, model.WorldTimeDay:
		return item.NotBefore.Tick <= now.Tick
	default:
		return false
	}
}

func removeQueueItem(queue []model.EventQueueItem, index int) []model.EventQueueItem {
	out := make([]model.EventQueueItem, 0, len(queue)-1)
	out = append(out, queue[:index]...)
	out = append(out, queue[index+1:]...)
	return out
}

func advanceClock(clock model.WorldClock) model.WorldClock {
	clock.Sequence++
	if clock.Current.Kind == model.WorldTimeTick {
		clock.Current.Tick++
	}
	return clock
}

func (r Runtime) evaluateRules(world model.World, event model.WorldEvent) error {
	ctx := RuleContext{World: world}
	for _, rule := range r.Rules {
		decision := rule.Evaluate(ctx, event)
		if err := decision.Validate(); err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID(), err)
		}
		if decision.Status == RuleDecisionReject {
			if decision.Reason == "" {
				return fmt.Errorf("rule %q rejected event", rule.ID())
			}
			return fmt.Errorf("rule %q rejected event: %s", rule.ID(), decision.Reason)
		}
	}
	return nil
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

func applyAddMemory(world model.World, effect model.Effect) (model.World, error) {
	ownerKind, err := payloadString(effect, "owner_kind")
	if err != nil {
		return model.World{}, err
	}
	ownerID := ""
	if value, ok := effect.Payload["owner_id"]; ok {
		raw, ok := value.Raw.(string)
		if !ok {
			return model.World{}, fmt.Errorf("payload.owner_id must be a string")
		}
		ownerID = raw
	}
	content, err := payloadString(effect, "content")
	if err != nil {
		return model.World{}, err
	}
	memory := model.MemoryRecord{
		ID:          model.MemoryID(effect.TargetID),
		Owner:       model.MemoryOwner{Kind: ownerKind, ID: ownerID},
		Scope:       payloadOptionalString(effect, "scope"),
		Kind:        payloadOptionalString(effect, "kind"),
		Content:     content,
		TruthStatus: payloadOptionalString(effect, "truth_status"),
		Confidence:  payloadOptionalFloat(effect, "confidence"),
		Importance:  payloadOptionalFloat(effect, "importance"),
	}
	if err := memory.Validate(); err != nil {
		return model.World{}, err
	}
	world.Memory = append(world.Memory, memory)
	return world, nil
}

func applyReviseMemory(world model.World, effect model.Effect) (model.World, error) {
	for i, memory := range world.Memory {
		if string(memory.ID) != effect.TargetID {
			continue
		}
		if value := payloadOptionalString(effect, "content"); value != "" {
			memory.Content = value
		}
		if value := payloadOptionalString(effect, "summary"); value != "" {
			memory.Summary = value
		}
		if value := payloadOptionalString(effect, "truth_status"); value != "" {
			memory.TruthStatus = value
		}
		if _, ok := effect.Payload["confidence"]; ok {
			memory.Confidence = payloadOptionalFloat(effect, "confidence")
		}
		if _, ok := effect.Payload["importance"]; ok {
			memory.Importance = payloadOptionalFloat(effect, "importance")
		}
		if err := memory.Validate(); err != nil {
			return model.World{}, err
		}
		world.Memory[i] = memory
		return world, nil
	}
	return model.World{}, fmt.Errorf("memory %q not found", effect.TargetID)
}

func applyReconcileMemory(world model.World, effect model.Effect) (model.World, error) {
	for i, memory := range world.Memory {
		if string(memory.ID) != effect.TargetID {
			continue
		}
		if value := payloadOptionalString(effect, "content"); value != "" {
			memory.Content = value
		}
		if value := payloadOptionalString(effect, "summary"); value != "" {
			memory.Summary = value
		}
		if value := payloadOptionalString(effect, "truth_status"); value != "" {
			memory.TruthStatus = value
		}
		if _, ok := effect.Payload["confidence_delta"]; ok {
			memory.Confidence = clamp01(memory.Confidence + payloadOptionalFloat(effect, "confidence_delta"))
		}
		if err := memory.Validate(); err != nil {
			return model.World{}, err
		}
		world.Memory[i] = memory
		if newMemory, ok, err := reconciliationMemoryFromPayload(effect, memory); err != nil {
			return model.World{}, err
		} else if ok {
			world.Memory = append(world.Memory, newMemory)
		}
		return world, nil
	}
	return model.World{}, fmt.Errorf("memory %q not found", effect.TargetID)
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

func reconciliationMemoryFromPayload(effect model.Effect, reconciled model.MemoryRecord) (model.MemoryRecord, bool, error) {
	id := payloadOptionalString(effect, "add_memory_id")
	content := payloadOptionalString(effect, "add_memory_content")
	if id == "" && content == "" {
		return model.MemoryRecord{}, false, nil
	}
	if id == "" {
		return model.MemoryRecord{}, false, fmt.Errorf("payload.add_memory_id is required when add_memory_content is set")
	}
	if content == "" {
		return model.MemoryRecord{}, false, fmt.Errorf("payload.add_memory_content is required when add_memory_id is set")
	}
	memory := model.MemoryRecord{
		ID:          model.MemoryID(id),
		Owner:       reconciled.Owner,
		Scope:       reconciled.Scope,
		Kind:        model.MemoryKindBelief,
		SubjectIDs:  slices.Clone(reconciled.SubjectIDs),
		EventIDs:    slices.Clone(reconciled.EventIDs),
		Content:     content,
		TruthStatus: model.TruthStatusUnknown,
		Confidence:  0.5,
		Importance:  reconciled.Importance,
	}
	if value := payloadOptionalString(effect, "add_memory_truth_status"); value != "" {
		memory.TruthStatus = value
	}
	if _, ok := effect.Payload["add_memory_confidence"]; ok {
		memory.Confidence = clamp01(payloadOptionalFloat(effect, "add_memory_confidence"))
	}
	if _, ok := effect.Payload["add_memory_importance"]; ok {
		memory.Importance = clamp01(payloadOptionalFloat(effect, "add_memory_importance"))
	}
	if err := memory.Validate(); err != nil {
		return model.MemoryRecord{}, false, err
	}
	return memory, true, nil
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func applyOpenThread(world model.World, effect model.Effect) (model.World, error) {
	kind, err := payloadString(effect, "kind")
	if err != nil {
		return model.World{}, err
	}
	title, err := payloadString(effect, "title")
	if err != nil {
		return model.World{}, err
	}
	thread := model.WorldThread{
		ID:       model.ThreadID(effect.TargetID),
		Kind:     kind,
		Title:    title,
		Summary:  payloadOptionalString(effect, "summary"),
		Status:   payloadOptionalString(effect, "status"),
		Priority: payloadOptionalFloat(effect, "priority"),
		Tension:  payloadOptionalFloat(effect, "tension"),
	}
	if thread.Status == "" {
		thread.Status = model.ThreadStatusOpen
	}
	if err := thread.Validate(); err != nil {
		return model.World{}, err
	}
	world.Threads = append(world.Threads, thread)
	return world, nil
}

func applyUpdateThread(world model.World, effect model.Effect) (model.World, error) {
	for i, thread := range world.Threads {
		if string(thread.ID) != effect.TargetID {
			continue
		}
		thread = updateThreadFromPayload(thread, effect)
		if err := thread.Validate(); err != nil {
			return model.World{}, err
		}
		world.Threads[i] = thread
		return world, nil
	}
	return model.World{}, fmt.Errorf("thread %q not found", effect.TargetID)
}

func applyCloseThread(world model.World, effect model.Effect) (model.World, error) {
	for i, thread := range world.Threads {
		if string(thread.ID) != effect.TargetID {
			continue
		}
		thread = updateThreadFromPayload(thread, effect)
		if _, ok := effect.Payload["status"]; !ok {
			thread.Status = model.ThreadStatusResolved
		}
		if err := thread.Validate(); err != nil {
			return model.World{}, err
		}
		world.Threads[i] = thread
		return world, nil
	}
	return model.World{}, fmt.Errorf("thread %q not found", effect.TargetID)
}

func updateThreadFromPayload(thread model.WorldThread, effect model.Effect) model.WorldThread {
	if value := payloadOptionalString(effect, "kind"); value != "" {
		thread.Kind = value
	}
	if value := payloadOptionalString(effect, "title"); value != "" {
		thread.Title = value
	}
	if value := payloadOptionalString(effect, "summary"); value != "" {
		thread.Summary = value
	}
	if value := payloadOptionalString(effect, "status"); value != "" {
		thread.Status = value
	}
	if _, ok := effect.Payload["priority"]; ok {
		thread.Priority = payloadOptionalFloat(effect, "priority")
	}
	if _, ok := effect.Payload["tension"]; ok {
		thread.Tension = payloadOptionalFloat(effect, "tension")
	}
	return thread
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

func applyRemoveMemory(world model.World, effect model.Effect) (model.World, error) {
	for i, mem := range world.Memory {
		if string(mem.ID) == effect.TargetID {
			world.Memory = append(world.Memory[:i], world.Memory[i+1:]...)
			return world, nil
		}
	}
	return model.World{}, fmt.Errorf("memory %q not found", effect.TargetID)
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
	return cloneAnyMap(raw), nil
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
