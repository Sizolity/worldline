// Package fog implements progressive world disclosure ("fog of war").
// The world is authored complete; this package tracks what has been revealed
// to the player/DM and filters the world accordingly before prompt construction.
package fog

import "github.com/sizolity/worldline/world/model"

// VisibilityLevel defines how much of an entity is visible to the DM.
type VisibilityLevel string

const (
	Hidden   VisibilityLevel = "hidden"   // not in DM context at all
	Known    VisibilityLevel = "known"    // DM sees name/type/tags only
	Explored VisibilityLevel = "explored" // DM sees full content
)

// DisclosureState tracks what the player has discovered in a specific world.
type DisclosureState struct {
	Entities  map[model.EntityID]EntityDisclosure   `json:"entities,omitempty"`
	Facts     map[model.FactID]bool                 `json:"facts,omitempty"`
	Relations map[model.RelationID]bool             `json:"relations,omitempty"`
}

// EntityDisclosure describes visibility for a single entity.
type EntityDisclosure struct {
	Level  VisibilityLevel `json:"level"`
	Pieces map[string]bool `json:"pieces,omitempty"` // per-component/knowledge-key visibility
}

// GetEntityLevel returns the visibility level for an entity.
// If not in the map, returns Hidden.
func (ds DisclosureState) GetEntityLevel(id model.EntityID) VisibilityLevel {
	if ed, ok := ds.Entities[id]; ok {
		return ed.Level
	}
	return Hidden
}

// IsFactVisible returns whether a fact has been revealed.
func (ds DisclosureState) IsFactVisible(id model.FactID) bool {
	visible, ok := ds.Facts[id]
	return ok && visible
}

// IsRelationVisible returns whether a relation has been revealed.
func (ds DisclosureState) IsRelationVisible(id model.RelationID) bool {
	visible, ok := ds.Relations[id]
	return ok && visible
}

// IsPieceVisible returns whether a specific knowledge piece of an entity is visible.
// Returns true if the entity is explored AND the piece is not explicitly locked.
func (ds DisclosureState) IsPieceVisible(entityID model.EntityID, piece string) bool {
	ed, ok := ds.Entities[entityID]
	if !ok || ed.Level != Explored {
		return false
	}
	if ed.Pieces == nil {
		return true // no piece-level restrictions
	}
	visible, defined := ed.Pieces[piece]
	if !defined {
		return true // undefined pieces default to visible when explored
	}
	return visible
}
