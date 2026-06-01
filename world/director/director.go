// Package director contains world event proposal sources.
package director

import (
	"context"

	"github.com/sizolity/worldline/world/model"
)

type Director interface {
	ID() string
	Propose(ctx Context) ([]model.WorldEvent, error)
}

type Context struct {
	Ctx   context.Context
	World model.World
}
