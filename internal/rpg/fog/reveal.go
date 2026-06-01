package fog

import "github.com/sizolity/worldline/internal/world/model"

// RevealAction describes a single disclosure state change.
type RevealAction struct {
	EntityID   model.EntityID   `json:"entity_id,omitempty"`
	FactID     model.FactID     `json:"fact_id,omitempty"`
	RelationID model.RelationID `json:"relation_id,omitempty"`
	Piece      string           `json:"piece,omitempty"` // unlock specific knowledge piece
	ToLevel    VisibilityLevel  `json:"to_level,omitempty"`
}

// Reveal applies one or more reveal actions to the disclosure state.
// Visibility only increases (hidden→known→explored), never decreases.
func Reveal(state *DisclosureState, actions ...RevealAction) {
	for _, a := range actions {
		if a.EntityID != "" {
			revealEntity(state, a)
		}
		if a.FactID != "" {
			revealFact(state, a.FactID)
		}
		if a.RelationID != "" {
			revealRelation(state, a.RelationID)
		}
	}
}

func revealEntity(state *DisclosureState, a RevealAction) {
	if state.Entities == nil {
		state.Entities = make(map[model.EntityID]EntityDisclosure)
	}

	ed := state.Entities[a.EntityID]

	if a.Piece != "" {
		if ed.Pieces == nil {
			ed.Pieces = make(map[string]bool)
		}
		ed.Pieces[a.Piece] = true
		state.Entities[a.EntityID] = ed
		return
	}

	targetLevel := a.ToLevel
	if targetLevel == "" {
		targetLevel = Explored
	}

	if levelOrd(targetLevel) > levelOrd(ed.Level) {
		ed.Level = targetLevel
	}
	state.Entities[a.EntityID] = ed
}

func revealFact(state *DisclosureState, id model.FactID) {
	if state.Facts == nil {
		state.Facts = make(map[model.FactID]bool)
	}
	state.Facts[id] = true
}

func revealRelation(state *DisclosureState, id model.RelationID) {
	if state.Relations == nil {
		state.Relations = make(map[model.RelationID]bool)
	}
	state.Relations[id] = true
}

func levelOrd(l VisibilityLevel) int {
	switch l {
	case Hidden:
		return 0
	case Known:
		return 1
	case Explored:
		return 2
	default:
		return 0
	}
}
