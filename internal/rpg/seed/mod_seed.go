package seed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sizolity/worldline/internal/rpg/app"
	"github.com/sizolity/worldline/internal/rpg/mod"
	"github.com/sizolity/worldline/internal/rpg/story"
	"github.com/sizolity/worldline/internal/world/store"
)

// SeedRequest carries the parameters for the v1 mod-driven seed flow.
type SeedRequest struct {
	// Workspace is the root directory under which worlds/<worldID>/ is
	// created.
	Workspace string

	// WorldID is the persisted world identifier. May differ from
	// ScenarioID if the caller wants multiple saves of the same scenario.
	// When empty, defaults to ScenarioID.
	WorldID string

	// ScenarioID is the mod/scenarios/<id>/ directory name. Required.
	ScenarioID string

	// StyleID is the mod/styles/<id>/ directory name; used only to
	// validate the style exists and to write into the play.json sidecar.
	// Required.
	StyleID string

	// Force, when true, overwrites an existing world.json. Without it,
	// SeedFromMod refuses to clobber player progress.
	Force bool

	// Locale is the locale stored in the sidecar. Defaults to "zh-CN".
	Locale string

	// PlayerID is the player identifier stored in the sidecar. Defaults
	// to "player-1".
	PlayerID string
}

// SeedResult summarizes what SeedFromMod wrote — used by the CLI to
// produce the "已铺设" success banner without re-loading the mod.
type SeedResult struct {
	Scenario         *mod.Scenario
	Style            *mod.Style
	PlayConfig       app.PlayConfig
	WorldlinesCount  int
	OverwroteExisting bool
}

// SeedFromMod loads a mod scenario + style, compiles them into a
// worldmodel.World plus starter worldlines, and persists everything
// under workspace/worlds/<worldID>/. Writes a play.json sidecar so
// resume flows can recover scenario/style without CLI re-input.
func SeedFromMod(ctx context.Context, req SeedRequest) (*SeedResult, error) {
	if req.Workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	if req.ScenarioID == "" {
		return nil, fmt.Errorf("scenario id is required")
	}
	if req.StyleID == "" {
		return nil, fmt.Errorf("style id is required")
	}
	worldID := req.WorldID
	if worldID == "" {
		worldID = req.ScenarioID
	}
	locale := req.Locale
	if locale == "" {
		locale = "zh-CN"
	}
	playerID := req.PlayerID
	if playerID == "" {
		playerID = "player-1"
	}

	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir workspace: %w", err)
	}

	worldJSON := filepath.Join(req.Workspace, "worlds", worldID, "world.json")
	overwrote := false
	if _, err := os.Stat(worldJSON); err == nil {
		if !req.Force {
			return nil, fmt.Errorf("world already exists at %s (pass force=true to overwrite)", worldJSON)
		}
		overwrote = true
	}

	modRoot, err := mod.LocateRoot()
	if err != nil {
		return nil, err
	}
	scenario, err := mod.LoadScenario(modRoot, req.ScenarioID)
	if err != nil {
		return nil, fmt.Errorf("load scenario: %w", err)
	}
	style, err := mod.LoadStyle(modRoot, req.StyleID)
	if err != nil {
		return nil, fmt.Errorf("load style: %w", err)
	}

	world, err := mod.CompileScenarioToWorld(scenario, worldID)
	if err != nil {
		return nil, fmt.Errorf("compile scenario: %w", err)
	}

	fs := store.NewFileStore(req.Workspace)
	if err := fs.SaveSnapshot(ctx, world); err != nil {
		return nil, fmt.Errorf("save world: %w", err)
	}

	worldsDir := filepath.Join(req.Workspace, "worlds")
	lines := mod.CompileScenarioToWorldLines(scenario)
	if lines == nil {
		lines = []story.WorldLine{}
	}
	st := story.NewStore(worldsDir)
	if err := st.Save(worldID, lines); err != nil {
		return nil, fmt.Errorf("save worldlines: %w", err)
	}

	cfg := app.PlayConfig{
		ScenarioID:  scenario.ID,
		StyleID:     style.ID,
		Locale:      locale,
		PlayerID:    playerID,
		PlayerName:  mod.PlayerCharacterName(scenario),
		CharacterID: mod.PlayerEntityID(scenario),
	}
	if err := app.SavePlayConfig(req.Workspace, worldID, cfg); err != nil {
		return nil, fmt.Errorf("save play sidecar: %w", err)
	}

	return &SeedResult{
		Scenario:          scenario,
		Style:             style,
		PlayConfig:        cfg,
		WorldlinesCount:   len(lines),
		OverwroteExisting: overwrote,
	}, nil
}
