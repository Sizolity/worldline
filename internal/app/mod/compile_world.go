package mod

import (
	"fmt"

	"github.com/sizolity/worldline/rpg/rule"
	worldmodel "github.com/sizolity/worldline/world/model"
)

// CompileScenarioToWorld turns a loaded mod Scenario into the runtime
// worldmodel.World. Per v1 directives (zero numerics / zero rulebooks /
// zero visible mechanics) entities ship with empty State maps and rules
// carry narrative content only — no d20 / HP / AC anywhere.
//
// Entity ID derivation (directive 2.8):
//
//	player character     → hero-<file_slug>
//	npc character        → npc-<file_slug>
//	location             → loc-<file_slug>
//	event                → evt-<file_slug>
//	threads.md H2 (Nth)  → thread-<N>
//	rules.md H2 (Nth)    → rule-<N>
//
// worldID is the persisted world identifier. Callers commonly pass the
// scenario ID through unchanged so the snapshot lives under
// worlds/<scenario>/.
func CompileScenarioToWorld(sc *Scenario, worldID string) (worldmodel.World, error) {
	if sc == nil {
		return worldmodel.World{}, fmt.Errorf("scenario is nil")
	}
	if worldID == "" {
		worldID = sc.ID
	}

	entities := map[worldmodel.EntityID]worldmodel.Entity{}
	for _, ch := range sc.Characters {
		id := characterEntityID(ch)
		tags := []string{}
		if ch.Role == RolePlayer {
			tags = append(tags, "player")
		}
		entities[worldmodel.EntityID(id)] = worldmodel.Entity{
			ID:          worldmodel.EntityID(id),
			Type:        "character",
			Name:        nameOrSlug(ch.Title, ch.FileSlug),
			Description: ch.Body,
			Tags:        tags,
		}
	}
	for _, loc := range sc.Locations {
		id := locationEntityID(loc)
		entities[worldmodel.EntityID(id)] = worldmodel.Entity{
			ID:          worldmodel.EntityID(id),
			Type:        "location",
			Name:        nameOrSlug(loc.Title, loc.FileSlug),
			Description: loc.Body,
		}
	}

	threads := make([]worldmodel.WorldThread, 0, len(sc.Threads))
	for i, th := range sc.Threads {
		id := threadID(i)
		status := worldmodel.ThreadStatusOpen
		if i == 0 {
			// Mirror the legacy demo: the first declared thread starts
			// active so SuggestActions has something to anchor on at
			// beat 0.
			status = worldmodel.ThreadStatusActive
		}
		threads = append(threads, worldmodel.WorldThread{
			ID:      worldmodel.ThreadID(id),
			Kind:    worldmodel.ThreadKindQuest,
			Title:   th.Title,
			Summary: th.Body,
			Status:  status,
		})
	}

	rules := make([]worldmodel.Rule, 0, len(sc.Rules))
	for i, r := range sc.Rules {
		rules = append(rules, rule.ToModelRule(rule.Rule{
			ID:      worldmodel.RuleID(ruleID(i)),
			Category: "general",
			Level:    0,
			Content:  r.Body,
			Source:   rule.SourceSystem,
			Enabled:  true,
			Tags:     []string{},
		}))
	}

	events := make([]worldmodel.WorldEvent, 0, len(sc.Events))
	for _, e := range sc.Events {
		id := eventEntityID(e)
		events = append(events, worldmodel.WorldEvent{
			ID:          worldmodel.EventID(id),
			Type:        worldmodel.EventTypeNote,
			Source:      worldmodel.EventSourceUser,
			Description: e.Body,
		})
	}

	description := sc.World.Lead
	canon := worldmodel.Canon{
		Genre: sc.World.Genre,
		Tone:  sc.World.Tone,
	}

	return worldmodel.World{
		ID:          worldmodel.WorldID(worldID),
		Name:        sc.World.Title,
		Description: description,
		Canon:       canon,
		Entities:    entities,
		Threads:     threads,
		Rules:       rules,
		EventLog:    events,
		Clock: worldmodel.WorldClock{
			Current:  worldmodel.WorldTime{Kind: worldmodel.WorldTimeScene, Tick: 1},
			Sequence: 1,
		},
		Metadata: worldmodel.WorldMetadata{
			Source: "mod:" + sc.ID,
		},
	}, nil
}

// --- ID derivation helpers ---

func characterEntityID(ch CharacterDoc) string {
	switch ch.Role {
	case RolePlayer:
		return "hero-" + ch.FileSlug
	default:
		return "npc-" + ch.FileSlug
	}
}

func locationEntityID(loc LocationDoc) string {
	return "loc-" + loc.FileSlug
}

func eventEntityID(e EventDoc) string {
	return "evt-" + e.FileSlug
}

// threadID returns the position-based thread slug. Directive 2.8 fixes
// "thread-<N>" (1-indexed) so non-ASCII titles never need pinyin
// transliteration to satisfy model.ValidateID.
func threadID(zeroIndex int) string {
	return fmt.Sprintf("thread-%d", zeroIndex+1)
}

func ruleID(zeroIndex int) string {
	return fmt.Sprintf("rule-%d", zeroIndex+1)
}

// PlayerEntityID returns the hero-<slug> ID for sc.Characters[sc.PlayerIndex].
// Returns "" if the scenario has no player (Load rejects this so callers
// in normal flow can assume non-empty).
func PlayerEntityID(sc *Scenario) string {
	if sc == nil || sc.PlayerIndex < 0 || sc.PlayerIndex >= len(sc.Characters) {
		return ""
	}
	return characterEntityID(sc.Characters[sc.PlayerIndex])
}

// PlayerCharacterName returns the H1 title of the player character entry.
func PlayerCharacterName(sc *Scenario) string {
	if sc == nil || sc.PlayerIndex < 0 || sc.PlayerIndex >= len(sc.Characters) {
		return ""
	}
	ch := sc.Characters[sc.PlayerIndex]
	return nameOrSlug(ch.Title, ch.FileSlug)
}

func nameOrSlug(title, slug string) string {
	if title != "" {
		return title
	}
	return slug
}
