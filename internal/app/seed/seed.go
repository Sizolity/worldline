package seed

import (
	"context"
)

// DefaultScenarioID is the v1 default mod scenario.
const DefaultScenarioID = "xiyou-changan"

// DefaultStyleID is the v1 default mod style.
const DefaultStyleID = "default"

// Seed writes a world (compiled from the default mod scenario + style)
// plus its starter worldlines into the workspace. It is a thin
// backward-compatible wrapper around SeedFromMod for callers that just
// want "seed the default xiyou demo".
//
// If force is false and the world already exists, it returns an error
// describing the conflict instead of overwriting.
//
// For full control (custom scenario / style / locale / player) use
// SeedFromMod directly.
func Seed(workspace, worldID string, force bool) error {
	_, err := SeedFromMod(context.Background(), SeedRequest{
		Workspace:  workspace,
		WorldID:    worldID,
		ScenarioID: DefaultScenarioID,
		StyleID:    DefaultStyleID,
		Force:      force,
	})
	return err
}
