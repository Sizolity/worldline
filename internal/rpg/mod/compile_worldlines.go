package mod

import (
	"fmt"

	"github.com/sizolity/worldline/internal/rpg/story"
	worldmodel "github.com/sizolity/worldline/internal/world/model"
)

// Keyword tables (directive 3): the v1 zero-number redline means authors
// write evocative words in mod files; all numerics live here in Go. These
// are seeded with the legacy xiyou-changan values so the migration is
// behaviour-preserving. A future mechanics/ override file may layer on top,
// but v1 keeps the vocabulary fixed in code.

// tempoTable maps a drift "tempo" word to its per-time-scale tension
// deltas. An omitted/unknown tempo yields a zero Drift (no drift).
//
//	渐磨 == the legacy xiyou wl_shitu drift {0.05, 0.20, 0.40}.
var tempoTable = map[string]story.Drift{
	"微澜": {Scene: 0.02, Day: 0.10, Chapter: 0.20},
	"渐磨": {Scene: 0.05, Day: 0.20, Chapter: 0.40},
	"紧迫": {Scene: 0.10, Day: 0.30, Chapter: 0.60},
}

// bandTable maps a tension "band" word (used in a 触发 line) to the
// thread_tension_gte threshold it represents.
var bandTable = map[string]float64{
	"初现": 0.30,
	"加深": 0.45,
	"决裂": 0.60,
	"临界": 0.80,
}

// statusTable maps a thread "status" word (used in a 结果 bullet whose
// target is a thread) to a model.ThreadStatus constant.
var statusTable = map[string]string{
	"浮上台面": worldmodel.ThreadStatusActive,
	"开启":   worldmodel.ThreadStatusOpen,
	"潜伏":   worldmodel.ThreadStatusDormant,
	"了结":   worldmodel.ThreadStatusResolved,
	"破裂":   worldmodel.ThreadStatusFailed,
	"搁置":   worldmodel.ThreadStatusAbandoned,
}

// worldLineID derives the WorldLine.ID from the file slug, mirroring the
// other positional/slug derivations in compile_world.go (e.g. shitu.md →
// wl_shitu).
func worldLineID(wl WorldLineDoc) string {
	return "wl_" + wl.FileSlug
}

// CompileScenarioToWorldLines builds the hidden worldlines for a scenario
// from its worldlines/*.md files (sc.WorldLines). Each doc carries only
// author handles (thread title, character names, band/status/tempo words);
// this step resolves them into the engine's IDs and numeric drift /
// thresholds via the Go keyword tables and the same ID derivation used by
// CompileScenarioToWorld.
//
// A scenario with no worldlines/ dir yields nil — story.Tick treats that as
// "no lines" and is a no-op, so the seed/play pipeline degrades gracefully
// to "demo without a hidden thread engine".
//
// Resolution is fail-loud: an unresolvable or ambiguous thread title /
// character name, or an unknown keyword, returns an error rather than
// silently dropping the effect.
func CompileScenarioToWorldLines(sc *Scenario) ([]story.WorldLine, error) {
	if sc == nil || len(sc.WorldLines) == 0 {
		return nil, nil
	}

	threadByTitle, threadAmbiguous := indexThreadTitles(sc.Threads)
	entityByName, entityAmbiguous := indexCharacterNames(sc.Characters)
	idx := refIndex{
		threadByTitle:   threadByTitle,
		threadAmbiguous: threadAmbiguous,
		entityByName:    entityByName,
		entityAmbiguous: entityAmbiguous,
	}

	lines := make([]story.WorldLine, 0, len(sc.WorldLines))
	for _, doc := range sc.WorldLines {
		line, err := compileWorldLine(doc, idx)
		if err != nil {
			return nil, fmt.Errorf("worldline %q: %w", doc.FileSlug, err)
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// refIndex holds the display-name → derived-ID lookups plus the ambiguity
// sets used to fail loud on duplicate titles / names.
type refIndex struct {
	threadByTitle   map[string]string
	threadAmbiguous map[string]bool
	entityByName    map[string]string
	entityAmbiguous map[string]bool
}

func compileWorldLine(doc WorldLineDoc, idx refIndex) (story.WorldLine, error) {
	threadID, err := idx.resolveThread(doc.ThreadName)
	if err != nil {
		return story.WorldLine{}, err
	}

	visibility := story.VisibilityHidden
	if doc.Visibility != "" {
		visibility = story.Visibility(doc.Visibility)
		if !visibility.IsValid() {
			return story.WorldLine{}, fmt.Errorf("visibility %q is invalid", doc.Visibility)
		}
	}

	drift := story.Drift{}
	if doc.Tempo != "" {
		d, ok := tempoTable[doc.Tempo]
		if !ok {
			return story.WorldLine{}, fmt.Errorf("unknown tempo %q", doc.Tempo)
		}
		drift = d
	}

	line := story.WorldLine{
		ID:           worldLineID(doc),
		ThreadID:     worldmodel.ThreadID(threadID),
		Visibility:   visibility,
		CurrentStage: doc.Stage,
		Drift:        drift,
	}

	for i, ms := range doc.Milestones {
		threshold, ok := bandTable[ms.Band]
		if !ok {
			return story.WorldLine{}, fmt.Errorf("milestone %q: unknown tension band %q", ms.Title, ms.Band)
		}
		milestone := story.Milestone{
			// Positional ID (directive 2): m1, m2, ... 1-indexed by H2 order.
			ID: fmt.Sprintf("m%d", i+1),
			Condition: story.MilestoneCondition{
				Kind: story.CondThreadTensionGTE,
				// thread_id defaults to the frontmatter-bound thread; the
				// author never repeats it in the 触发 line.
				Args: map[string]any{"thread_id": threadID, "threshold": threshold},
			},
		}
		for _, oc := range ms.Outcomes {
			eff, err := idx.resolveOutcome(oc)
			if err != nil {
				return story.WorldLine{}, fmt.Errorf("milestone %q: %w", ms.Title, err)
			}
			milestone.Effects = append(milestone.Effects, eff)
		}
		line.Milestones = append(line.Milestones, milestone)
	}

	if err := line.Validate(); err != nil {
		return story.WorldLine{}, err
	}
	return line, nil
}

// resolveOutcome maps a `- 目标：词` bullet to an Effect. A target matching
// a threads.md title becomes an update_thread{status} (word → statusTable);
// otherwise it resolves as a character name to an update_entity_state with
// the word stored under the default "disposition" key.
func (idx refIndex) resolveOutcome(oc OutcomeSpec) (worldmodel.Effect, error) {
	if idx.threadAmbiguous[oc.Target] {
		return worldmodel.Effect{}, fmt.Errorf("target %q is an ambiguous thread title (multiple threads.md H2 share it)", oc.Target)
	}
	if tid, ok := idx.threadByTitle[oc.Target]; ok {
		status, ok := statusTable[oc.Word]
		if !ok {
			return worldmodel.Effect{}, fmt.Errorf("unknown thread status word %q for thread %q", oc.Word, oc.Target)
		}
		return worldmodel.Effect{
			Kind:     worldmodel.EffectUpdateThread,
			TargetID: tid,
			Payload: map[string]worldmodel.Value{
				"status": {Kind: worldmodel.ValueKindString, Raw: status},
			},
		}, nil
	}

	if idx.entityAmbiguous[oc.Target] {
		return worldmodel.Effect{}, fmt.Errorf("target %q is an ambiguous character name (multiple characters share it)", oc.Target)
	}
	if eid, ok := idx.entityByName[oc.Target]; ok {
		return worldmodel.Effect{
			Kind:     worldmodel.EffectUpdateEntityState,
			TargetID: eid,
			Payload: map[string]worldmodel.Value{
				"disposition": {Kind: worldmodel.ValueKindString, Raw: oc.Word},
			},
		}, nil
	}

	return worldmodel.Effect{}, fmt.Errorf("target %q resolves to neither a threads.md title nor a character name", oc.Target)
}

func (idx refIndex) resolveThread(title string) (string, error) {
	if idx.threadAmbiguous[title] {
		return "", fmt.Errorf("thread %q is ambiguous (multiple threads.md H2 share this title)", title)
	}
	id, ok := idx.threadByTitle[title]
	if !ok {
		return "", fmt.Errorf("thread %q not found in threads.md", title)
	}
	return id, nil
}

// indexThreadTitles maps each threads.md H2 title to its positional
// thread-N ID (same derivation as CompileScenarioToWorld). Titles shared
// by more than one thread are recorded as ambiguous so references to them
// fail loud rather than silently binding to the first.
func indexThreadTitles(threads []ThreadEntry) (byTitle map[string]string, ambiguous map[string]bool) {
	byTitle = make(map[string]string, len(threads))
	ambiguous = map[string]bool{}
	for i, th := range threads {
		if _, seen := byTitle[th.Title]; seen {
			ambiguous[th.Title] = true
			continue
		}
		byTitle[th.Title] = threadID(i)
	}
	return byTitle, ambiguous
}

// indexCharacterNames maps each character display name (H1 title, else file
// slug) to its derived entity ID. Duplicate names are flagged ambiguous.
func indexCharacterNames(chars []CharacterDoc) (byName map[string]string, ambiguous map[string]bool) {
	byName = make(map[string]string, len(chars))
	ambiguous = map[string]bool{}
	for _, ch := range chars {
		name := nameOrSlug(ch.Title, ch.FileSlug)
		if _, seen := byName[name]; seen {
			ambiguous[name] = true
			continue
		}
		byName[name] = characterEntityID(ch)
	}
	return byName, ambiguous
}
