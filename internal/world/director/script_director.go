package director

import (
	"github.com/sizolity/worldline/internal/world/model"
)

type ScriptDirector struct {
	id     string
	events []model.WorldEvent
}

func NewScriptDirector(id string, events []model.WorldEvent) ScriptDirector {
	cloned := model.CloneEvents(events)
	if cloned == nil {
		cloned = []model.WorldEvent{}
	}
	return ScriptDirector{
		id:     id,
		events: cloned,
	}
}

func (d ScriptDirector) ID() string {
	return d.id
}

func (d ScriptDirector) Propose(_ Context) ([]model.WorldEvent, error) {
	cloned := model.CloneEvents(d.events)
	if cloned == nil {
		cloned = []model.WorldEvent{}
	}
	return cloned, nil
}
