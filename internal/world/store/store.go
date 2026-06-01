package store

import (
	"context"

	"github.com/sizolity/worldline/internal/world/model"
)

type Store interface {
	SaveWorld(context.Context, model.World) error
	LoadWorld(context.Context, string) (model.World, error)
	SaveSnapshot(context.Context, model.World) error
	LoadSnapshot(context.Context, string) (model.World, error)
	SaveEntity(context.Context, string, model.Entity) error
	LoadEntity(context.Context, string, string) (model.Entity, error)
	SaveRelations(context.Context, string, []model.Relation) error
	LoadRelations(context.Context, string) ([]model.Relation, error)
	SaveFacts(context.Context, string, []model.Fact) error
	LoadFacts(context.Context, string) ([]model.Fact, error)
	AppendEvent(context.Context, string, model.WorldEvent) error
	ListEvents(context.Context, string) ([]model.WorldEvent, error)
	SaveMemories(context.Context, string, []model.MemoryRecord) error
	LoadMemories(context.Context, string) ([]model.MemoryRecord, error)
	SaveThreads(context.Context, string, []model.WorldThread) error
	LoadThreads(context.Context, string) ([]model.WorldThread, error)
}
