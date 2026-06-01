package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sizolity/worldline/internal/world/director"
	"github.com/sizolity/worldline/internal/world/model"
)

// ConflictResolver resolves a single merge conflict.
type ConflictResolver interface {
	Resolve(ctx context.Context, input ConflictInput) (Resolution, error)
}

// ConflictInput provides context for resolving a single conflict.
type ConflictInput struct {
	Conflict MergeConflict `json:"conflict"`

	// For entity conflicts: base, source, and target versions of the entity.
	BaseEntity   *model.Entity `json:"base_entity,omitempty"`
	SourceEntity *model.Entity `json:"source_entity,omitempty"`
	TargetEntity *model.Entity `json:"target_entity,omitempty"`

	// For thread conflicts: base, source, and target versions of the thread.
	BaseThread   *model.WorldThread `json:"base_thread,omitempty"`
	SourceThread *model.WorldThread `json:"source_thread,omitempty"`
	TargetThread *model.WorldThread `json:"target_thread,omitempty"`
}

// Resolution is the LLM's decision for a conflict.
type Resolution struct {
	Pick   string            `json:"pick"`
	Entity *model.Entity     `json:"entity,omitempty"`
	Thread *model.WorldThread `json:"thread,omitempty"`
	Reason string            `json:"reason"`
}

const (
	PickSource = "source"
	PickTarget = "target"
	PickMerged = "merged"
)

// TextGenerator abstracts LLM calls for conflict resolution.
type TextGenerator interface {
	Generate(ctx context.Context, system, user string) (string, error)
}

const resolverSystemPrompt = `You are a world-state merge conflict resolver for an interactive narrative system.

You receive a conflict between two branches of a world timeline. Each conflict has:
- The base version (common ancestor before the branches diverged)
- The source version (the branch being merged in)
- The target version (the branch being merged into)

Your job: decide how to resolve the conflict. Return JSON:

For entity conflicts:
{"pick": "source"|"target"|"merged", "reason": "brief explanation", "entity": {only if pick=merged, the merged entity}}

For thread conflicts:
{"pick": "source"|"target"|"merged", "reason": "brief explanation", "thread": {only if pick=merged, the merged thread}}

Guidelines:
- Prefer "source" or "target" when one clearly supersedes the other.
- Use "merged" only when both branches made valuable changes that can coexist.
- If an entity was removed in source but modified in target, consider whether the modification makes removal inappropriate.
- For thread status conflicts, consider narrative progression (e.g. "resolved" > "active" > "open").
- Keep the reason concise (one sentence).
- Return ONLY valid JSON. No markdown, no explanation outside JSON.`

// LLMConflictResolver uses an LLM to resolve merge conflicts.
type LLMConflictResolver struct {
	gen TextGenerator
}

func NewLLMConflictResolver(gen TextGenerator) *LLMConflictResolver {
	return &LLMConflictResolver{gen: gen}
}

func (r *LLMConflictResolver) Resolve(ctx context.Context, input ConflictInput) (Resolution, error) {
	userPrompt, err := json.Marshal(input)
	if err != nil {
		return Resolution{}, fmt.Errorf("marshal conflict input: %w", err)
	}
	response, err := r.gen.Generate(ctx, resolverSystemPrompt, string(userPrompt))
	if err != nil {
		return Resolution{}, fmt.Errorf("llm resolve: %w", err)
	}
	response = director.StripMarkdownFences(response)

	var res Resolution
	if err := json.Unmarshal([]byte(response), &res); err != nil {
		return Resolution{}, fmt.Errorf("parse resolution: %w", err)
	}
	switch res.Pick {
	case PickSource, PickTarget, PickMerged:
	default:
		return Resolution{}, fmt.Errorf("invalid pick %q (must be source, target, or merged)", res.Pick)
	}
	return res, nil
}

// ResolveMergeConflicts takes a merged world with unresolved conflicts, resolves
// each conflict using the resolver, applies the resolutions, and returns the
// updated world along with applied resolutions.
func ResolveMergeConflicts(ctx context.Context, merged model.World, base, source, target model.World, conflicts []MergeConflict, resolver ConflictResolver) (model.World, []Resolution, error) {
	out := merged.Clone()
	resolutions := make([]Resolution, 0, len(conflicts))

	for _, conflict := range conflicts {
		input := buildConflictInput(conflict, base, source, target)
		res, err := resolver.Resolve(ctx, input)
		if err != nil {
			return out, resolutions, fmt.Errorf("resolve %s %s: %w", conflict.Kind, conflict.ID, err)
		}
		applyResolution(&out, conflict, res, source, target)
		resolutions = append(resolutions, res)
	}

	return out, resolutions, nil
}

func buildConflictInput(c MergeConflict, base, source, target model.World) ConflictInput {
	input := ConflictInput{Conflict: c}

	switch c.Kind {
	case "entity":
		eid := model.EntityID(c.ID)
		if e, ok := base.Entities[eid]; ok {
			input.BaseEntity = &e
		}
		if e, ok := source.Entities[eid]; ok {
			input.SourceEntity = &e
		}
		if e, ok := target.Entities[eid]; ok {
			input.TargetEntity = &e
		}
	case "thread":
		for i := range base.Threads {
			if string(base.Threads[i].ID) == c.ID {
				input.BaseThread = &base.Threads[i]
				break
			}
		}
		for i := range source.Threads {
			if string(source.Threads[i].ID) == c.ID {
				input.SourceThread = &source.Threads[i]
				break
			}
		}
		for i := range target.Threads {
			if string(target.Threads[i].ID) == c.ID {
				input.TargetThread = &target.Threads[i]
				break
			}
		}
	}

	return input
}

func applyResolution(out *model.World, conflict MergeConflict, res Resolution, source, target model.World) {
	switch conflict.Kind {
	case "entity":
		eid := model.EntityID(conflict.ID)
		switch res.Pick {
		case PickSource:
			if e, ok := source.Entities[eid]; ok {
				out.Entities[eid] = e
			} else {
				delete(out.Entities, eid)
			}
		case PickTarget:
			// already has target version
		case PickMerged:
			if res.Entity != nil {
				out.Entities[eid] = *res.Entity
			}
		}
	case "thread":
		tid := model.ThreadID(conflict.ID)
		switch res.Pick {
		case PickSource:
			for i := range source.Threads {
				if source.Threads[i].ID == tid {
					for j := range out.Threads {
						if out.Threads[j].ID == tid {
							out.Threads[j] = source.Threads[i]
							return
						}
					}
					out.Threads = append(out.Threads, source.Threads[i])
					return
				}
			}
			filtered := out.Threads[:0]
			for _, t := range out.Threads {
				if t.ID != tid {
					filtered = append(filtered, t)
				}
			}
			out.Threads = filtered
		case PickTarget:
			// already has target version
		case PickMerged:
			if res.Thread != nil {
				for j := range out.Threads {
					if out.Threads[j].ID == tid {
						out.Threads[j] = *res.Thread
						return
					}
				}
				out.Threads = append(out.Threads, *res.Thread)
			}
		}
	}
}

