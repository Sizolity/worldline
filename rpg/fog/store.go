package fog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Store handles disclosure state persistence.
// Files are stored at <BaseDir>/<worldID>/disclosure.json.
type Store struct {
	BaseDir string
}

// NewStore creates a store rooted at baseDir.
func NewStore(baseDir string) *Store {
	return &Store{BaseDir: baseDir}
}

func (s *Store) path(worldID string) string {
	return filepath.Join(s.BaseDir, worldID, "disclosure.json")
}

// Load reads the disclosure state for a world.
// Returns an empty state (not an error) if the file doesn't exist yet.
func (s *Store) Load(worldID string) (DisclosureState, error) {
	data, err := os.ReadFile(s.path(worldID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DisclosureState{}, nil
		}
		return DisclosureState{}, fmt.Errorf("read disclosure: %w", err)
	}
	var state DisclosureState
	if err := json.Unmarshal(data, &state); err != nil {
		return DisclosureState{}, fmt.Errorf("parse disclosure: %w", err)
	}
	return state, nil
}

// Save writes the disclosure state for a world, creating directories as needed.
func (s *Store) Save(worldID string, state DisclosureState) error {
	p := s.path(worldID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal disclosure: %w", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("write disclosure: %w", err)
	}
	return nil
}

// Exists checks whether a disclosure file exists for the given world.
func (s *Store) Exists(worldID string) bool {
	_, err := os.Stat(s.path(worldID))
	return err == nil
}
