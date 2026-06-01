// Package app provides application-level wiring for the RPG engine.
// It assembles the full object graph (LLM client → agents → narrator →
// session) so multiple entry points (CLI, server) can share the same
// construction logic without duplication.
package app

import (
	"context"
	"fmt"

	"github.com/sizolity/worldline/internal/agent/provider/deepseek"
	"github.com/sizolity/worldline/internal/agent/react"
	"github.com/sizolity/worldline/internal/agent/typed"
	"github.com/sizolity/worldline/internal/env"
	"github.com/sizolity/worldline/internal/rpg/intent"
	"github.com/sizolity/worldline/internal/rpg/mod"
	"github.com/sizolity/worldline/internal/rpg/narrator"
	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/rpg/session"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
	"github.com/sizolity/worldline/internal/world/store"
)

// PlayOptions holds the inputs needed to assemble a play runtime.
// CLI and server fill this from their own flag/config sources.
//
// Renamed from PlayConfig in Stage C-1 to disambiguate from the
// per-world PlayConfig sidecar (now also living in this package).
type PlayOptions struct {
	Workspace string
	WorldID   string
	StyleID   string // overrides sidecar if non-empty
	NoStory   bool
	MaxStep   int
}

// PlayRuntime is the assembled object graph ready for the REPL loop or
// an HTTP beat handler.
type PlayRuntime struct {
	Session        *session.Session
	IntentResolver *intent.Resolver
	Player         role.Player
	Style          *mod.Style
	PlayCfg        PlayConfig // resolved config (merged sidecar + overrides)
	Resuming       bool
	ProloguePrompt string
	RecapPrompt    string
}

// NewPlayRuntime assembles the full play object graph: loads the play
// sidecar, resolves style, creates the LLM client, wires all agents,
// and constructs the session. The caller is responsible for the REPL or
// HTTP interaction loop.
func NewPlayRuntime(ctx context.Context, cfg PlayOptions) (*PlayRuntime, error) {
	// 0. Load .env files so downstream lookups (DEEPSEEK_API_KEY,
	// WORLDLINE_MOD_DIR, etc.) resolve before any consumer runs. Must
	// precede mod.LocateRoot below. This is the temporary home for env
	// bootstrap; a dedicated init package will own it once it exists.
	env.LoadIfNeeded(cfg.Workspace)

	// 1. Load and merge sidecar config.
	playCfg, err := resolveSidecar(cfg)
	if err != nil {
		return nil, err
	}

	// Persist the (possibly updated) sidecar so the next resume starts
	// from the same scenario/style. Best-effort; missing sidecar is OK.
	if err := SavePlayConfig(cfg.Workspace, cfg.WorldID, playCfg); err != nil {
		return nil, fmt.Errorf("persist play sidecar: %w", err)
	}

	// 2. Load mod style.
	modRoot, err := mod.LocateRoot()
	if err != nil {
		return nil, err
	}
	style, err := mod.LoadStyle(modRoot, playCfg.StyleID)
	if err != nil {
		return nil, fmt.Errorf("load style: %w", err)
	}

	// 3. Probe the world to decide whether to run the prologue.
	resuming := false
	if preview, loadErr := store.NewFileStore(cfg.Workspace).LoadSnapshot(ctx, cfg.WorldID); loadErr == nil {
		if preview.Clock.Sequence > 1 {
			resuming = true
		}
	}

	// 4. Create LLM client.
	chatModel, err := deepseek.NewChatModel()
	if err != nil {
		return nil, err
	}

	// 5. Assemble agents.
	suggestAgent := typed.NewToolCall[narrator.SuggestParams](chatModel, narrator.SuggestToolName, narrator.SuggestToolDesc)
	gm, err := narrator.New(suggestAgent, narrator.WithStyle(style))
	if err != nil {
		return nil, fmt.Errorf("create narrator: %w", err)
	}

	loreAgent := typed.NewJSONObject[narrator.LoreDraft](chatModel)
	lk := narrator.NewLoreParser(loreAgent, gm)
	beatAgent := react.New(chatModel)

	intentAgent := typed.NewToolCall[intent.Params](chatModel, intent.ResolveToolName, intent.ResolveToolDesc)
	intentResolver, err := intent.NewResolver(intentAgent, style.IntentPersona)
	if err != nil {
		return nil, fmt.Errorf("create intent resolver: %w", err)
	}

	// 6. Construct player and session.
	player := role.Player{
		ID:          playCfg.PlayerID,
		Name:        playCfg.PlayerName,
		CharacterID: worldmodel.EntityID(playCfg.CharacterID),
	}
	sess, err := session.New(session.Config{
		GM:            gm,
		Players:       []role.Player{player},
		WorkspacePath: cfg.Workspace,
		BeatAgent:     beatAgent,
		MaxStep:       cfg.MaxStep,
		StoryEnabled:  !cfg.NoStory,
		Lorekeeper:    lk,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// 7. Resolve prologue / recap prompts with defaults.
	prologuePrompt := style.ProloguePrompt
	if prologuePrompt == "" {
		prologuePrompt = "请以说书人之口起一段定场诗或开篇套话，简述当下场景与玩家此刻处境，最后抛出一个自然的抉择点。"
	}
	recapPrompt := style.RecapPrompt
	if recapPrompt == "" {
		recapPrompt = "上次玩家在此处中断。请阅读最近几条事件，以两三句简短文字提示当前场景。"
	}

	return &PlayRuntime{
		Session:        sess,
		IntentResolver: intentResolver,
		Player:         player,
		Style:          style,
		PlayCfg:        playCfg,
		Resuming:       resuming,
		ProloguePrompt: prologuePrompt,
		RecapPrompt:    recapPrompt,
	}, nil
}

// resolveSidecar loads the play.json sidecar and merges it with CLI/config
// overrides to produce the final PlayConfig. The sidecar is the single
// source of truth for scenario/style/player binding — it is written by
// `rpg-cli seed`, so play requires that seed has already run for the
// world. Missing sidecar is therefore a hard error rather than silently
// falling back to a hardcoded scenario/style pair.
func resolveSidecar(cfg PlayOptions) (PlayConfig, error) {
	sidecar, err := LoadPlayConfig(cfg.Workspace, cfg.WorldID)
	if err != nil {
		return PlayConfig{}, fmt.Errorf("read play sidecar: %w", err)
	}
	if sidecar == nil {
		return PlayConfig{}, fmt.Errorf("play sidecar missing at %s; run `rpg-cli seed` for this world first", PlayConfigPath(cfg.Workspace, cfg.WorldID))
	}
	playCfg := *sidecar
	if cfg.StyleID != "" {
		playCfg.StyleID = cfg.StyleID
	}
	return playCfg, nil
}
