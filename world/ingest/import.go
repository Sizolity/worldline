package ingest

import (
	"context"
	"fmt"
	"strings"

	"github.com/sizolity/worldline/world/model"
)

// ImportFileResult is the full result of ImportFile / ImportFileChunked.
//
// Validation is non-empty even on success; callers should inspect Warnings
// to surface advisory issues (e.g. confidence out of range, draft-internal
// dangling refs). Errors are non-empty only when ValidationError is also
// returned and World/CompileReport are zero values.
type ImportFileResult struct {
	SourceDocument SourceDocument
	Chunks         []SourceChunk
	Validation     ValidationReport
	CompileReport  CompileReport
	World          model.World
}

// ValidationError wraps a ValidationReport with errors into an error.
type ValidationError struct {
	Report ValidationReport
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("draft validation failed: %s", strings.Join(e.Report.Errors, "; "))
}

// ImportFile orchestrates the full ingestion pipeline using a whole-document
// Parser: load source, chunk (for informational output only), parse, validate, compile.
//
// chunker may be nil; WholeDocumentChunker is used as the fallback. Since
// the whole-document Parser does not consume chunks, the chunks here are
// purely informational (echoed back in ImportFileResult.Chunks for auditing).
//
// Does NOT persist the world; the caller decides whether to save.
func ImportFile(ctx context.Context, world model.World, path string, parser Parser, chunker Chunker, opts CompileOptions) (ImportFileResult, error) {
	doc, err := LoadSource(path)
	if err != nil {
		return ImportFileResult{}, err
	}
	if chunker == nil {
		chunker = WholeDocumentChunker{}
	}
	chunks := chunker.Chunk(doc)

	draft, err := parser.Parse(ctx, doc)
	if err != nil {
		return ImportFileResult{}, err
	}
	return finishImport(world, doc, chunks, draft, opts)
}

// ImportFileChunked orchestrates the full ingestion pipeline using a per-chunk
// ChunkParser: load source, chunk via the supplied Chunker, parse each chunk,
// merge, validate, compile.
//
// chunker is required and must not be nil: chunked parsing without a real
// splitter degenerates to whole-document parsing and the caller should use
// ImportFile instead. Pass textchunk.Basic() (or a custom Chunker) explicitly.
func ImportFileChunked(ctx context.Context, world model.World, path string, cp ChunkParser, chunker Chunker, opts CompileOptions) (ImportFileResult, error) {
	if chunker == nil {
		return ImportFileResult{}, fmt.Errorf("ImportFileChunked requires a non-nil Chunker")
	}
	doc, err := LoadSource(path)
	if err != nil {
		return ImportFileResult{}, err
	}
	chunks := chunker.Chunk(doc)

	draft, err := ParseChunks(ctx, cp, chunks)
	if err != nil {
		return ImportFileResult{}, err
	}
	return finishImport(world, doc, chunks, draft, opts)
}

func finishImport(world model.World, doc SourceDocument, chunks []SourceChunk, draft Draft, opts CompileOptions) (ImportFileResult, error) {
	vr := ValidateDraft(draft)
	if len(vr.Errors) > 0 {
		return ImportFileResult{}, &ValidationError{Report: vr}
	}

	compiled, report, err := CompileDraft(world, draft, opts)
	if err != nil {
		return ImportFileResult{}, err
	}

	return ImportFileResult{
		SourceDocument: doc,
		Chunks:         chunks,
		Validation:     vr,
		CompileReport:  report,
		World:          compiled,
	}, nil
}
