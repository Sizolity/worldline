package fog

import "github.com/sizolity/worldline/internal/world/model"

// FilterWorld returns a copy of the world with hidden content stripped
// according to the disclosure state.
//
// Filtering rules:
//   - Hidden entities: removed entirely
//   - Known entities: keep ID, Type, Name, Tags; strip Description, Components, State
//   - Explored entities: keep everything, except locked Pieces (those Components stripped)
//   - Facts: included only if explicitly visible in state.Facts
//   - Relations: included only if both endpoints are >= Known AND relation is in state.Relations
//   - Rules: always pass through (L0 bypass)
//   - EventLog/Memory: always pass through (already happened = known)
//   - Threads: always pass through (narrative structure is meta-knowledge)
func FilterWorld(w model.World, state DisclosureState) model.World {
	out := w
	out.Entities = filterEntities(w.Entities, state)
	out.Facts = filterFacts(w.Facts, state)
	out.Relations = filterRelations(w.Relations, state)
	return out
}

func filterEntities(entities map[model.EntityID]model.Entity, state DisclosureState) map[model.EntityID]model.Entity {
	if len(entities) == 0 {
		return nil
	}
	out := make(map[model.EntityID]model.Entity, len(entities))
	for id, e := range entities {
		level := state.GetEntityLevel(id)
		switch level {
		case Hidden:
			continue
		case Known:
			out[id] = model.Entity{
				ID:   e.ID,
				Type: e.Type,
				Name: e.Name,
				Tags: e.Tags,
			}
		case Explored:
			out[id] = filterEntityPieces(e, state)
		}
	}
	return out
}

func filterEntityPieces(e model.Entity, state DisclosureState) model.Entity {
	ed, ok := state.Entities[e.ID]
	if !ok || ed.Pieces == nil {
		return e
	}

	filtered := e
	if e.Components != nil {
		filtered.Components = make(map[string]any, len(e.Components))
		for key, comp := range e.Components {
			if visible, defined := ed.Pieces[key]; defined && !visible {
				continue // locked piece
			}
			filtered.Components[key] = comp
		}
	}
	return filtered
}

func filterFacts(facts []model.Fact, state DisclosureState) []model.Fact {
	if state.Facts == nil {
		return nil
	}
	var out []model.Fact
	for _, f := range facts {
		if state.IsFactVisible(f.ID) {
			out = append(out, f)
		}
	}
	return out
}

func filterRelations(relations []model.Relation, state DisclosureState) []model.Relation {
	if state.Relations == nil {
		return nil
	}
	var out []model.Relation
	for _, r := range relations {
		if !state.IsRelationVisible(r.ID) {
			continue
		}
		srcLevel := state.GetEntityLevel(r.SourceID)
		tgtLevel := state.GetEntityLevel(r.TargetID)
		if srcLevel == Hidden || tgtLevel == Hidden {
			continue
		}
		out = append(out, r)
	}
	return out
}
