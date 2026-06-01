package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- WholeDocumentChunker (the only chunker shipped by ingest) ---

func TestWholeDocumentChunkerEmitsSingleChunk(t *testing.T) {
	doc := SourceDocument{ID: "src_x", Format: "md", Kind: "novel", Text: "# C1\n\nbody"}
	chunks := WholeDocumentChunker{}.Chunk(doc)
	require.Len(t, chunks, 1)
	assert.Equal(t, "src_x_chunk_0", chunks[0].ID)
	assert.Equal(t, "novel", chunks[0].SourceKind)
}

func TestWholeDocumentChunkerEmptyDoc(t *testing.T) {
	chunks := WholeDocumentChunker{}.Chunk(SourceDocument{ID: "src_x", Text: ""})
	assert.Empty(t, chunks)
}

// --- Source loading ---

func TestLoadSourceTxt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "story.txt")
	content := "Chapter 1\n\nThe hero arrived at the village.\n\nChapter 2\n\nA storm gathered over the mountains."
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	doc, err := LoadSource(path)
	require.NoError(t, err)
	assert.Equal(t, "story.txt", doc.Filename)
	assert.Equal(t, "txt", doc.Format)
	assert.Equal(t, content, doc.Text)
	assert.NotEmpty(t, doc.ID)
}

func TestLoadSourceMd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "world.md")
	content := "# Chapter 1\n\nThe hero arrived.\n\n# Chapter 2\n\nA storm gathered."
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	doc, err := LoadSource(path)
	require.NoError(t, err)
	assert.Equal(t, "world.md", doc.Filename)
	assert.Equal(t, "md", doc.Format)
	assert.Equal(t, content, doc.Text)
}

func TestLoadSourceUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.pdf")
	require.NoError(t, os.WriteFile(path, []byte("pdf data"), 0o644))

	_, err := LoadSource(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestLoadSourceNotFound(t *testing.T) {
	_, err := LoadSource("/nonexistent/path.txt")
	assert.Error(t, err)
}

// --- Parser interface with fake ---

type fakeParser struct {
	draft Draft
	err   error
}

func (f *fakeParser) Parse(_ context.Context, _ SourceDocument) (Draft, error) {
	return f.draft, f.err
}

func TestParserInterfaceFake(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "Kael", Confidence: 0.9, SourceRefs: []string{"ch1_p1"}},
		},
	}
	p := &fakeParser{draft: draft}
	doc := SourceDocument{ID: "src_1", Format: "txt", Text: "some text"}
	result, err := p.Parse(context.Background(), doc)
	require.NoError(t, err)
	require.Len(t, result.Entities, 1)
	assert.Equal(t, "char_hero", result.Entities[0].ID)
}

// --- ChunkParser interface with fake ---

type fakeChunkParser struct {
	drafts map[string]Draft
}

func (f *fakeChunkParser) ParseChunk(_ context.Context, chunk SourceChunk) (Draft, error) {
	if d, ok := f.drafts[chunk.ID]; ok {
		return d, nil
	}
	return Draft{}, nil
}

func TestChunkParserInterface(t *testing.T) {
	cp := &fakeChunkParser{
		drafts: map[string]Draft{
			"src_1_chunk_0": {Entities: []DraftEntity{{ID: "char_hero", Type: "character", Name: "Kael", SourceRefs: []string{"src_1_chunk_0"}}}},
			"src_1_chunk_1": {Entities: []DraftEntity{{ID: "char_sage", Type: "character", Name: "Mirael", SourceRefs: []string{"src_1_chunk_1"}}}},
		},
	}
	chunks := []SourceChunk{
		{ID: "src_1_chunk_0", SourceID: "src_1", Text: "Chapter 1"},
		{ID: "src_1_chunk_1", SourceID: "src_1", Text: "Chapter 2"},
	}
	draft, err := ParseChunks(context.Background(), cp, chunks)
	require.NoError(t, err)
	assert.Len(t, draft.Entities, 2)
}

func TestChunkParserMergesDuplicateEntities(t *testing.T) {
	cp := &fakeChunkParser{
		drafts: map[string]Draft{
			"c0": {Entities: []DraftEntity{
				{ID: "char_hero", Type: "character", Name: "Kael", Confidence: 0.8, SourceRefs: []string{"c0"}},
			}},
			"c1": {Entities: []DraftEntity{
				{ID: "char_hero", Type: "character", Name: "Kael the Brave", Confidence: 0.95, SourceRefs: []string{"c1"}},
			}},
		},
	}
	chunks := []SourceChunk{
		{ID: "c0", SourceID: "src_1", Text: "part 1"},
		{ID: "c1", SourceID: "src_1", Text: "part 2"},
	}
	draft, err := ParseChunks(context.Background(), cp, chunks)
	require.NoError(t, err)
	require.Len(t, draft.Entities, 1)
	assert.Equal(t, "Kael the Brave", draft.Entities[0].Name)
	assert.InDelta(t, 0.95, draft.Entities[0].Confidence, 0.001)
	assert.Equal(t, []string{"c0", "c1"}, draft.Entities[0].SourceRefs)
}

// --- MergeDrafts ---

func TestMergeDraftsDeduplicatesEntities(t *testing.T) {
	a := Draft{Entities: []DraftEntity{
		{ID: "char_a", Type: "character", Name: "A", Confidence: 0.7, SourceRefs: []string{"c0"}},
	}}
	b := Draft{Entities: []DraftEntity{
		{ID: "char_a", Type: "character", Name: "A Updated", Confidence: 0.9, SourceRefs: []string{"c1"}},
		{ID: "char_b", Type: "character", Name: "B", SourceRefs: []string{"c1"}},
	}}
	merged := MergeDrafts(a, b)
	require.Len(t, merged.Entities, 2)

	var charA DraftEntity
	for _, e := range merged.Entities {
		if e.ID == "char_a" {
			charA = e
		}
	}
	assert.Equal(t, "A Updated", charA.Name)
	assert.InDelta(t, 0.9, charA.Confidence, 0.001)
	assert.Equal(t, []string{"c0", "c1"}, charA.SourceRefs)
}

func TestMergeDraftsDeduplicatesFacts(t *testing.T) {
	a := Draft{Facts: []DraftFact{
		{ID: "fact_1", SubjectID: "char_a", Predicate: "status", Value: "alive", Confidence: 0.6},
	}}
	b := Draft{Facts: []DraftFact{
		{ID: "fact_1", SubjectID: "char_a", Predicate: "status", Value: "dead", Confidence: 0.9},
	}}
	merged := MergeDrafts(a, b)
	require.Len(t, merged.Facts, 1)
	assert.Equal(t, "dead", merged.Facts[0].Value)
}

func TestMergeDraftsMergesCanon(t *testing.T) {
	a := Draft{Canon: &DraftCanon{Genre: []string{"fantasy"}}}
	b := Draft{Canon: &DraftCanon{Genre: []string{"adventure"}, Premise: "New premise"}}
	merged := MergeDrafts(a, b)
	require.NotNil(t, merged.Canon)
	assert.Equal(t, []string{"fantasy", "adventure"}, merged.Canon.Genre)
	assert.Equal(t, "New premise", merged.Canon.Premise)
}

// --- MergeDrafts: high-conf empty must not overwrite low-conf non-empty ---

func TestMergeDraftsPrefersNonEmptyForEntityScalars(t *testing.T) {
	low := Draft{Entities: []DraftEntity{{
		ID: "char_a", Type: "character", Name: "Kael",
		Description: "A wanderer.", Confidence: 0.5,
	}}}
	high := Draft{Entities: []DraftEntity{{
		ID: "char_a", Type: "", Name: "", Description: "", Confidence: 0.9,
	}}}
	merged := MergeDrafts(low, high)
	require.Len(t, merged.Entities, 1)
	got := merged.Entities[0]
	assert.Equal(t, "character", got.Type, "empty Type from high conf must not wipe Type from low conf")
	assert.Equal(t, "Kael", got.Name, "empty Name from high conf must not wipe Name from low conf")
	assert.Equal(t, "A wanderer.", got.Description)
	assert.InDelta(t, 0.9, got.Confidence, 0.001, "high conf still wins the Confidence value itself")
}

func TestMergeDraftsPrefersNonEmptyForRelationScalars(t *testing.T) {
	low := Draft{Relations: []DraftRelation{{
		ID: "rel_1", Type: "knows", SourceID: "char_a", TargetID: "char_b", Confidence: 0.5,
	}}}
	high := Draft{Relations: []DraftRelation{{
		ID: "rel_1", Type: "", SourceID: "", TargetID: "", Confidence: 0.9,
	}}}
	merged := MergeDrafts(low, high)
	require.Len(t, merged.Relations, 1)
	got := merged.Relations[0]
	assert.Equal(t, "knows", got.Type)
	assert.Equal(t, "char_a", got.SourceID)
	assert.Equal(t, "char_b", got.TargetID)
}

func TestMergeDraftsPrefersNonZeroForThreadScalars(t *testing.T) {
	low := Draft{Threads: []DraftThread{{
		ID: "th_1", Kind: "quest", Title: "Q", Status: "open",
		Priority: 0.7, Tension: 0.4, Confidence: 0.5,
		ParticipantIDs: []string{"char_a"}, LocationID: "loc_x",
	}}}
	high := Draft{Threads: []DraftThread{{
		ID: "th_1", Kind: "quest", Title: "Q", Confidence: 0.9,
		ParticipantIDs: []string{"char_b"},
	}}}
	merged := MergeDrafts(low, high)
	require.Len(t, merged.Threads, 1)
	got := merged.Threads[0]
	assert.InDelta(t, 0.7, got.Priority, 0.001, "zero Priority from high must not wipe 0.7 from low")
	assert.InDelta(t, 0.4, got.Tension, 0.001)
	assert.Equal(t, "open", got.Status, "empty Status from high must not wipe 'open' from low")
	assert.Equal(t, "loc_x", got.LocationID, "empty LocationID from high must not wipe low's value")
	assert.ElementsMatch(t, []string{"char_a", "char_b"}, got.ParticipantIDs, "participants must union")
}

func TestMergeDraftsUnionsMemorySubjectsAndEvents(t *testing.T) {
	low := Draft{Memories: []DraftMemory{{
		ID: "mem_1", OwnerKind: "character", OwnerID: "char_a",
		Content: "saw fire", SubjectIDs: []string{"char_b"}, EventIDs: []string{"evt_1"},
		Confidence: 0.5,
	}}}
	high := Draft{Memories: []DraftMemory{{
		ID: "mem_1", OwnerKind: "", OwnerID: "",
		SubjectIDs: []string{"char_c"}, EventIDs: []string{"evt_2"},
		Confidence: 0.9,
	}}}
	merged := MergeDrafts(low, high)
	require.Len(t, merged.Memories, 1)
	got := merged.Memories[0]
	assert.Equal(t, "character", got.OwnerKind)
	assert.Equal(t, "char_a", got.OwnerID)
	assert.Equal(t, "saw fire", got.Content)
	assert.ElementsMatch(t, []string{"char_b", "char_c"}, got.SubjectIDs)
	assert.ElementsMatch(t, []string{"evt_1", "evt_2"}, got.EventIDs)
}

// --- Thread / Memory entity references propagate through compile ---

func TestCompileDraftPopulatesThreadParticipantsAndLocation(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{
		"char_a": {ID: "char_a", Type: "character", Name: "A"},
		"loc_x":  {ID: "loc_x", Type: "location", Name: "X"},
	}}
	draft := Draft{Threads: []DraftThread{{
		ID: "th_1", Kind: "quest", Title: "Q", Status: "open",
		ParticipantIDs: []string{"char_a"}, LocationID: "loc_x",
	}}}
	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	require.Equal(t, 0, report.Rejected)
	require.Len(t, result.Threads, 1)
	assert.Equal(t, []model.EntityID{"char_a"}, result.Threads[0].ParticipantIDs)
	assert.Equal(t, model.EntityID("loc_x"), result.Threads[0].LocationID)
}

func TestCompileDraftPopulatesMemorySubjectsAndEvents(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{
		"char_a": {ID: "char_a", Type: "character", Name: "A"},
	}}
	draft := Draft{Memories: []DraftMemory{{
		ID: "mem_1", OwnerKind: "character", OwnerID: "char_a",
		Content: "saw fire",
		SubjectIDs: []string{"char_a"}, EventIDs: []string{"evt_1"},
	}}}
	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	require.Equal(t, 0, report.Rejected, "should not be rejected")
	require.Len(t, result.Memory, 1)
	assert.Equal(t, []model.EntityID{"char_a"}, result.Memory[0].SubjectIDs)
	assert.Equal(t, []model.EventID{"evt_1"}, result.Memory[0].EventIDs)
}

func TestCompileDraftReplaceUnionsThreadParticipantsAndMemorySubjects(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{
		"char_a": {ID: "char_a", Type: "character", Name: "A"},
		"char_b": {ID: "char_b", Type: "character", Name: "B"},
	}, Threads: []model.WorldThread{{
		ID: "th_1", Kind: "quest", Title: "Q", Status: "open",
		ParticipantIDs: []model.EntityID{"char_a"},
		LocationID:     "loc_x",
	}}, Memory: []model.MemoryRecord{{
		ID: "mem_1", Owner: model.MemoryOwner{Kind: "character", ID: "char_a"},
		Content:    "history",
		SubjectIDs: []model.EntityID{"char_a"},
		EventIDs:   []model.EventID{"evt_1"},
	}}}
	draft := Draft{
		Threads: []DraftThread{{
			ID: "th_1", Kind: "quest", Title: "Q updated", Status: "open",
			ParticipantIDs: []string{"char_b"},
		}},
		Memories: []DraftMemory{{
			ID: "mem_1", OwnerKind: "character", OwnerID: "char_a",
			Content: "history updated", SubjectIDs: []string{"char_b"}, EventIDs: []string{"evt_2"},
		}},
	}
	result, _, err := CompileDraft(world, draft, CompileOptions{ConflictPolicy: ConflictPolicyReplace})
	require.NoError(t, err)
	require.Len(t, result.Threads, 1)
	assert.ElementsMatch(t, []model.EntityID{"char_a", "char_b"}, result.Threads[0].ParticipantIDs,
		"replace must union participants, not replace them")
	assert.Equal(t, model.EntityID("loc_x"), result.Threads[0].LocationID,
		"empty LocationID in draft must preserve existing")
	require.Len(t, result.Memory, 1)
	assert.ElementsMatch(t, []model.EntityID{"char_a", "char_b"}, result.Memory[0].SubjectIDs)
	assert.ElementsMatch(t, []model.EventID{"evt_1", "evt_2"}, result.Memory[0].EventIDs)
}

func TestValidateDraftRejectsMalformedThreadAndMemoryIDs(t *testing.T) {
	draft := Draft{
		Threads: []DraftThread{{
			ID: "th_1", Kind: "quest", Title: "Q", Status: "open",
			ParticipantIDs: []string{""},
		}},
		Memories: []DraftMemory{{
			ID: "mem_1", OwnerKind: "character", OwnerID: "char_a", Content: "x",
			SubjectIDs: []string{"with whitespace"},
			EventIDs:   []string{""},
		}},
	}
	report := ValidateDraft(draft)
	require.NotEmpty(t, report.Errors)
	joined := strings.Join(report.Errors, "|")
	assert.Contains(t, joined, "participant_ids[0] must not be empty")
	assert.Contains(t, joined, "subject_ids[0]")
	assert.Contains(t, joined, "event_ids[0] must not be empty")
}

// --- BatchAliasResolver fast path ---

type batchResolver struct {
	mapping     map[string]string
	batchCalls  int
	singleCalls int
}

func (b *batchResolver) Resolve(_ context.Context, e DraftEntity, _ model.World) (string, error) {
	b.singleCalls++
	if c, ok := b.mapping[e.ID]; ok {
		return c, nil
	}
	return "", nil
}

func (b *batchResolver) ResolveBatch(_ context.Context, entities []DraftEntity, _ model.World) (map[string]string, error) {
	b.batchCalls++
	out := map[string]string{}
	for _, e := range entities {
		if c, ok := b.mapping[e.ID]; ok {
			out[e.ID] = c
		}
	}
	return out, nil
}

func TestBatchAliasResolverIsUsedWhenAvailable(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{
		"char_kael": {ID: "char_kael", Type: "character", Name: "Kael"},
	}}
	draft := Draft{Entities: []DraftEntity{
		{ID: "char_kael_alt", Type: "character", Name: "Kael Alt"},
		{ID: "char_other", Type: "character", Name: "Other"},
	}}
	res := &batchResolver{mapping: map[string]string{"char_kael_alt": "char_kael"}}
	result, report, err := CompileDraft(world, draft, CompileOptions{
		Resolver:       res,
		ConflictPolicy: ConflictPolicyReplace,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.batchCalls, "ResolveBatch must be called exactly once")
	assert.Equal(t, 0, res.singleCalls, "Resolve must not be called when batch path is available")
	assert.Equal(t, "char_kael", report.Aliases["char_kael_alt"])
	assert.NotContains(t, result.Entities, model.EntityID("char_kael_alt"))
	assert.Contains(t, result.Entities, model.EntityID("char_other"))
}

func TestBatchResolverIgnoresMappingsForUnseenIDs(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{
		"char_a": {ID: "char_a", Type: "character", Name: "A"},
	}}
	draft := Draft{Entities: []DraftEntity{{ID: "char_a", Type: "character", Name: "A"}}}
	res := &batchResolver{mapping: map[string]string{
		"char_a":       "",
		"char_unknown": "char_a",
	}}
	_, report, err := CompileDraft(world, draft, CompileOptions{Resolver: res})
	require.NoError(t, err)
	assert.NotContains(t, report.Aliases, "char_unknown",
		"resolver must not be allowed to inject mappings for IDs the compiler never asked about")
}

// --- Confidence filter ---

func TestCompileDraftMinConfidenceFilters(t *testing.T) {
	world := model.World{ID: "world_test", Name: "Test", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_high", Type: "character", Name: "High", Confidence: 0.9},
			{ID: "char_low", Type: "character", Name: "Low", Confidence: 0.3},
			{ID: "char_zero", Type: "character", Name: "Zero", Confidence: 0},
		},
		Facts: []DraftFact{
			{ID: "fact_high", SubjectID: "char_high", Predicate: "ok", Value: "yes", Confidence: 0.8},
			{ID: "fact_low", SubjectID: "char_high", Predicate: "maybe", Value: "no", Confidence: 0.2},
		},
	}
	result, report, err := CompileDraft(world, draft, CompileOptions{MinConfidence: 0.5})
	require.NoError(t, err)
	assert.Equal(t, 2, report.Inserted, "1 entity + 1 fact survives the filter")
	assert.Equal(t, 3, report.Filtered)
	assert.Contains(t, result.Entities, model.EntityID("char_high"))
	assert.NotContains(t, result.Entities, model.EntityID("char_low"))
	assert.NotContains(t, result.Entities, model.EntityID("char_zero"))
	assert.Len(t, result.Facts, 1)
}

func TestCompileDraftMinConfidenceZeroNoFilter(t *testing.T) {
	world := model.World{ID: "world_test", Name: "Test", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_a", Type: "character", Name: "A", Confidence: 0.1},
		},
	}
	result, _, err := CompileDraft(world, draft, CompileOptions{MinConfidence: 0})
	require.NoError(t, err)
	assert.Contains(t, result.Entities, model.EntityID("char_a"))
}

// --- Draft validation ---

func TestValidateDraftValid(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "Kael"},
		},
		Facts: []DraftFact{
			{ID: "fact_origin", SubjectID: "char_hero", Predicate: "origin", Value: "unknown"},
		},
	}
	report := ValidateDraft(draft)
	assert.Empty(t, report.Errors)
}

func TestValidateDraftInvalidID(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "", Type: "character", Name: "Nobody"},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

func TestValidateDraftMissingEntityName(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_x", Type: "character", Name: ""},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

func TestValidateDraftDanglingFactRef(t *testing.T) {
	draft := Draft{
		Facts: []DraftFact{
			{ID: "fact_1", SubjectID: "nonexistent_entity", Predicate: "status", Value: "alive"},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Warnings)
}

func TestValidateDraftDuplicateIDs(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_a", Type: "character", Name: "A"},
			{ID: "char_a", Type: "character", Name: "B"},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

// --- Strengthened validation ---

func TestValidateDraftConfidenceOutOfRange(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_a", Type: "character", Name: "A", Confidence: 1.5},
			{ID: "char_b", Type: "character", Name: "B", Confidence: -0.1},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Warnings, "out-of-range confidence should warn")
}

func TestValidateDraftThreadPriorityRange(t *testing.T) {
	draft := Draft{
		Threads: []DraftThread{
			{ID: "t1", Kind: "quest", Title: "Q", Status: "open", Priority: 2.0},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors, "priority > 1 should error")
}

func TestValidateDraftRelationEmptyType(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "a", Type: "character", Name: "A"},
			{ID: "b", Type: "character", Name: "B"},
		},
		Relations: []DraftRelation{
			{ID: "rel_1", Type: "", SourceID: "a", TargetID: "b"},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

func TestValidateDraftRelationDanglingRefWarning(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "a", Type: "character", Name: "A"},
		},
		Relations: []DraftRelation{
			{ID: "rel_1", Type: "knows", SourceID: "a", TargetID: "ghost"},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Warnings, "draft-internal dangling ref should warn at validate time")
}

func TestValidateDraftMemoryRequiresContentOrSummary(t *testing.T) {
	draft := Draft{
		Memories: []DraftMemory{
			{ID: "m1", OwnerKind: "world", Content: "", Summary: ""},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

func TestValidateDraftMemoryOwnerIDRequiredForNonWorld(t *testing.T) {
	draft := Draft{
		Memories: []DraftMemory{
			{ID: "m1", OwnerKind: "character", OwnerID: "", Content: "x"},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

// --- Compile draft into world ---

func TestCompileDraftNewWorld(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	draft := Draft{
		Canon: &DraftCanon{
			Genre: []string{"fantasy"},
			Tone:  []string{"epic"},
		},
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "Kael", Description: "A wanderer."},
			{ID: "loc_village", Type: "location", Name: "Thornhaven", Description: "A quiet village."},
		},
		Relations: []DraftRelation{
			{ID: "rel_lives", Type: "lives_in", SourceID: "char_hero", TargetID: "loc_village"},
		},
		Facts: []DraftFact{
			{ID: "fact_origin", SubjectID: "char_hero", Predicate: "origin", Value: "unknown"},
		},
		Threads: []DraftThread{
			{ID: "thread_quest", Kind: "quest", Title: "Find the sword", Status: "open"},
		},
	}

	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 5, report.Inserted, "2 entities + 1 relation + 1 fact + 1 thread")
	assert.Equal(t, 0, report.Skipped)
	assert.Equal(t, 0, report.Rejected)

	assert.Contains(t, result.Entities, model.EntityID("char_hero"))
	assert.Contains(t, result.Entities, model.EntityID("loc_village"))
	assert.Len(t, result.Relations, 1)
	assert.Len(t, result.Facts, 1)
	assert.Len(t, result.Threads, 1)
	assert.Equal(t, []string{"fantasy"}, result.Canon.Genre)
}

func TestCompileDraftSkipsExistingByDefault(t *testing.T) {
	world := model.World{
		ID:   "world_test",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_hero": {ID: "char_hero", Type: "character", Name: "OldKael"},
		},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "NewKael"},
			{ID: "char_sage", Type: "character", Name: "Mirael"},
		},
	}

	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Inserted)
	assert.Equal(t, 1, report.Skipped)
	assert.Equal(t, "OldKael", result.Entities["char_hero"].Name)
	assert.Equal(t, "Mirael", result.Entities["char_sage"].Name)
}

func TestCompileDraftReplacePolicy(t *testing.T) {
	world := model.World{
		ID:   "world_test",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_hero": {ID: "char_hero", Type: "character", Name: "OldKael"},
		},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "NewKael"},
		},
	}

	result, report, err := CompileDraft(world, draft, CompileOptions{ConflictPolicy: ConflictPolicyReplace})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Inserted)
	assert.Equal(t, 0, report.Skipped)
	assert.Equal(t, "NewKael", result.Entities["char_hero"].Name)
}

func TestCompileDraftRelationDanglingRef(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	draft := Draft{
		Relations: []DraftRelation{
			{ID: "rel_x", Type: "knows", SourceID: "nonexistent_a", TargetID: "nonexistent_b"},
		},
	}

	_, _, err := CompileDraft(world, draft, CompileOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dangling")
}

func TestCompileDraftAllowDanglingRefs(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	draft := Draft{
		Relations: []DraftRelation{
			{ID: "rel_x", Type: "knows", SourceID: "nonexistent_a", TargetID: "nonexistent_b"},
		},
	}

	_, _, err := CompileDraft(world, draft, CompileOptions{AllowDanglingRefs: true})
	assert.NoError(t, err)
}

// --- Compile-time model validation ---

func TestCompileDraftRejectsInvalidThreadKind(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{
		Threads: []DraftThread{
			{ID: "t_bad", Kind: "subplot", Title: "Bad", Status: "open"},
			{ID: "t_good", Kind: "quest", Title: "Good", Status: "open"},
		},
	}
	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Rejected, "invalid thread kind should be rejected")
	assert.Equal(t, 1, report.Inserted)
	require.Len(t, result.Threads, 1)
	assert.Equal(t, model.ThreadID("t_good"), result.Threads[0].ID)
}

func TestCompileDraftRejectsInvalidMemoryOwner(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{
		Memories: []DraftMemory{
			{ID: "m_bad", OwnerKind: "rogue", Content: "x"},
			{ID: "m_ok", OwnerKind: "world", Content: "y"},
		},
	}
	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Rejected)
	assert.Equal(t, 1, report.Inserted)
	require.Len(t, result.Memory, 1)
}

func TestCompileDraftCanonDedupOnRepeat(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{Canon: &DraftCanon{Genre: []string{"fantasy", "epic"}, Tone: []string{"dark"}}}

	world1, _, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	world2, _, err := CompileDraft(world1, draft, CompileOptions{})
	require.NoError(t, err)

	assert.Equal(t, []string{"fantasy", "epic"}, world2.Canon.Genre)
	assert.Equal(t, []string{"dark"}, world2.Canon.Tone)
}

// --- Aliases / nicknames ---

func TestCompileDraftCopiesAliases(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_kael", Type: "character", Name: "Kael", Aliases: []string{"Kael the Brave", "凯尔"}},
		},
	}
	result, _, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	got := result.Entities["char_kael"]
	assert.Equal(t, []string{"Kael the Brave", "凯尔"}, got.Aliases)
}

func TestCompileDraftReplaceUnionsAliases(t *testing.T) {
	world := model.World{
		ID:   "w",
		Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"char_kael": {ID: "char_kael", Type: "character", Name: "Kael", Aliases: []string{"K", "Kael the Brave"}},
		},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_kael", Type: "character", Name: "Kael", Aliases: []string{"Kael the Brave", "凯尔"}},
		},
	}
	result, _, err := CompileDraft(world, draft, CompileOptions{ConflictPolicy: ConflictPolicyReplace})
	require.NoError(t, err)
	got := result.Entities["char_kael"]
	assert.Equal(t, []string{"K", "Kael the Brave", "凯尔"}, got.Aliases, "aliases should be union-deduped, preserving existing order")
}

func TestValidateDraftRejectsEmptyAlias(t *testing.T) {
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_a", Type: "character", Name: "A", Aliases: []string{"X", ""}},
		},
	}
	report := ValidateDraft(draft)
	assert.NotEmpty(t, report.Errors)
}

func TestMergeDraftsUnionsAliases(t *testing.T) {
	a := Draft{Entities: []DraftEntity{
		{ID: "char_kael", Type: "character", Name: "Kael", Aliases: []string{"K"}, Confidence: 0.7},
	}}
	b := Draft{Entities: []DraftEntity{
		{ID: "char_kael", Type: "character", Name: "Kael", Aliases: []string{"Kael the Brave"}, Confidence: 0.9},
	}}
	merged := MergeDrafts(a, b)
	require.Len(t, merged.Entities, 1)
	assert.ElementsMatch(t, []string{"K", "Kael the Brave"}, merged.Entities[0].Aliases)
}

// --- Alias resolver ---

type fakeAliasResolver struct {
	mapping map[string]string
}

func (f *fakeAliasResolver) Resolve(_ context.Context, e DraftEntity, _ model.World) (string, error) {
	if canon, ok := f.mapping[e.ID]; ok {
		return canon, nil
	}
	return "", nil
}

func TestNoopAliasResolverDoesNothing(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{
		Entities: []DraftEntity{{ID: "char_kael", Type: "character", Name: "Kael"}},
	}
	result, report, err := CompileDraft(world, draft, CompileOptions{Resolver: NoopAliasResolver{}})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Inserted)
	assert.Contains(t, result.Entities, model.EntityID("char_kael"))
}

func TestAliasResolverMergesIDs(t *testing.T) {
	world := model.World{
		ID:   "w",
		Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"char_kael": {ID: "char_kael", Type: "character", Name: "Kael"},
		},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_kael_brave", Type: "character", Name: "Kael the Brave", Description: "A wanderer."},
		},
		Facts: []DraftFact{
			{ID: "fact_origin", SubjectID: "char_kael_brave", Predicate: "origin", Value: "unknown"},
		},
	}

	resolver := &fakeAliasResolver{mapping: map[string]string{"char_kael_brave": "char_kael"}}
	result, report, err := CompileDraft(world, draft, CompileOptions{
		Resolver:       resolver,
		ConflictPolicy: ConflictPolicyReplace,
	})
	require.NoError(t, err)
	assert.NotContains(t, result.Entities, model.EntityID("char_kael_brave"), "alias should not create new entity")
	assert.Contains(t, result.Entities, model.EntityID("char_kael"))
	require.Len(t, result.Facts, 1)
	assert.Equal(t, model.EntityID("char_kael"), result.Facts[0].SubjectID, "fact references should rewrite to canonical id")
	require.Len(t, report.Aliases, 1)
	assert.Equal(t, "char_kael", report.Aliases["char_kael_brave"])
}

func TestAliasResolverPromotesDraftNameIntoCanonicalAliases(t *testing.T) {
	world := model.World{
		ID:   "w",
		Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"char_kael": {ID: "char_kael", Type: "character", Name: "Kael"},
		},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_kael_brave", Type: "character", Name: "Kael the Brave", Aliases: []string{"the Brave One"}},
		},
	}
	resolver := &fakeAliasResolver{mapping: map[string]string{"char_kael_brave": "char_kael"}}
	result, _, err := CompileDraft(world, draft, CompileOptions{Resolver: resolver})
	require.NoError(t, err)
	got := result.Entities["char_kael"]
	assert.ElementsMatch(t, []string{"Kael the Brave", "the Brave One"}, got.Aliases,
		"resolver should promote dropped draft Name + Aliases into canonical entity")
}

func TestAliasResolverPromotesIntoDraftEntityWhenCanonicalNotYetInWorld(t *testing.T) {
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_kael", Type: "character", Name: "Kael"},
			{ID: "char_kael_brave", Type: "character", Name: "Kael the Brave"},
		},
	}
	resolver := &fakeAliasResolver{mapping: map[string]string{"char_kael_brave": "char_kael"}}
	result, _, err := CompileDraft(world, draft, CompileOptions{Resolver: resolver})
	require.NoError(t, err)
	got := result.Entities["char_kael"]
	assert.Contains(t, got.Aliases, "Kael the Brave",
		"resolver should fold dropped draft Name into the canonical draft entity before insert")
}

func TestAliasResolverRewritesRelationEndpoints(t *testing.T) {
	world := model.World{
		ID:   "w",
		Name: "W",
		Entities: map[model.EntityID]model.Entity{
			"char_kael":   {ID: "char_kael", Type: "character", Name: "Kael"},
			"loc_village": {ID: "loc_village", Type: "location", Name: "Thornhaven"},
		},
	}
	draft := Draft{
		Relations: []DraftRelation{
			{ID: "rel_lives", Type: "lives_in", SourceID: "char_kael_brave", TargetID: "loc_village"},
		},
	}
	resolver := &fakeAliasResolver{mapping: map[string]string{"char_kael_brave": "char_kael"}}
	result, _, err := CompileDraft(world, draft, CompileOptions{Resolver: resolver})
	require.NoError(t, err)
	require.Len(t, result.Relations, 1)
	assert.Equal(t, model.EntityID("char_kael"), result.Relations[0].SourceID)
}

// --- Replace preserves runtime fields ---

func TestReplacePreservesEntityComponents(t *testing.T) {
	existing := model.Entity{
		ID:          "char_hero",
		Type:        "character",
		Name:        "Hero",
		Description: "old desc",
		Components:  map[string]any{"spatial": map[string]any{"location_id": "loc_town"}},
		State:       map[string]model.Value{"hp": {Kind: model.ValueKindNumber, Raw: 50}},
		Tags:        []string{"player"},
	}
	world := model.World{
		ID:       "w",
		Name:     "W",
		Entities: map[model.EntityID]model.Entity{"char_hero": existing},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "Hero the Brave", Description: "new desc", Tags: []string{"hero"}},
		},
	}

	result, _, err := CompileDraft(world, draft, CompileOptions{ConflictPolicy: ConflictPolicyReplace})
	require.NoError(t, err)
	got := result.Entities["char_hero"]
	assert.Equal(t, "Hero the Brave", got.Name, "name should be replaced")
	assert.Equal(t, "new desc", got.Description, "description should be replaced")
	assert.Equal(t, []string{"hero"}, got.Tags, "tags should be replaced")
	assert.NotNil(t, got.Components, "components must be preserved on replace")
	assert.NotNil(t, got.State, "state must be preserved on replace")
	assert.Contains(t, got.State, "hp")
}

func TestReplacePreservesThreadRuntimeFields(t *testing.T) {
	existing := model.WorldThread{
		ID:             "t_quest",
		Kind:           "quest",
		Title:          "Old title",
		Status:         "open",
		ParticipantIDs: []model.EntityID{"char_a", "char_b"},
		LocationID:     "loc_x",
	}
	world := model.World{
		ID:       "w",
		Name:     "W",
		Entities: map[model.EntityID]model.Entity{},
		Threads:  []model.WorldThread{existing},
	}
	draft := Draft{
		Threads: []DraftThread{
			{ID: "t_quest", Kind: "quest", Title: "New title", Status: "active"},
		},
	}
	result, _, err := CompileDraft(world, draft, CompileOptions{ConflictPolicy: ConflictPolicyReplace})
	require.NoError(t, err)
	require.Len(t, result.Threads, 1)
	got := result.Threads[0]
	assert.Equal(t, "New title", got.Title)
	assert.Equal(t, "active", got.Status)
	assert.Equal(t, []model.EntityID{"char_a", "char_b"}, got.ParticipantIDs, "participants preserved")
	assert.Equal(t, model.EntityID("loc_x"), got.LocationID, "location preserved")
}

func TestReplacePreservesMemoryRuntimeFields(t *testing.T) {
	existing := model.MemoryRecord{
		ID:         "m1",
		Owner:      model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
		Content:    "old content",
		SubjectIDs: []model.EntityID{"char_a"},
		EventIDs:   []model.EventID{"ev_1"},
	}
	world := model.World{
		ID:       "w",
		Name:     "W",
		Entities: map[model.EntityID]model.Entity{},
		Memory:   []model.MemoryRecord{existing},
	}
	draft := Draft{
		Memories: []DraftMemory{
			{ID: "m1", OwnerKind: "world", Content: "new content"},
		},
	}
	result, _, err := CompileDraft(world, draft, CompileOptions{ConflictPolicy: ConflictPolicyReplace})
	require.NoError(t, err)
	require.Len(t, result.Memory, 1)
	got := result.Memory[0]
	assert.Equal(t, "new content", got.Content)
	assert.Equal(t, []model.EntityID{"char_a"}, got.SubjectIDs, "subject_ids preserved")
	assert.Equal(t, []model.EventID{"ev_1"}, got.EventIDs, "event_ids preserved")
}

// --- LoadSource content-hash ID ---

func TestLoadSourceIDIncludesContentHash(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(pathA, []byte("Story A content"), 0o644))
	pathB := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(pathB, []byte("Story B content"), 0o644))

	docA, err := LoadSource(pathA)
	require.NoError(t, err)
	docB, err := LoadSource(pathB)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(docA.ID, "src_a_"), "expected src_a_<hash>, got %q", docA.ID)
	assert.True(t, strings.HasPrefix(docB.ID, "src_b_"), "expected src_b_<hash>, got %q", docB.ID)
	assert.NotEqual(t, docA.ID, docB.ID)
}

func TestLoadSourceSameNameDifferentContentDistinctIDs(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	pathA := filepath.Join(dirA, "novel.txt")
	pathB := filepath.Join(dirB, "novel.txt")
	require.NoError(t, os.WriteFile(pathA, []byte("Version 1"), 0o644))
	require.NoError(t, os.WriteFile(pathB, []byte("Version 2"), 0o644))

	docA, err := LoadSource(pathA)
	require.NoError(t, err)
	docB, err := LoadSource(pathB)
	require.NoError(t, err)

	assert.NotEqual(t, docA.ID, docB.ID, "same filename different content must produce different IDs")
}

func TestLoadSourceIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "novel.txt")
	require.NoError(t, os.WriteFile(path, []byte("Fixed content"), 0o644))

	doc1, err := LoadSource(path)
	require.NoError(t, err)
	doc2, err := LoadSource(path)
	require.NoError(t, err)

	assert.Equal(t, doc1.ID, doc2.ID, "same file should yield same ID (idempotent reload)")
}

// --- Atomicity: caller's world untouched on error ---

func TestCompileDraftIsAtomicOnError(t *testing.T) {
	world := model.World{
		ID:       "w",
		Name:     "W",
		Entities: map[model.EntityID]model.Entity{},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_hero", Type: "character", Name: "Hero"},
		},
		Relations: []DraftRelation{
			// Dangling target_id will trip CompileDraft AFTER entity was processed
			{ID: "rel_bad", Type: "knows", SourceID: "char_hero", TargetID: "ghost"},
		},
	}
	_, _, err := CompileDraft(world, draft, CompileOptions{})
	require.Error(t, err, "should fail on dangling relation")
	assert.NotContains(t, world.Entities, model.EntityID("char_hero"),
		"caller's world.Entities must NOT have been mutated when compile fails")
}

func TestCompileDraftDoesNotMutateCallerWorldOnSuccess(t *testing.T) {
	original := map[model.EntityID]model.Entity{
		"char_existing": {ID: "char_existing", Type: "character", Name: "Old"},
	}
	world := model.World{ID: "w", Name: "W", Entities: original}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_new", Type: "character", Name: "New"},
		},
	}
	compiled, _, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	assert.Contains(t, compiled.Entities, model.EntityID("char_new"))
	assert.NotContains(t, original, model.EntityID("char_new"),
		"caller's entity map must not gain new entries")
}

// --- Import orchestration ---

func TestImportFileSurfaceValidationWarnings(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "novel.txt")
	require.NoError(t, os.WriteFile(srcPath, []byte("text"), 0o644))

	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{}}
	parser := &fakeParser{
		draft: Draft{
			Entities: []DraftEntity{
				{ID: "char_a", Type: "character", Name: "A"},
			},
			Facts: []DraftFact{
				// dangling subject ref → warning
				{ID: "fact_ghost", SubjectID: "phantom", Predicate: "is", Value: "void"},
			},
		},
	}
	result, err := ImportFile(context.Background(), world, srcPath, parser, WholeDocumentChunker{}, CompileOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Validation.Warnings, "warnings should surface in result")
}

func TestImportFileWithDocParser(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "novel.txt")
	content := "Chapter 1\n\nThe hero set out."
	require.NoError(t, os.WriteFile(srcPath, []byte(content), 0o644))

	world := model.World{
		ID:       "world_import",
		Name:     "Import Test",
		Entities: map[model.EntityID]model.Entity{},
	}

	parser := &fakeParser{
		draft: Draft{
			Entities: []DraftEntity{
				{ID: "char_hero", Type: "character", Name: "Hero", SourceRefs: []string{"ch1"}},
			},
		},
	}

	result, err := ImportFile(context.Background(), world, srcPath, parser, WholeDocumentChunker{}, CompileOptions{})
	require.NoError(t, err)
	assert.Contains(t, result.World.Entities, model.EntityID("char_hero"))
	assert.Equal(t, 1, result.CompileReport.Inserted)
	assert.NotEmpty(t, result.SourceDocument.ID)
}

// indexedChunkParser maps chunk Index to a draft so tests are independent of
// content-hash-based chunk IDs.
type indexedChunkParser struct {
	byIndex map[int]Draft
}

func (p *indexedChunkParser) ParseChunk(_ context.Context, chunk SourceChunk) (Draft, error) {
	if d, ok := p.byIndex[chunk.Index]; ok {
		return d, nil
	}
	return Draft{}, nil
}

// h1Chunker is a minimal in-test Chunker that splits md by "# " headings.
// Kept inline because internal/textchunk imports ingest (no cycle from a
// test using package ingest).
type h1Chunker struct{}

func (h1Chunker) Chunk(doc SourceDocument) []SourceChunk {
	lines := strings.Split(strings.TrimSpace(doc.Text), "\n")
	var chunks []SourceChunk
	var current strings.Builder
	heading := ""
	idx := 0
	flush := func() {
		body := strings.TrimSpace(current.String())
		if body == "" && heading == "" {
			return
		}
		chunks = append(chunks, SourceChunk{
			ID:         fmt.Sprintf("%s_chunk_%d", doc.ID, idx),
			SourceID:   doc.ID,
			SourceKind: doc.Kind,
			Index:      idx,
			Heading:    heading,
			Text:       body,
		})
		idx++
		current.Reset()
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			flush()
			heading = strings.TrimPrefix(line, "# ")
		} else {
			current.WriteString(line)
			current.WriteString("\n")
		}
	}
	flush()
	return chunks
}

func TestImportFileWithChunkParser(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "novel.md")
	content := "# Chapter 1\n\nThe hero set out.\n\n# Chapter 2\n\nA sage appeared."
	require.NoError(t, os.WriteFile(srcPath, []byte(content), 0o644))

	world := model.World{
		ID:       "world_import",
		Name:     "Import Test",
		Entities: map[model.EntityID]model.Entity{},
	}

	cp := &indexedChunkParser{
		byIndex: map[int]Draft{
			0: {Entities: []DraftEntity{{ID: "char_hero", Type: "character", Name: "Hero", Confidence: 0.9}}},
			1: {Entities: []DraftEntity{{ID: "char_sage", Type: "character", Name: "Sage", Confidence: 0.85}}},
		},
	}

	result, err := ImportFileChunked(context.Background(), world, srcPath, cp, h1Chunker{}, CompileOptions{})
	require.NoError(t, err)
	assert.Contains(t, result.World.Entities, model.EntityID("char_hero"))
	assert.Contains(t, result.World.Entities, model.EntityID("char_sage"))
	assert.Equal(t, 2, result.CompileReport.Inserted)
}

func TestImportFileChunkedRequiresChunker(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "x.md")
	require.NoError(t, os.WriteFile(srcPath, []byte("# A\n\nb"), 0o644))
	world := model.World{ID: "w", Name: "W", Entities: map[model.EntityID]model.Entity{}}
	_, err := ImportFileChunked(context.Background(), world, srcPath, &indexedChunkParser{}, nil, CompileOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-nil Chunker")
}

// --- Compile report provenance ---

func TestCompileReportTracksSourceRefs(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_a", Type: "character", Name: "A", SourceRefs: []string{"ch1_p1", "ch2_p3"}},
		},
	}

	_, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	require.Len(t, report.Provenance, 1)
	assert.Equal(t, "char_a", report.Provenance[0].WorldID)
	assert.Equal(t, []string{"ch1_p1", "ch2_p3"}, report.Provenance[0].SourceRefs)
}

// --- Edge cases ---

func TestCompileDraftEmptyDraft(t *testing.T) {
	world := model.World{ID: "world_test", Name: "Test", Entities: map[model.EntityID]model.Entity{}}
	result, report, err := CompileDraft(world, Draft{}, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, report.Inserted)
	assert.Equal(t, world.ID, result.ID)
}

func TestCompileDraftThreadValidation(t *testing.T) {
	world := model.World{ID: "world_test", Name: "Test", Entities: map[model.EntityID]model.Entity{}}
	draft := Draft{
		Threads: []DraftThread{
			{ID: "t1", Kind: "quest", Title: "Quest", Status: "open"},
		},
	}
	result, _, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	require.Len(t, result.Threads, 1)
	assert.Equal(t, model.ThreadStatusOpen, result.Threads[0].Status)
}

func TestCompileDraftFactMerge(t *testing.T) {
	world := model.World{
		ID:       "world_test",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
		Facts: []model.Fact{
			{ID: "fact_existing", SubjectID: "char_a", Predicate: "status", Value: model.Value{Kind: model.ValueKindString, Raw: "alive"}},
		},
	}
	draft := Draft{
		Entities: []DraftEntity{
			{ID: "char_a", Type: "character", Name: "A"},
		},
		Facts: []DraftFact{
			{ID: "fact_existing", SubjectID: "char_a", Predicate: "status", Value: "dead"},
			{ID: "fact_new", SubjectID: "char_a", Predicate: "mood", Value: "happy"},
		},
	}

	result, report, err := CompileDraft(world, draft, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, report.Skipped)
	assert.Len(t, result.Facts, 2)
}
