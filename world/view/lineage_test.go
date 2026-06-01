package view

import (
	"context"
	"testing"

	"github.com/sizolity/worldline/world/model"
	"github.com/sizolity/worldline/world/store"
)

func TestListWorldsEmpty(t *testing.T) {
	t.Parallel()
	st := store.NewFileStore(t.TempDir())
	ids, err := st.ListWorlds(context.Background())
	if err != nil {
		t.Fatalf("ListWorlds: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0 worlds, got %d", len(ids))
	}
}

func TestListWorldsReturnsSavedWorlds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())

	for _, id := range []string{"world_a", "world_b", "world_c"} {
		if err := st.SaveSnapshot(ctx, model.World{ID: model.WorldID(id), Name: id}); err != nil {
			t.Fatal(err)
		}
	}

	ids, err := st.ListWorlds(ctx)
	if err != nil {
		t.Fatalf("ListWorlds: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 worlds, got %d", len(ids))
	}
	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	for _, want := range []string{"world_a", "world_b", "world_c"} {
		if !found[want] {
			t.Fatalf("missing world %q in %v", want, ids)
		}
	}
}

// buildLineageStore creates:
//
//	root
//	├── child_a (forked from root@5)
//	│   └── grandchild (forked from child_a@10)
//	└── child_b (forked from root@3)
func buildLineageStore(t *testing.T) *store.FileStore {
	t.Helper()
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())

	root := model.World{ID: "root", Name: "Root", Clock: model.WorldClock{Sequence: 10}}
	if err := st.SaveSnapshot(ctx, root); err != nil {
		t.Fatal(err)
	}

	childA := model.World{
		ID: "child_a", Name: "Child A",
		Clock:    model.WorldClock{Sequence: 15},
		Metadata: model.WorldMetadata{Fork: &model.ForkInfo{ParentWorldID: "root", ForkSequence: 5}},
	}
	if err := st.SaveSnapshot(ctx, childA); err != nil {
		t.Fatal(err)
	}

	childB := model.World{
		ID: "child_b", Name: "Child B",
		Clock:    model.WorldClock{Sequence: 7},
		Metadata: model.WorldMetadata{Fork: &model.ForkInfo{ParentWorldID: "root", ForkSequence: 3}},
	}
	if err := st.SaveSnapshot(ctx, childB); err != nil {
		t.Fatal(err)
	}

	grandchild := model.World{
		ID: "grandchild", Name: "Grandchild",
		Clock:    model.WorldClock{Sequence: 20},
		Metadata: model.WorldMetadata{Fork: &model.ForkInfo{ParentWorldID: "child_a", ForkSequence: 10}},
	}
	if err := st.SaveSnapshot(ctx, grandchild); err != nil {
		t.Fatal(err)
	}

	return st
}

func TestAncestorsFromRoot(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	ancestors, err := Ancestors(context.Background(), st, "root")
	if err != nil {
		t.Fatalf("Ancestors: %v", err)
	}
	if len(ancestors) != 0 {
		t.Fatalf("root should have 0 ancestors, got %d", len(ancestors))
	}
}

func TestAncestorsFromChild(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	ancestors, err := Ancestors(context.Background(), st, "child_a")
	if err != nil {
		t.Fatalf("Ancestors: %v", err)
	}
	if len(ancestors) != 1 {
		t.Fatalf("child_a should have 1 ancestor, got %d", len(ancestors))
	}
	if ancestors[0].WorldID != "root" {
		t.Fatalf("ancestor = %q, want root", ancestors[0].WorldID)
	}
}

func TestAncestorsFromGrandchild(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	ancestors, err := Ancestors(context.Background(), st, "grandchild")
	if err != nil {
		t.Fatalf("Ancestors: %v", err)
	}
	if len(ancestors) != 2 {
		t.Fatalf("grandchild should have 2 ancestors, got %d", len(ancestors))
	}
	if ancestors[0].WorldID != "child_a" {
		t.Fatalf("first ancestor = %q, want child_a", ancestors[0].WorldID)
	}
	if ancestors[1].WorldID != "root" {
		t.Fatalf("second ancestor = %q, want root", ancestors[1].WorldID)
	}
}

func TestChildrenOfRoot(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	children, err := Children(context.Background(), st, "root")
	if err != nil {
		t.Fatalf("Children: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("root should have 2 children, got %d", len(children))
	}
	ids := map[model.WorldID]bool{}
	for _, c := range children {
		ids[c.WorldID] = true
	}
	if !ids["child_a"] || !ids["child_b"] {
		t.Fatalf("children = %v, want child_a + child_b", children)
	}
}

func TestChildrenOfLeaf(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	children, err := Children(context.Background(), st, "child_b")
	if err != nil {
		t.Fatalf("Children: %v", err)
	}
	if len(children) != 0 {
		t.Fatalf("child_b should have 0 children, got %d", len(children))
	}
}

func TestSiblingsOfChildA(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	siblings, err := Siblings(context.Background(), st, "child_a")
	if err != nil {
		t.Fatalf("Siblings: %v", err)
	}
	if len(siblings) != 1 {
		t.Fatalf("child_a should have 1 sibling, got %d", len(siblings))
	}
	if siblings[0].WorldID != "child_b" {
		t.Fatalf("sibling = %q, want child_b", siblings[0].WorldID)
	}
}

func TestSiblingsOfRoot(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	siblings, err := Siblings(context.Background(), st, "root")
	if err != nil {
		t.Fatalf("Siblings: %v", err)
	}
	if len(siblings) != 0 {
		t.Fatalf("root should have 0 siblings, got %d", len(siblings))
	}
}

func TestLineageTreeFromGrandchild(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	tree, err := LineageTree(context.Background(), st, "grandchild")
	if err != nil {
		t.Fatalf("LineageTree: %v", err)
	}
	// Tree should contain: root, child_a, child_b, grandchild
	ids := map[model.WorldID]bool{}
	for _, n := range tree {
		ids[n.WorldID] = true
	}
	for _, want := range []model.WorldID{"root", "child_a", "child_b", "grandchild"} {
		if !ids[want] {
			t.Fatalf("missing %q in tree %v", want, tree)
		}
	}
}

func TestLineageTreeFromRoot(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	tree, err := LineageTree(context.Background(), st, "root")
	if err != nil {
		t.Fatalf("LineageTree: %v", err)
	}
	ids := map[model.WorldID]bool{}
	for _, n := range tree {
		ids[n.WorldID] = true
	}
	for _, want := range []model.WorldID{"root", "child_a", "child_b", "grandchild"} {
		if !ids[want] {
			t.Fatalf("missing %q in tree %v", want, tree)
		}
	}
}

func TestChildrenPreservesForkInfo(t *testing.T) {
	t.Parallel()
	st := buildLineageStore(t)
	children, err := Children(context.Background(), st, "root")
	if err != nil {
		t.Fatalf("Children: %v", err)
	}
	for _, c := range children {
		if c.ForkInfo == nil {
			t.Fatalf("child %q missing ForkInfo", c.WorldID)
		}
		if c.ForkInfo.ParentWorldID != "root" {
			t.Fatalf("child %q parent = %q, want root", c.WorldID, c.ForkInfo.ParentWorldID)
		}
	}
}
