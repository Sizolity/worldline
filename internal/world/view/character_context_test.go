package view

import (
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func TestCharacterContextViewRequiresPerspective(t *testing.T) {
	world := model.World{ID: "test_world", Name: "Test World"}
	_, err := CharacterContextView{}.Render(world, CharacterContextRequest{})
	if err == nil {
		t.Fatal("Render returned nil without perspective")
	}
}

func TestCharacterContextViewRequiresExistingEntity(t *testing.T) {
	world := model.World{
		ID:       "test_world",
		Name:     "Test World",
		Entities: map[model.EntityID]model.Entity{},
	}
	_, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_b"})
	if err == nil {
		t.Fatal("Render returned nil for missing perspective entity")
	}
}

func TestCharacterContextViewIncludesOwnAndPublicWorldMemories(t *testing.T) {
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_b": {ID: "char_b", Type: "character", Name: "B"},
		},
		Memory: []model.MemoryRecord{
			{
				ID:          "memory_world_public",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "The king is dead.",
				TruthStatus: model.TruthStatusTrue,
				Confidence:  1.0,
				Importance:  0.8,
			},
			{
				ID:          "memory_b_private",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_b"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindBelief,
				Content:     "A killed the king.",
				TruthStatus: model.TruthStatusUnknown,
				Confidence:  0.8,
				Importance:  0.7,
			},
		},
	}

	got, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if got.Perspective.ID != "char_b" {
		t.Fatalf("perspective mismatch: %#v", got.Perspective)
	}
	if len(got.Memories) != 2 {
		t.Fatalf("Memories length = %d, want 2: %#v", len(got.Memories), got.Memories)
	}
}

func TestCharacterContextViewDoesNotLeakOtherOrSecretMemories(t *testing.T) {
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_b": {ID: "char_b", Type: "character", Name: "B"},
		},
		Memory: []model.MemoryRecord{
			{
				ID:          "memory_world_secret",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "D killed the king.",
				TruthStatus: model.TruthStatusSecret,
				Confidence:  1.0,
				Importance:  1.0,
			},
			{
				ID:          "memory_c_private",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindBelief,
				Content:     "A may have been framed.",
				TruthStatus: model.TruthStatusUnknown,
				Confidence:  0.5,
				Importance:  0.5,
			},
			{
				ID:          "memory_b_private",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_b"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindBelief,
				Content:     "A killed the king.",
				TruthStatus: model.TruthStatusUnknown,
				Confidence:  0.8,
				Importance:  0.7,
			},
		},
	}

	got, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if len(got.Memories) != 1 {
		t.Fatalf("Memories length = %d, want 1: %#v", len(got.Memories), got.Memories)
	}
	if got.Memories[0].ID != "memory_b_private" {
		t.Fatalf("unexpected visible memory: %#v", got.Memories)
	}
}

func TestCharacterContextViewHidesWorldMemoriesWithUnknownTruthStatus(t *testing.T) {
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_b": {ID: "char_b", Type: "character", Name: "B"},
		},
		Memory: []model.MemoryRecord{{
			ID:          "memory_world_invalid",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
			Content:     "Hidden by malformed status.",
			TruthStatus: "hidden",
		}},
	}

	got, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if len(got.Memories) != 0 {
		t.Fatalf("malformed world memory was visible: %#v", got.Memories)
	}
}
