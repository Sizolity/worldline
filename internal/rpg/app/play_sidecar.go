package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// PlayConfig is the persisted per-world play sidecar. It records the
// scenario/style pair the world was seeded against plus the player
// identity, so resuming a saved game does not require re-passing CLI
// flags. Lives at worlds/<worldID>/play.json next to world.json.
//
// Field set is deliberately tight per directive 2.2 — no rule/dice/
// numeric config is allowed here, only orchestration identifiers.
type PlayConfig struct {
	ScenarioID  string `json:"scenario_id"`
	StyleID     string `json:"style_id"`
	Locale      string `json:"locale"`
	PlayerID    string `json:"player_id"`
	PlayerName  string `json:"player_name,omitempty"`
	CharacterID string `json:"character_id"`
}

// PlayConfigPath returns the canonical sidecar path for a given workspace
// and world ID. The file may not exist; callers should treat
// os.IsNotExist as a clean "no sidecar" signal.
func PlayConfigPath(workspace, worldID string) string {
	return filepath.Join(workspace, "worlds", worldID, "play.json")
}

// LoadPlayConfig reads the sidecar at workspace/worlds/<worldID>/play.json.
// Returns (nil, nil) when the file does not exist — callers may then
// fall back to CLI flag defaults.
func LoadPlayConfig(workspace, worldID string) (*PlayConfig, error) {
	path := PlayConfigPath(workspace, worldID)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read play sidecar: %w", err)
	}
	var cfg PlayConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse play sidecar: %w", err)
	}
	return &cfg, nil
}

// SavePlayConfig writes the sidecar, creating directories as needed.
func SavePlayConfig(workspace, worldID string, cfg PlayConfig) error {
	path := PlayConfigPath(workspace, worldID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create play sidecar dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal play sidecar: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write play sidecar: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename play sidecar: %w", err)
	}
	return nil
}
