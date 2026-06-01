package store

import (
	"fmt"

	"github.com/sizolity/worldline/internal/world/model"
)

// WorldTemplate provides starter content for a new world.
type WorldTemplate struct {
	Name        string
	Description string
	Canon       model.Canon
	Entities    map[model.EntityID]model.Entity
	Relations   []model.Relation
	Facts       []model.Fact
	Threads     []model.WorldThread
	Rules       []model.Rule
}

// ApplyTemplate creates a World from a template, using the given ID and name.
func ApplyTemplate(tmpl WorldTemplate, worldID, worldName string) (model.World, error) {
	w := model.World{
		ID:          model.WorldID(worldID),
		Name:        worldName,
		Description: tmpl.Description,
		Canon:       tmpl.Canon,
		Entities:    tmpl.Entities,
		Relations:   tmpl.Relations,
		Facts:       tmpl.Facts,
		Threads:     tmpl.Threads,
		Rules:       tmpl.Rules,
	}
	if err := w.Validate(); err != nil {
		return model.World{}, fmt.Errorf("template produced invalid world: %w", err)
	}
	return w, nil
}
