package ingest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SourceArchive persists source documents and their provenance alongside world data.
type SourceArchive struct {
	root string
}

// NewSourceArchive creates an archive rooted at the given directory.
// Typically this is <workspace>/worlds/<worldID>/sources/.
func NewSourceArchive(root string) *SourceArchive {
	return &SourceArchive{root: root}
}

// SaveSource persists a SourceDocument to disk.
func (a *SourceArchive) SaveSource(doc SourceDocument) error {
	dir := filepath.Join(a.root, doc.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(dir, "source.json"), doc)
}

// LoadSource loads a previously archived SourceDocument.
func (a *SourceArchive) LoadSource(sourceID string) (SourceDocument, error) {
	var doc SourceDocument
	path := filepath.Join(a.root, sourceID, "source.json")
	if err := readJSONFile(path, &doc); err != nil {
		return SourceDocument{}, fmt.Errorf("load source %q: %w", sourceID, err)
	}
	return doc, nil
}

// ProvenanceRecord is one append-only audit entry: which entries were produced
// by which compile invocation, at what time. The archive stores a history of
// these so that re-ingesting the same source never silently overwrites earlier
// compile audit trails.
type ProvenanceRecord struct {
	RecordedAt time.Time         `json:"recorded_at"`
	Entries    []ProvenanceEntry `json:"entries,omitempty"`
}

// nowUTC is overridable for tests.
var nowUTC = func() time.Time { return time.Now().UTC() }

// SaveProvenance appends a new ProvenanceRecord to the source's history.
// Previous records are never overwritten — every compile pass produces an
// auditable entry. To inspect the full history, use LoadProvenanceHistory;
// LoadProvenance returns only the latest record's entries for convenience.
//
// Empty input still appends a record (the timestamp itself is meaningful:
// "we re-ran compile at T and nothing was produced").
func (a *SourceArchive) SaveProvenance(sourceID string, entries []ProvenanceEntry) error {
	dir := filepath.Join(a.root, sourceID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	history, err := a.LoadProvenanceHistory(sourceID)
	if err != nil {
		return err
	}
	history = append(history, ProvenanceRecord{
		RecordedAt: nowUTC(),
		Entries:    entries,
	})
	return writeJSONFile(filepath.Join(dir, "provenance.json"), history)
}

// LoadProvenance loads the entries from the most recent compile pass.
// Returns nil if no provenance exists.
func (a *SourceArchive) LoadProvenance(sourceID string) ([]ProvenanceEntry, error) {
	history, err := a.LoadProvenanceHistory(sourceID)
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return nil, nil
	}
	return history[len(history)-1].Entries, nil
}

// LoadProvenanceHistory returns the full append-only compile audit trail,
// oldest first. Returns nil if no provenance exists.
//
// For backwards compatibility, also reads the legacy single-array format
// (a top-level []ProvenanceEntry written by older versions) and exposes it
// as a single record with a zero RecordedAt.
func (a *SourceArchive) LoadProvenanceHistory(sourceID string) ([]ProvenanceRecord, error) {
	path := filepath.Join(a.root, sourceID, "provenance.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("load provenance %q: %w", sourceID, err)
	}
	// Discriminate new vs legacy format by peeking at the first element's keys.
	var probe []map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("decode provenance %q: %w", sourceID, err)
	}
	if len(probe) == 0 {
		return nil, nil
	}
	first := probe[0]
	_, hasRecordedAt := first["recorded_at"]
	_, hasEntries := first["entries"]
	if hasRecordedAt || hasEntries {
		var history []ProvenanceRecord
		if err := json.Unmarshal(data, &history); err != nil {
			return nil, fmt.Errorf("decode provenance %q: %w", sourceID, err)
		}
		return history, nil
	}
	// Legacy fallback: top-level []ProvenanceEntry → wrap into one record.
	var legacy []ProvenanceEntry
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("decode provenance %q: %w", sourceID, err)
	}
	return []ProvenanceRecord{{Entries: legacy}}, nil
}

// ListSources returns the IDs of all archived sources.
func (a *SourceArchive) ListSources() ([]string, error) {
	entries, err := os.ReadDir(a.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
