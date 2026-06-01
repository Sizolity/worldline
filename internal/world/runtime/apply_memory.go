package runtime

import (
	"fmt"
	"slices"

	"github.com/sizolity/worldline/internal/world/model"
)

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

func applyRemoveMemory(world model.World, effect model.Effect) (model.World, error) {
	for i, mem := range world.Memory {
		if string(mem.ID) == effect.TargetID {
			world.Memory = append(world.Memory[:i], world.Memory[i+1:]...)
			return world, nil
		}
	}
	return model.World{}, fmt.Errorf("memory %q not found", effect.TargetID)
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
