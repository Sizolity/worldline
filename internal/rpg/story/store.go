package story

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Store persists []WorldLine per world. Files live at
// <BaseDir>/<worldID>/worldlines.json, colocated with snapshot.json (FileStore)
// and disclosure.json (fog.Store) so all world-scoped data shares one dir.
type Store struct {
	BaseDir string
}

// NewStore creates a store rooted at baseDir (typically <workspace>/worlds).
func NewStore(baseDir string) *Store {
	return &Store{BaseDir: baseDir}
}

func (s *Store) path(worldID string) string {
	return filepath.Join(s.BaseDir, worldID, "worldlines.json")
}

// Load returns the world lines for worldID. A missing file is not an error;
// the caller gets a nil slice and can proceed (treat as "no lines").
func (s *Store) Load(worldID string) ([]WorldLine, error) {
	data, err := os.ReadFile(s.path(worldID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read worldlines: %w", err)
	}
	var lines []WorldLine
	if err := json.Unmarshal(data, &lines); err != nil {
		return nil, fmt.Errorf("parse worldlines: %w", err)
	}
	return lines, nil
}

// Save writes the world lines for worldID, creating directories as needed.
func (s *Store) Save(worldID string, lines []WorldLine) error {
	p := s.path(worldID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	data, err := json.MarshalIndent(lines, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal worldlines: %w", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("write worldlines: %w", err)
	}
	return nil
}

// Exists reports whether a worldlines file is on disk for worldID.
func (s *Store) Exists(worldID string) bool {
	_, err := os.Stat(s.path(worldID))
	return err == nil
}
