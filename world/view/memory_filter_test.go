package view

import (
	"testing"

	"github.com/sizolity/worldline/world/model"
)

func testMemories() []model.MemoryRecord {
	return []model.MemoryRecord{
		{ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "World is at peace.", Importance: 0.9, TruthStatus: model.TruthStatusTrue},
		{ID: "m2", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "alice"}, Content: "Alice remembers the storm.", Importance: 0.7, SubjectIDs: []model.EntityID{"alice"}, TruthStatus: model.TruthStatusUnknown},
		{ID: "m3", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "A secret was buried.", Importance: 0.5, TruthStatus: model.TruthStatusSecret},
		{ID: "m4", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "bob"}, Content: "Bob suspects a conspiracy.", Importance: 0.3, SubjectIDs: []model.EntityID{"bob", "alice"}, TruthStatus: model.TruthStatusDisputed},
		{ID: "m5", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindFaction, ID: "guild"}, Content: "The guild knows.", Importance: 0.1, TruthStatus: model.TruthStatusTrue},
	}
}

func TestMemoryFilterNoFilter(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{}
	result := f.Filter(testMemories())
	if len(result) != 5 {
		t.Errorf("expected 5, got %d", len(result))
	}
}

func TestMemoryFilterNilSlice(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{MaxCount: 10}
	result := f.Filter(nil)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestMemoryFilterByOwnerKinds(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{OwnerKinds: []string{model.MemoryOwnerKindWorld}}
	result := f.Filter(testMemories())
	if len(result) != 2 {
		t.Errorf("expected 2 world memories, got %d", len(result))
	}
	for _, m := range result {
		if m.Owner.Kind != model.MemoryOwnerKindWorld {
			t.Errorf("got owner kind %q", m.Owner.Kind)
		}
	}
}

func TestMemoryFilterByMultipleOwnerKinds(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{OwnerKinds: []string{model.MemoryOwnerKindWorld, model.MemoryOwnerKindCharacter}}
	result := f.Filter(testMemories())
	if len(result) != 4 {
		t.Errorf("expected 4, got %d", len(result))
	}
}

func TestMemoryFilterByMinImportance(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{MinImportance: 0.5}
	result := f.Filter(testMemories())
	if len(result) != 3 {
		t.Errorf("expected 3 (>=0.5), got %d", len(result))
	}
	for _, m := range result {
		if m.Importance < 0.5 {
			t.Errorf("memory %s has importance %f", m.ID, m.Importance)
		}
	}
}

func TestMemoryFilterByExcludeTruthStatus(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{ExcludeTruthStatus: []string{model.TruthStatusSecret}}
	result := f.Filter(testMemories())
	if len(result) != 4 {
		t.Errorf("expected 4, got %d", len(result))
	}
	for _, m := range result {
		if m.TruthStatus == model.TruthStatusSecret {
			t.Errorf("secret memory should be excluded: %s", m.ID)
		}
	}
}

func TestMemoryFilterBySubjectIDs(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{SubjectIDs: []model.EntityID{"alice"}}
	result := f.Filter(testMemories())
	// m1 (no subjects → passes), m2 (has alice → passes), m3 (no subjects → passes),
	// m4 (has alice → passes), m5 (no subjects → passes)
	if len(result) != 5 {
		t.Errorf("expected 5 (empty SubjectIDs always pass), got %d", len(result))
	}
}

func TestMemoryFilterBySubjectIDsExcludesNonMatching(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{SubjectIDs: []model.EntityID{"charlie"}}
	result := f.Filter(testMemories())
	// m2 has {alice} → no overlap with charlie → excluded
	// m4 has {bob, alice} → no overlap with charlie → excluded
	// m1, m3, m5 have empty SubjectIDs → pass
	if len(result) != 3 {
		t.Errorf("expected 3 (empty SubjectIDs pass, non-matching excluded), got %d", len(result))
	}
}

func TestMemoryFilterMaxCount(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{MaxCount: 2}
	result := f.Filter(testMemories())
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
	if result[0].ID != "m1" || result[1].ID != "m2" {
		t.Errorf("expected top-2 by importance (m1=0.9, m2=0.7), got %s, %s", result[0].ID, result[1].ID)
	}
}

func TestMemoryFilterCombined(t *testing.T) {
	t.Parallel()
	f := MemoryFilter{
		OwnerKinds:         []string{model.MemoryOwnerKindWorld, model.MemoryOwnerKindCharacter},
		ExcludeTruthStatus: []string{model.TruthStatusSecret},
		MinImportance:      0.3,
		MaxCount:           2,
	}
	result := f.Filter(testMemories())
	// After owner filter: m1, m2, m3, m4
	// After exclude secret: m1, m2, m4
	// After min importance (>=0.3): m1(0.9), m2(0.7), m4(0.3)
	// MaxCount 2, sorted by importance: m1, m2
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
	if result[0].ID != "m1" {
		t.Errorf("first should be m1 (highest importance), got %s", result[0].ID)
	}
}

func TestMemoryFilterDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	memories := testMemories()
	original := make([]model.MemoryRecord, len(memories))
	copy(original, memories)

	f := MemoryFilter{MaxCount: 1}
	_ = f.Filter(memories)

	for i, m := range memories {
		if m.ID != original[i].ID {
			t.Errorf("input mutated at index %d: got %s, want %s", i, m.ID, original[i].ID)
		}
	}
}
