package view

import (
	"context"

	"github.com/sizolity/worldline/world/model"
)

// WorldLister can enumerate and load worlds. FileStore satisfies this interface.
type WorldLister interface {
	ListWorlds(ctx context.Context) ([]string, error)
	LoadWorld(ctx context.Context, worldID string) (model.World, error)
}

// LineageNode represents one world in a lineage graph.
type LineageNode struct {
	WorldID  model.WorldID  `json:"world_id"`
	ForkInfo *model.ForkInfo `json:"fork,omitempty"`
}

// Ancestors returns the chain of parent worlds from the given world up to the
// root (a world with no ForkInfo). The first element is the immediate parent;
// the last is the root. Returns an empty slice if the world has no parent.
func Ancestors(ctx context.Context, lister WorldLister, worldID string) ([]LineageNode, error) {
	w, err := lister.LoadWorld(ctx, worldID)
	if err != nil {
		return nil, err
	}
	var chain []LineageNode
	visited := map[model.WorldID]bool{w.ID: true}
	for w.Metadata.Fork != nil {
		parentID := string(w.Metadata.Fork.ParentWorldID)
		if visited[model.WorldID(parentID)] {
			break
		}
		parent, err := lister.LoadWorld(ctx, parentID)
		if err != nil {
			return nil, err
		}
		chain = append(chain, LineageNode{
			WorldID:  parent.ID,
			ForkInfo: parent.Metadata.Fork,
		})
		visited[parent.ID] = true
		w = parent
	}
	return chain, nil
}

// Children returns the worlds whose ForkInfo.ParentWorldID matches the given
// world ID. This requires scanning all worlds.
func Children(ctx context.Context, lister WorldLister, worldID string) ([]LineageNode, error) {
	ids, err := lister.ListWorlds(ctx)
	if err != nil {
		return nil, err
	}
	var children []LineageNode
	for _, id := range ids {
		w, err := lister.LoadWorld(ctx, id)
		if err != nil {
			return nil, err
		}
		if w.Metadata.Fork != nil && string(w.Metadata.Fork.ParentWorldID) == worldID {
			children = append(children, LineageNode{
				WorldID:  w.ID,
				ForkInfo: w.Metadata.Fork,
			})
		}
	}
	if children == nil {
		children = []LineageNode{}
	}
	return children, nil
}

// Siblings returns worlds that share the same parent as the given world,
// excluding the world itself. Returns an empty slice if the world has no parent.
func Siblings(ctx context.Context, lister WorldLister, worldID string) ([]LineageNode, error) {
	w, err := lister.LoadWorld(ctx, worldID)
	if err != nil {
		return nil, err
	}
	if w.Metadata.Fork == nil {
		return []LineageNode{}, nil
	}
	parentID := string(w.Metadata.Fork.ParentWorldID)
	allChildren, err := Children(ctx, lister, parentID)
	if err != nil {
		return nil, err
	}
	var siblings []LineageNode
	for _, child := range allChildren {
		if string(child.WorldID) != worldID {
			siblings = append(siblings, child)
		}
	}
	if siblings == nil {
		siblings = []LineageNode{}
	}
	return siblings, nil
}

// LineageTree returns a flat list of all nodes reachable from the given world:
// ancestors upward and all descendants downward (breadth-first).
func LineageTree(ctx context.Context, lister WorldLister, worldID string) ([]LineageNode, error) {
	w, err := lister.LoadWorld(ctx, worldID)
	if err != nil {
		return nil, err
	}
	root := LineageNode{WorldID: w.ID, ForkInfo: w.Metadata.Fork}

	ancestors, err := Ancestors(ctx, lister, worldID)
	if err != nil {
		return nil, err
	}

	var rootID string
	if len(ancestors) > 0 {
		rootID = string(ancestors[len(ancestors)-1].WorldID)
	} else {
		rootID = worldID
	}

	tree, err := descendants(ctx, lister, rootID)
	if err != nil {
		return nil, err
	}

	hasRoot := false
	for _, n := range tree {
		if string(n.WorldID) == rootID {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		rw, err := lister.LoadWorld(ctx, rootID)
		if err != nil {
			return nil, err
		}
		tree = append([]LineageNode{{WorldID: rw.ID, ForkInfo: rw.Metadata.Fork}}, tree...)
	}

	_ = root
	return tree, nil
}

func descendants(ctx context.Context, lister WorldLister, rootID string) ([]LineageNode, error) {
	queue := []string{rootID}
	visited := map[string]bool{rootID: true}
	var result []LineageNode

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		kids, err := Children(ctx, lister, current)
		if err != nil {
			return nil, err
		}
		for _, kid := range kids {
			id := string(kid.WorldID)
			if visited[id] {
				continue
			}
			visited[id] = true
			result = append(result, kid)
			queue = append(queue, id)
		}
	}
	return result, nil
}
