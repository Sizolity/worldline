package runtime

import (
	"strings"
	"testing"

	"github.com/sizolity/worldline/internal/world/model"
)

func cleanWorld() model.World {
	return model.World{
		ID: "test_world", Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "Alice"},
			"char_b": {ID: "char_b", Type: "character", Name: "Bob"},
			"loc_1":  {ID: "loc_1", Type: "location", Name: "Market"},
		},
		Relations: []model.Relation{
			{ID: "rel_1", Type: "ally", SourceID: "char_a", TargetID: "char_b"},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "char_a", Predicate: "alive", Value: model.Value{Kind: model.ValueKindBoolean, Raw: true}},
		},
		Memories: []model.MemoryRecord{
			{
				ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_a"},
				Content: "I saw something.", SubjectIDs: []model.EntityID{"char_b"},
			},
		},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector, ActorIDs: []model.EntityID{"char_a"}},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusActive},
		},
	}
}

func TestDeepValidateCleanWorld(t *testing.T) {
	t.Parallel()
	report := DeepValidate(cleanWorld())
	if !report.IsClean() {
		t.Fatalf("expected clean, got %d issues: %+v", len(report.Issues), report.Issues)
	}
}

func TestDeepValidateRelationBrokenRef(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Relations = append(w.Relations, model.Relation{ID: "rel_bad", Type: "enemy", SourceID: "char_a", TargetID: "nonexistent"})
	report := DeepValidate(w)
	if report.IsClean() {
		t.Fatal("expected issues")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "relations[1].target_id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("missing relation target_id issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateFactBrokenRef(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Facts = append(w.Facts, model.Fact{ID: "f_bad", SubjectID: "ghost"})
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "facts[1].subject_id" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing fact subject_id issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateMemoryOwnerBrokenRef(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Memories = append(w.Memories, model.MemoryRecord{
		ID: "m_bad", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "ghost"},
		Content: "x",
	})
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "memory[1].owner.id" && issue.Severity == ValidationError {
			found = true
		}
	}
	if !found {
		t.Errorf("missing memory owner issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateMemorySubjectWarning(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Memories[0].SubjectIDs = []model.EntityID{"nonexistent"}
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "memory[0].subject_ids[0]" && issue.Severity == ValidationWarning {
			found = true
		}
	}
	if !found {
		t.Errorf("missing memory subject warning, got: %+v", report.Issues)
	}
}

func TestDeepValidateEventActorWarning(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.EventLog[0].ActorIDs = []model.EntityID{"ghost"}
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "event_log[0].actor_ids[0]" && issue.Severity == ValidationWarning {
			found = true
		}
	}
	if !found {
		t.Errorf("missing event actor warning, got: %+v", report.Issues)
	}
}

func TestDeepValidateDuplicateIDs(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Facts = append(w.Facts, model.Fact{ID: "f1", SubjectID: "char_a"})
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "facts[1]" && issue.Severity == ValidationError {
			found = true
		}
	}
	if !found {
		t.Errorf("missing duplicate fact issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateCanonSecretsBrokenRef(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Canon.Secrets = []model.EntityID{"ghost"}
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "canon.secrets[0]" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing canon secrets issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateErrorCount(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Relations = append(w.Relations, model.Relation{ID: "rel_bad", Type: "x", SourceID: "ghost", TargetID: "ghost2"})
	w.EventLog[0].ActorIDs = []model.EntityID{"ghost3"}
	report := DeepValidate(w)
	if report.ErrorCount() < 2 {
		t.Errorf("expected >= 2 errors, got %d", report.ErrorCount())
	}
}

func TestDeepValidateEventLocationBrokenRef(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.EventLog[0].LocationID = "ghost_location"
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "event_log[0].location_id" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing event location_id issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateEntityValidation(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Entities["bad"] = model.Entity{ID: "bad", Name: "Bad"}
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "entities[bad]" && issue.Severity == ValidationError {
			found = true
		}
	}
	if !found {
		t.Errorf("missing entity validation issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateThreadValidation(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Threads = append(w.Threads, model.WorldThread{ID: "t_bad", Title: "Bad", Kind: "invalid_kind", Status: model.ThreadStatusOpen})
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "threads[1]" && issue.Severity == ValidationError {
			found = true
		}
	}
	if !found {
		t.Errorf("missing thread validation issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateMemoryValidation(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.Memories = append(w.Memories, model.MemoryRecord{
		ID: "m_bad", Owner: model.MemoryOwner{Kind: "invalid_owner"},
	})
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "memory[1]" && issue.Severity == ValidationError {
			found = true
		}
	}
	if !found {
		t.Errorf("missing memory validation issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateEventLogValidation(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.EventLog = append(w.EventLog, model.WorldEvent{ID: "ev_bad"})
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "event_log[1]" && issue.Severity == ValidationError {
			found = true
		}
	}
	if !found {
		t.Errorf("missing event_log validation issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateQueueItemValidation(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.EventQueue = []model.EventQueueItem{
		{Event: model.WorldEvent{ID: "q1", Type: "note", Source: "test"}, ErrorPolicy: "bad_policy"},
	}
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "event_queue[0]" && issue.Severity == ValidationError {
			found = true
		}
	}
	if !found {
		t.Errorf("missing queue item validation issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateQueueDuplicateIDs(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.EventQueue = []model.EventQueueItem{
		{Event: model.WorldEvent{ID: "q1", Type: "note", Source: "test"}},
		{Event: model.WorldEvent{ID: "q1", Type: "note", Source: "test"}},
	}
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "event_queue[1]" && issue.Severity == ValidationError {
			found = true
		}
	}
	if !found {
		t.Errorf("missing queue duplicate ID issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateQueueEntityRef(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.EventQueue = []model.EventQueueItem{
		{Event: model.WorldEvent{ID: "q1", Type: "note", Source: "test", ActorIDs: []model.EntityID{"ghost"}}},
	}
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "event_queue[0].event.actor_ids[0]" && issue.Severity == ValidationWarning {
			found = true
		}
	}
	if !found {
		t.Errorf("missing queue actor ref issue, got: %+v", report.Issues)
	}
}

func TestDeepValidateThreadEffectBrokenRef(t *testing.T) {
	t.Parallel()
	w := cleanWorld()
	w.EventLog = append(w.EventLog, model.WorldEvent{
		ID: "ev2", Type: model.EventTypeThreadChanged, Source: model.EventSourceDirector,
		Effects: []model.Effect{{Kind: model.EffectUpdateThread, TargetID: "nonexistent_thread"}},
	})
	report := DeepValidate(w)
	found := false
	for _, issue := range report.Issues {
		if issue.Path == "event_log[1].effects" && issue.Severity == ValidationWarning {
			found = true
		}
	}
	if !found {
		t.Errorf("missing thread effect ref issue, got: %+v", report.Issues)
	}
}

func TestFormatValidationReportClean(t *testing.T) {
	t.Parallel()
	report := ValidationReport{WorldID: "w1", Issues: []ValidationIssue{}}
	text := FormatValidationReport(report)
	if !strings.Contains(text, "clean") {
		t.Errorf("expected 'clean':\n%s", text)
	}
}

func TestFormatValidationReportWithIssues(t *testing.T) {
	t.Parallel()
	report := ValidationReport{
		WorldID: "w1",
		Issues: []ValidationIssue{
			{Severity: ValidationError, Path: "entities[bad]", Message: "type required"},
			{Severity: ValidationWarning, Path: "memory[0].subject_ids[0]", Message: "nonexistent entity"},
		},
	}
	text := FormatValidationReport(report)
	if !strings.Contains(text, "1 error(s), 1 warning(s)") {
		t.Errorf("missing summary:\n%s", text)
	}
	if !strings.Contains(text, "type required") {
		t.Errorf("missing error:\n%s", text)
	}
	if !strings.Contains(text, "nonexistent entity") {
		t.Errorf("missing warning:\n%s", text)
	}
}
