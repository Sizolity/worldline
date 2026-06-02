// Package session orchestrates the RPG beat pipeline using Eino's ReAct agent.
// The GM role (injected via role.GM) controls prompt generation, tool selection,
// and action suggestion. See rpg/narrator/ for concrete GM implementations.
//
// Session itself is pure orchestration — no LLM logic, no prompt construction,
// no effect application. Each beat:
//
//  1. Loads the world (+ disclosure if fog enabled)
//  2. Asks the GM for the disclosed toolset and the system prompt
//  3. Runs the Eino ReAct agent with the resulting tools + prompt
//  4. Applies any pending effects via world/runtime.ApplyEvent
//  5. Runs the Lorekeeper (if configured) to extract structured world
//     knowledge from the narrative and compile it into the snapshot.
//     Failure is non-fatal; surfaces via BeatResult.LoreErr.
//  6. Persists the updated world + disclosure
//  7. Asks the GM to suggest next-step ActionChoices for the PL
package session

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/sizolity/worldline/internal/agent/react"
	"github.com/sizolity/worldline/internal/rpg/fog"
	"github.com/sizolity/worldline/internal/rpg/role"
	"github.com/sizolity/worldline/internal/rpg/story"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
	worldruntime "github.com/sizolity/worldline/internal/world/runtime"
	"github.com/sizolity/worldline/internal/world/store"
)

// Session manages a single RPG game session tied to one world.
type Session struct {
	gm         role.GM
	players    []role.Player
	store      *store.FileStore
	fogStore   *fog.Store
	storyStore *story.Store
	lorekeeper role.Lorekeeper
	runtime    worldruntime.Runtime
	beatAgent  react.Agent
	rng        *rand.Rand
	maxStep    int
	fogEnabled bool
}

// Config holds parameters for creating a new Session.
type Config struct {
	GM            role.GM
	Players       []role.Player
	WorkspacePath string // root for all data; worlds, fog, and worldlines colocated under {WorkspacePath}/worlds/{worldID}/
	BeatAgent     react.Agent
	Rng           *rand.Rand
	MaxStep       int  // max tool-calling iterations per beat (default 10)
	FogEnabled    bool // enable progressive world disclosure (fog of war)
	// StoryEnabled toggles the WorldLine scheduler. When true, the session
	// loads worldlines.json at beat start, ticks them after player effects
	// apply, applies emitted events, and persists updated lines. Default off
	// keeps existing sessions unchanged.
	StoryEnabled bool
	// Lorekeeper is optional. When set, after each beat the lorekeeper
	// extracts an ingest.Draft from the narrative and compiles it into
	// the world snapshot. When nil, the lore pipeline is skipped entirely.
	// Lorekeeper failure never aborts a beat — errors surface via
	// BeatResult.LoreErr.
	Lorekeeper role.Lorekeeper
}

func New(cfg Config) (*Session, error) {
	if cfg.GM == nil {
		return nil, fmt.Errorf("GM is required")
	}
	if cfg.WorkspacePath == "" {
		return nil, fmt.Errorf("workspace path is required")
	}
	if cfg.BeatAgent == nil {
		return nil, fmt.Errorf("beat agent is required")
	}
	rng := cfg.Rng
	if rng == nil {
		rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	maxStep := cfg.MaxStep
	if maxStep <= 0 {
		maxStep = 10
	}
	worldsDir := store.WorldsDir(cfg.WorkspacePath)
	sess := &Session{
		gm:         cfg.GM,
		players:    cfg.Players,
		store:      store.NewFileStore(cfg.WorkspacePath),
		fogStore:   fog.NewStore(worldsDir),
		lorekeeper: cfg.Lorekeeper,
		runtime:    worldruntime.NewRuntime(),
		beatAgent:  cfg.BeatAgent,
		rng:        rng,
		maxStep:    maxStep,
		fogEnabled: cfg.FogEnabled,
	}
	if cfg.StoryEnabled {
		sess.storyStore = story.NewStore(worldsDir)
	}
	return sess, nil
}

// LoadWorld loads a world snapshot by ID.
func (s *Session) LoadWorld(ctx context.Context, worldID string) (worldmodel.World, error) {
	return s.store.LoadSnapshot(ctx, worldID)
}
