package runtime

import (
	"fmt"
	"strings"

	"github.com/sizolity/worldline/world/model"
)

// ValidationIssue describes a single consistency problem in a world.
type ValidationIssue struct {
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

const (
	ValidationError   = "error"
	ValidationWarning = "warning"
)

// ValidationReport is the result of a deep consistency check.
type ValidationReport struct {
	WorldID string            `json:"world_id"`
	Issues  []ValidationIssue `json:"issues"`
}

// IsClean returns true if no issues were found.
func (r ValidationReport) IsClean() bool { return len(r.Issues) == 0 }

// ErrorCount returns the number of error-severity issues.
func (r ValidationReport) ErrorCount() int {
	n := 0
	for _, issue := range r.Issues {
		if issue.Severity == ValidationError {
			n++
		}
	}
	return n
}

// DeepValidate performs cross-reference consistency checks on a world snapshot.
func DeepValidate(w model.World) ValidationReport {
	report := ValidationReport{
		WorldID: string(w.ID),
		Issues:  []ValidationIssue{},
	}

	if err := w.Validate(); err != nil {
		report.Issues = append(report.Issues, ValidationIssue{
			Severity: ValidationError, Path: "world", Message: err.Error(),
		})
		return report
	}

	entitySet := make(map[model.EntityID]bool, len(w.Entities))
	for id := range w.Entities {
		entitySet[id] = true
	}
	eventSet := eventIDSet(w.EventLog)
	threadSet := threadIDSet(w.Threads)

	checkDuplicateEntities(&report, w)
	checkDuplicateSliceIDs(&report, "facts", factIDSlice(w.Facts))
	checkDuplicateSliceIDs(&report, "relations", relationIDSlice(w.Relations))
	checkDuplicateSliceIDs(&report, "memory", memoryIDSlice(w.Memory))
	checkDuplicateSliceIDs(&report, "event_log", eventIDSlice(w.EventLog))
	checkDuplicateSliceIDs(&report, "threads", threadIDSlice(w.Threads))
	checkDuplicateSliceIDs(&report, "rules", ruleIDSlice(w.Rules))

	for i, rel := range w.Relations {
		path := fmt.Sprintf("relations[%d]", i)
		if !entitySet[rel.SourceID] {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationError, Path: path + ".source_id",
				Message: fmt.Sprintf("references non-existent entity %q", rel.SourceID),
			})
		}
		if !entitySet[rel.TargetID] {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationError, Path: path + ".target_id",
				Message: fmt.Sprintf("references non-existent entity %q", rel.TargetID),
			})
		}
	}

	for i, fact := range w.Facts {
		if fact.SubjectID != "" && !entitySet[fact.SubjectID] {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationError,
				Path:     fmt.Sprintf("facts[%d].subject_id", i),
				Message:  fmt.Sprintf("references non-existent entity %q", fact.SubjectID),
			})
		}
	}

	for i, mem := range w.Memory {
		path := fmt.Sprintf("memory[%d]", i)
		if mem.Owner.Kind == model.MemoryOwnerKindCharacter && mem.Owner.ID != "" {
			if !entitySet[model.EntityID(mem.Owner.ID)] {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: ValidationError, Path: path + ".owner.id",
					Message: fmt.Sprintf("character memory owner references non-existent entity %q", mem.Owner.ID),
				})
			}
		}
		for j, sid := range mem.SubjectIDs {
			if !entitySet[sid] {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: ValidationWarning,
					Path:     fmt.Sprintf("%s.subject_ids[%d]", path, j),
					Message:  fmt.Sprintf("references non-existent entity %q", sid),
				})
			}
		}
		for j, eid := range mem.EventIDs {
			if !eventSet[eid] {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: ValidationWarning,
					Path:     fmt.Sprintf("%s.event_ids[%d]", path, j),
					Message:  fmt.Sprintf("references non-existent event %q", eid),
				})
			}
		}
	}

	for i, ev := range w.EventLog {
		path := fmt.Sprintf("event_log[%d]", i)
		for j, aid := range ev.ActorIDs {
			if !entitySet[aid] {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: ValidationWarning, Path: fmt.Sprintf("%s.actor_ids[%d]", path, j),
					Message: fmt.Sprintf("references non-existent entity %q", aid),
				})
			}
		}
		for j, tid := range ev.TargetIDs {
			if !entitySet[tid] {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: ValidationWarning, Path: fmt.Sprintf("%s.target_ids[%d]", path, j),
					Message: fmt.Sprintf("references non-existent entity %q", tid),
				})
			}
		}
		if ev.LocationID != "" && !entitySet[ev.LocationID] {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationWarning, Path: path + ".location_id",
				Message: fmt.Sprintf("references non-existent entity %q", ev.LocationID),
			})
		}
	}

	for i, sid := range w.Canon.Secrets {
		if !entitySet[sid] {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationWarning,
				Path:     fmt.Sprintf("canon.secrets[%d]", i),
				Message:  fmt.Sprintf("references non-existent entity %q", sid),
			})
		}
	}

	for id, e := range w.Entities {
		if err := e.Validate(); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationError,
				Path:     fmt.Sprintf("entities[%s]", id),
				Message:  err.Error(),
			})
		}
	}

	for i, th := range w.Threads {
		if err := th.Validate(); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationError,
				Path:     fmt.Sprintf("threads[%d]", i),
				Message:  err.Error(),
			})
		}
	}

	for i, mem := range w.Memory {
		if err := mem.Validate(); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationError,
				Path:     fmt.Sprintf("memory[%d]", i),
				Message:  err.Error(),
			})
		}
	}

	for i, ev := range w.EventLog {
		if err := ev.Validate(); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationError,
				Path:     fmt.Sprintf("event_log[%d]", i),
				Message:  err.Error(),
			})
		}
	}

	checkDuplicateSliceIDs(&report, "event_queue", queueEventIDSlice(w.EventQueue))
	for i, item := range w.EventQueue {
		path := fmt.Sprintf("event_queue[%d]", i)
		if err := item.Validate(); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationError, Path: path, Message: err.Error(),
			})
		}
		for j, aid := range item.Event.ActorIDs {
			if !entitySet[aid] {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: ValidationWarning,
					Path:     fmt.Sprintf("%s.event.actor_ids[%d]", path, j),
					Message:  fmt.Sprintf("references non-existent entity %q", aid),
				})
			}
		}
		for j, tid := range item.Event.TargetIDs {
			if !entitySet[tid] {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: ValidationWarning,
					Path:     fmt.Sprintf("%s.event.target_ids[%d]", path, j),
					Message:  fmt.Sprintf("references non-existent entity %q", tid),
				})
			}
		}
	}

	for i, ev := range w.EventLog {
		for _, eff := range ev.Effects {
			if eff.Kind == model.EffectUpdateThread || eff.Kind == model.EffectCloseThread {
				if !threadSet[model.ThreadID(eff.TargetID)] {
					report.Issues = append(report.Issues, ValidationIssue{
						Severity: ValidationWarning,
						Path:     fmt.Sprintf("event_log[%d].effects", i),
						Message:  fmt.Sprintf("thread effect references non-existent thread %q", eff.TargetID),
					})
				}
			}
		}
	}

	return report
}

func checkDuplicateEntities(report *ValidationReport, w model.World) {
	// Entity map naturally deduplicates; nothing to check.
}

func checkDuplicateSliceIDs(report *ValidationReport, collection string, ids []string) {
	seen := make(map[string]bool, len(ids))
	for i, id := range ids {
		if seen[id] {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: ValidationError,
				Path:     fmt.Sprintf("%s[%d]", collection, i),
				Message:  fmt.Sprintf("duplicate id %q", id),
			})
		}
		seen[id] = true
	}
}

func eventIDSet(events []model.WorldEvent) map[model.EventID]bool {
	m := make(map[model.EventID]bool, len(events))
	for _, e := range events {
		m[e.ID] = true
	}
	return m
}

func threadIDSet(threads []model.WorldThread) map[model.ThreadID]bool {
	m := make(map[model.ThreadID]bool, len(threads))
	for _, t := range threads {
		m[t.ID] = true
	}
	return m
}

func factIDSlice(facts []model.Fact) []string {
	out := make([]string, len(facts))
	for i, f := range facts {
		out[i] = string(f.ID)
	}
	return out
}

func relationIDSlice(rels []model.Relation) []string {
	out := make([]string, len(rels))
	for i, r := range rels {
		out[i] = string(r.ID)
	}
	return out
}

func memoryIDSlice(mems []model.MemoryRecord) []string {
	out := make([]string, len(mems))
	for i, m := range mems {
		out[i] = string(m.ID)
	}
	return out
}

func eventIDSlice(events []model.WorldEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = string(e.ID)
	}
	return out
}

func threadIDSlice(threads []model.WorldThread) []string {
	out := make([]string, len(threads))
	for i, t := range threads {
		out[i] = string(t.ID)
	}
	return out
}

func ruleIDSlice(rules []model.Rule) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = string(r.ID)
	}
	return out
}

func queueEventIDSlice(items []model.EventQueueItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = string(item.Event.ID)
	}
	return out
}

// FormatValidationReport returns a human-readable text report.
func FormatValidationReport(r ValidationReport) string {
	if r.IsClean() {
		return fmt.Sprintf("World %s: clean — no issues found.\n", r.WorldID)
	}
	errors := r.ErrorCount()
	warnings := len(r.Issues) - errors

	var b strings.Builder
	fmt.Fprintf(&b, "World %s: %d error(s), %d warning(s)\n\n", r.WorldID, errors, warnings)

	if errors > 0 {
		b.WriteString("## Errors\n\n")
		for _, issue := range r.Issues {
			if issue.Severity == ValidationError {
				fmt.Fprintf(&b, "  ✗ %s: %s\n", issue.Path, issue.Message)
			}
		}
		b.WriteString("\n")
	}
	if warnings > 0 {
		b.WriteString("## Warnings\n\n")
		for _, issue := range r.Issues {
			if issue.Severity == ValidationWarning {
				fmt.Fprintf(&b, "  ! %s: %s\n", issue.Path, issue.Message)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
