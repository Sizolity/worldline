package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveSaveAndLoadSource(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	doc := SourceDocument{
		ID:       "src_novel",
		Filename: "novel.txt",
		Format:   "txt",
		Text:     "Once upon a time...",
	}
	require.NoError(t, archive.SaveSource(doc))

	loaded, err := archive.LoadSource("src_novel")
	require.NoError(t, err)
	assert.Equal(t, doc.ID, loaded.ID)
	assert.Equal(t, doc.Text, loaded.Text)
}

func TestArchiveLoadSourceNotFound(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	_, err := archive.LoadSource("nonexistent")
	assert.Error(t, err)
}

func TestArchiveSaveAndLoadProvenance(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	provenance := []ProvenanceEntry{
		{WorldID: "char_hero", Kind: "entity", SourceRefs: []string{"ch1_p1"}},
		{WorldID: "fact_origin", Kind: "fact", SourceRefs: []string{"ch2_p3"}},
	}
	require.NoError(t, archive.SaveProvenance("src_novel", provenance))

	loaded, err := archive.LoadProvenance("src_novel")
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "char_hero", loaded[0].WorldID)
	assert.Equal(t, []string{"ch2_p3"}, loaded[1].SourceRefs)
}

func TestArchiveLoadProvenanceNotFound(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	loaded, err := archive.LoadProvenance("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestArchiveSaveProvenanceAppendsHistory(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	first := []ProvenanceEntry{{WorldID: "char_a", Kind: "entity", SourceRefs: []string{"ch1"}}}
	second := []ProvenanceEntry{{WorldID: "char_b", Kind: "entity", SourceRefs: []string{"ch2"}}}

	require.NoError(t, archive.SaveProvenance("src_x", first))
	require.NoError(t, archive.SaveProvenance("src_x", second))

	history, err := archive.LoadProvenanceHistory("src_x")
	require.NoError(t, err)
	require.Len(t, history, 2, "second SaveProvenance must append, not overwrite")
	assert.Equal(t, "char_a", history[0].Entries[0].WorldID)
	assert.Equal(t, "char_b", history[1].Entries[0].WorldID)

	latest, err := archive.LoadProvenance("src_x")
	require.NoError(t, err)
	require.Len(t, latest, 1)
	assert.Equal(t, "char_b", latest[0].WorldID, "LoadProvenance must return the latest record")
}

func TestArchiveLoadProvenanceHistoryReadsLegacyFormat(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	// Simulate a legacy file written by the previous overwrite-only version:
	// a top-level []ProvenanceEntry array.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src_legacy"), 0o755))
	legacy := `[{"world_id":"char_old","kind":"entity","source_refs":["ch1"]}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src_legacy", "provenance.json"), []byte(legacy), 0o644))

	history, err := archive.LoadProvenanceHistory("src_legacy")
	require.NoError(t, err)
	require.Len(t, history, 1, "legacy single-array must surface as one record")
	require.Len(t, history[0].Entries, 1)
	assert.Equal(t, "char_old", history[0].Entries[0].WorldID)
}

func TestArchiveListSources(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	require.NoError(t, archive.SaveSource(SourceDocument{ID: "src_a", Filename: "a.txt", Format: "txt", Text: "A"}))
	require.NoError(t, archive.SaveSource(SourceDocument{ID: "src_b", Filename: "b.md", Format: "md", Text: "B"}))

	ids, err := archive.ListSources()
	require.NoError(t, err)
	assert.Contains(t, ids, "src_a")
	assert.Contains(t, ids, "src_b")
}

func TestArchiveListSourcesEmptyDir(t *testing.T) {
	dir := t.TempDir()
	archive := NewSourceArchive(dir)

	ids, err := archive.ListSources()
	require.NoError(t, err)
	assert.Empty(t, ids)
}
