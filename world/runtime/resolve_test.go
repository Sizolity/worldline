package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sizolity/worldline/world/model"
)

type fakeResolver struct {
	resolutions []Resolution
	idx         int
	inputs      []ConflictInput
}

func (r *fakeResolver) Resolve(_ context.Context, input ConflictInput) (Resolution, error) {
	r.inputs = append(r.inputs, input)
	if r.idx >= len(r.resolutions) {
		return Resolution{}, fmt.Errorf("no more resolutions")
	}
	res := r.resolutions[r.idx]
	r.idx++
	return res, nil
}

type fakeGen struct {
	responses []string
	idx       int
}

func (g *fakeGen) Generate(_ context.Context, _, _ string) (string, error) {
	if g.idx >= len(g.responses) {
		return "", fmt.Errorf("no more responses")
	}
	r := g.responses[g.idx]
	g.idx++
	return r, nil
}

func TestResolveMergeConflictsPickSource(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "Alice the Brave"}

	target := baseWorld()
	target.ID = "target"
	target.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "Alice the Wise"}

	merged, report := MergeWorlds(base, source, target)
	if !report.HasConflicts() {
		t.Fatal("expected conflict")
	}

	resolver := &fakeResolver{resolutions: []Resolution{
		{Pick: PickSource, Reason: "Source has better name"},
	}}

	resolved, resolutions, err := ResolveMergeConflicts(context.Background(), merged, base, source, target, report.Conflicts, resolver)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(resolutions) != 1 {
		t.Fatalf("resolutions = %d, want 1", len(resolutions))
	}
	if resolved.Entities["e1"].Name != "Alice the Brave" {
		t.Errorf("entity name = %q, want 'Alice the Brave'", resolved.Entities["e1"].Name)
	}
}

func TestResolveMergeConflictsPickTarget(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "Alice X"}

	target := baseWorld()
	target.ID = "target"
	target.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "Alice Y"}

	merged, report := MergeWorlds(base, source, target)

	resolver := &fakeResolver{resolutions: []Resolution{
		{Pick: PickTarget, Reason: "Target version preferred"},
	}}

	resolved, _, err := ResolveMergeConflicts(context.Background(), merged, base, source, target, report.Conflicts, resolver)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resolved.Entities["e1"].Name != "Alice Y" {
		t.Errorf("entity name = %q, want 'Alice Y'", resolved.Entities["e1"].Name)
	}
}

func TestResolveMergeConflictsPickMerged(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "Alice the Brave"}

	target := baseWorld()
	target.ID = "target"
	target.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "Alice the Wise"}

	merged, report := MergeWorlds(base, source, target)

	mergedEntity := model.Entity{ID: "e1", Type: "character", Name: "Alice the Brave and Wise"}
	resolver := &fakeResolver{resolutions: []Resolution{
		{Pick: PickMerged, Reason: "Combined both names", Entity: &mergedEntity},
	}}

	resolved, _, err := ResolveMergeConflicts(context.Background(), merged, base, source, target, report.Conflicts, resolver)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resolved.Entities["e1"].Name != "Alice the Brave and Wise" {
		t.Errorf("entity name = %q", resolved.Entities["e1"].Name)
	}
}

func TestResolveMergeConflictsThreadStatus(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Threads[0].Status = model.ThreadStatusResolved

	target := baseWorld()
	target.ID = "target"
	target.Threads[0].Status = model.ThreadStatusDormant

	merged, report := MergeWorlds(base, source, target)

	resolver := &fakeResolver{resolutions: []Resolution{
		{Pick: PickSource, Reason: "Quest was resolved in source"},
	}}

	resolved, _, err := ResolveMergeConflicts(context.Background(), merged, base, source, target, report.Conflicts, resolver)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resolved.Threads[0].Status != model.ThreadStatusResolved {
		t.Errorf("thread status = %q, want resolved", resolved.Threads[0].Status)
	}
}

func TestResolveMergeConflictsReceivesEntityVersions(t *testing.T) {
	t.Parallel()

	base := baseWorld()
	source := baseWorld()
	source.ID = "source"
	source.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "S"}

	target := baseWorld()
	target.ID = "target"
	target.Entities["e1"] = model.Entity{ID: "e1", Type: "character", Name: "T"}

	merged, report := MergeWorlds(base, source, target)

	resolver := &fakeResolver{resolutions: []Resolution{
		{Pick: PickTarget, Reason: "ok"},
	}}

	ResolveMergeConflicts(context.Background(), merged, base, source, target, report.Conflicts, resolver)

	if len(resolver.inputs) != 1 {
		t.Fatalf("inputs = %d", len(resolver.inputs))
	}
	input := resolver.inputs[0]
	if input.BaseEntity == nil || input.BaseEntity.Name != "Alice" {
		t.Error("missing or wrong base entity")
	}
	if input.SourceEntity == nil || input.SourceEntity.Name != "S" {
		t.Error("missing or wrong source entity")
	}
	if input.TargetEntity == nil || input.TargetEntity.Name != "T" {
		t.Error("missing or wrong target entity")
	}
}

func TestLLMConflictResolverParsesResponse(t *testing.T) {
	t.Parallel()

	gen := &fakeGen{responses: []string{
		`{"pick":"source","reason":"Source is better"}`,
	}}
	resolver := NewLLMConflictResolver(gen)

	res, err := resolver.Resolve(context.Background(), ConflictInput{
		Conflict: MergeConflict{Kind: "entity", ID: "e1", Desc: "test"},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Pick != PickSource {
		t.Errorf("pick = %q", res.Pick)
	}
	if res.Reason != "Source is better" {
		t.Errorf("reason = %q", res.Reason)
	}
}

func TestLLMConflictResolverStripsMarkdown(t *testing.T) {
	t.Parallel()

	gen := &fakeGen{responses: []string{
		"```json\n{\"pick\":\"target\",\"reason\":\"ok\"}\n```",
	}}
	resolver := NewLLMConflictResolver(gen)

	res, err := resolver.Resolve(context.Background(), ConflictInput{
		Conflict: MergeConflict{Kind: "entity", ID: "e1", Desc: "test"},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Pick != PickTarget {
		t.Errorf("pick = %q", res.Pick)
	}
}

func TestLLMConflictResolverRejectsInvalidPick(t *testing.T) {
	t.Parallel()

	gen := &fakeGen{responses: []string{
		`{"pick":"both","reason":"why not"}`,
	}}
	resolver := NewLLMConflictResolver(gen)

	_, err := resolver.Resolve(context.Background(), ConflictInput{
		Conflict: MergeConflict{Kind: "entity", ID: "e1", Desc: "test"},
	})
	if err == nil {
		t.Fatal("expected error for invalid pick")
	}
}

func TestLLMConflictResolverParsesEntityMerge(t *testing.T) {
	t.Parallel()

	merged := model.Entity{ID: "e1", Type: "character", Name: "Combined"}
	data, _ := json.Marshal(merged)
	gen := &fakeGen{responses: []string{
		fmt.Sprintf(`{"pick":"merged","reason":"combined","entity":%s}`, string(data)),
	}}
	resolver := NewLLMConflictResolver(gen)

	res, err := resolver.Resolve(context.Background(), ConflictInput{
		Conflict: MergeConflict{Kind: "entity", ID: "e1", Desc: "test"},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Pick != PickMerged {
		t.Errorf("pick = %q", res.Pick)
	}
	if res.Entity == nil || res.Entity.Name != "Combined" {
		t.Error("missing or wrong merged entity")
	}
}
