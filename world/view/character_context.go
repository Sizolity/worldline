package view

import (
	"fmt"

	"github.com/sizolity/worldline/world/model"
)

type CharacterContextRequest struct {
	PerspectiveID model.EntityID
}

type CharacterContext struct {
	Perspective model.Entity
	Memories    []model.MemoryRecord
}

type CharacterContextView struct{}

func (v CharacterContextView) Render(world model.World, req CharacterContextRequest) (CharacterContext, error) {
	if req.PerspectiveID == "" {
		return CharacterContext{}, fmt.Errorf("perspective_id is required")
	}
	if err := model.ValidateID(string(req.PerspectiveID)); err != nil {
		return CharacterContext{}, fmt.Errorf("perspective_id: %w", err)
	}
	perspective, ok := world.Entities[req.PerspectiveID]
	if !ok {
		return CharacterContext{}, fmt.Errorf("perspective entity %q not found", req.PerspectiveID)
	}
	return CharacterContext{
		Perspective: perspective,
		Memories:    VisibleMemoriesForCharacter(world.Memory, req.PerspectiveID),
	}, nil
}

func VisibleMemoriesForCharacter(memories []model.MemoryRecord, characterID model.EntityID) []model.MemoryRecord {
	visible := make([]model.MemoryRecord, 0, len(memories))
	for _, memory := range memories {
		if isVisibleMemoryForCharacter(memory, characterID) {
			visible = append(visible, memory)
		}
	}
	return visible
}

func isVisibleMemoryForCharacter(memory model.MemoryRecord, characterID model.EntityID) bool {
	switch memory.Owner.Kind {
	case model.MemoryOwnerKindCharacter:
		return memory.Owner.ID == string(characterID)
	case model.MemoryOwnerKindWorld:
		return isPublicWorldTruthStatus(memory.TruthStatus)
	default:
		return false
	}
}

func isPublicWorldTruthStatus(status string) bool {
	switch status {
	case "", model.TruthStatusTrue, model.TruthStatusFalse, model.TruthStatusUnknown, model.TruthStatusDisputed, model.TruthStatusOutdated:
		return true
	default:
		return false
	}
}
