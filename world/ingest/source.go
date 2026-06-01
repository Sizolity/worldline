// Package ingest provides product-neutral infrastructure for loading external
// narrative sources into structured world data. It defines schemas for source
// documents, extraction drafts, and the Parser / ChunkParser / Chunker /
// AliasResolver interfaces that external implementations (e.g. LLM extractors,
// markdown / PDF splitters, embedding-based aliasers) can satisfy.
//
// The framework never ships concrete LLM or format-specific implementations.
// Format-specific text splitting lives in sibling utility packages such as
// internal/textchunk; ingest only defines the Chunker interface and a trivial
// WholeDocumentChunker fallback.
package ingest

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SourceDocument holds the raw content of an ingested file along with metadata.
//
// Kind is an open-ended hint for the parser, set by the caller (CLI / product
// layer). The ingest framework does not enumerate or validate Kind values —
// products decide their own taxonomy (e.g. "novel", "script", "wiki",
// "setting-bible"). Parsers may switch strategies based on Kind, or ignore it.
type SourceDocument struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Format   string `json:"format"`
	Kind     string `json:"kind,omitempty"`
	Text     string `json:"text"`
}

// SourceChunk is a stable slice of a source document with structural hints.
// SourceKind mirrors SourceDocument.Kind so a ChunkParser can vary strategy
// per chunk without needing the parent document.
type SourceChunk struct {
	ID         string `json:"id"`
	SourceID   string `json:"source_id"`
	SourceKind string `json:"source_kind,omitempty"`
	Index      int    `json:"index"`
	Heading    string `json:"heading,omitempty"`
	Text       string `json:"text"`
}

var supportedFormats = map[string]bool{
	"txt": true,
	"md":  true,
}

// LoadSource reads a file from disk and produces a SourceDocument.
// Only .txt and .md formats are supported.
//
// The generated ID is content-addressable: src_<basename>_<sha1[:6]>.
// This guarantees:
//
//   - same file reloaded → same ID (idempotent for archive dedup);
//   - same filename across directories with different content → different IDs;
//   - same content from different paths → same ID (intentional dedup).
//
// The basename component is kept for human readability; the hash component
// is what enforces uniqueness.
func LoadSource(path string) (SourceDocument, error) {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if !supportedFormats[ext] {
		return SourceDocument{}, fmt.Errorf("unsupported source format %q", ext)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SourceDocument{}, err
	}
	filename := filepath.Base(path)
	base := sanitizeBasename(strings.TrimSuffix(filename, filepath.Ext(filename)))
	sum := sha1.Sum(data)
	hash := hex.EncodeToString(sum[:])[:6]
	id := fmt.Sprintf("src_%s_%s", base, hash)
	return SourceDocument{
		ID:       id,
		Filename: filename,
		Format:   ext,
		Text:     string(data),
	}, nil
}

// sanitizeBasename keeps only [A-Za-z0-9_-]; collapses runs of disallowed
// characters into a single underscore so IDs stay safeIDPattern-compatible.
func sanitizeBasename(name string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := b.String()
	if out == "" || out == "_" {
		return "file"
	}
	return strings.Trim(out, "_-")
}

// Chunker is the contract for splitting a SourceDocument into SourceChunks.
//
// The ingest package intentionally does NOT ship format-specific implementations:
// markdown headings, paragraph heuristics, fixed-window splitting, sentence
// boundaries, PDF page splitting and so on are concerns of utility packages or
// product-layer code. The framework only specifies the interface and provides
// a trivial fallback (WholeDocumentChunker).
//
// Implementations must:
//   - return nil/empty when doc.Text is empty;
//   - propagate doc.ID, doc.Kind into each chunk (SourceID, SourceKind);
//   - assign stable, unique chunk IDs (typically "<doc.ID>_chunk_<i>");
//   - set Index monotonically from 0.
type Chunker interface {
	Chunk(doc SourceDocument) []SourceChunk
}

// WholeDocumentChunker treats the full document as one chunk. Useful as a
// default when the caller does not need granular splitting (e.g. when the
// upstream Parser handles whole documents and chunks are only informational).
type WholeDocumentChunker struct{}

// Chunk implements Chunker.
func (WholeDocumentChunker) Chunk(doc SourceDocument) []SourceChunk {
	text := strings.TrimSpace(doc.Text)
	if text == "" {
		return nil
	}
	return []SourceChunk{{
		ID:         fmt.Sprintf("%s_chunk_0", doc.ID),
		SourceID:   doc.ID,
		SourceKind: doc.Kind,
		Index:      0,
		Text:       text,
	}}
}
