package mod

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sizolity/worldline/internal/world/model"
)

// LoadScenario reads mod/scenarios/<id>/ from root and returns the
// validated Scenario. Validation enforced:
//
//   - world.md present and parses; `start_location` frontmatter required.
//   - exactly one characters/*.md has `role: player`.
//   - start_location points at an existing locations/<file>.md file.
func LoadScenario(root, id string) (*Scenario, error) {
	if id == "" {
		return nil, fmt.Errorf("scenario id is required")
	}
	dir := filepath.Join(root, "scenarios", id)
	if !isDir(dir) {
		return nil, fmt.Errorf("scenario %q: directory not found at %s", id, dir)
	}

	sc := &Scenario{ID: id, PlayerIndex: -1}

	// world.md
	worldDoc, err := loadDoc(filepath.Join(dir, "world.md"))
	if err != nil {
		return nil, fmt.Errorf("world.md: %w", err)
	}
	startLoc := worldDoc.FrontmatterString("start_location")
	if startLoc == "" {
		return nil, fmt.Errorf("world.md frontmatter: start_location is required")
	}
	sc.World = &WorldDoc{
		Title:         worldDoc.Title,
		Lead:          worldDoc.Lead,
		StartLocation: startLoc,
	}
	if sec := worldDoc.SectionByTitle("类型"); sec != nil {
		sc.World.Genre = splitTokens(sec.Body)
	}
	if sec := worldDoc.SectionByTitle("基调"); sec != nil {
		sc.World.Tone = splitTokens(sec.Body)
	}

	// rules.md (optional, but expected for v1 demo).
	if doc, err := tryLoadDoc(filepath.Join(dir, "rules.md")); err != nil {
		return nil, fmt.Errorf("rules.md: %w", err)
	} else if doc != nil {
		for _, sec := range doc.Sections {
			sc.Rules = append(sc.Rules, RuleEntry{Title: sec.Title, Body: sec.Body})
		}
	}

	// threads.md (optional).
	if doc, err := tryLoadDoc(filepath.Join(dir, "threads.md")); err != nil {
		return nil, fmt.Errorf("threads.md: %w", err)
	} else if doc != nil {
		for _, sec := range doc.Sections {
			sc.Threads = append(sc.Threads, ThreadEntry{Title: sec.Title, Body: sec.Body})
		}
	}

	// characters/*.md
	chars, err := loadDocsInDir(filepath.Join(dir, "characters"))
	if err != nil {
		return nil, fmt.Errorf("characters: %w", err)
	}
	for _, entry := range chars {
		role := entry.doc.FrontmatterString("role")
		if role == "" {
			role = RoleNPC
		}
		sc.Characters = append(sc.Characters, CharacterDoc{
			FileSlug: entry.slug,
			Role:     role,
			Title:    entry.doc.Title,
			Body:     documentBody(entry.doc),
		})
	}

	// Validate exactly one player.
	playerCount := 0
	for i, ch := range sc.Characters {
		if ch.Role == RolePlayer {
			playerCount++
			sc.PlayerIndex = i
		}
	}
	if playerCount != 1 {
		return nil, fmt.Errorf("scenario %q: expected exactly one character with `role: player`, found %d", id, playerCount)
	}

	// locations/*.md
	locs, err := loadDocsInDir(filepath.Join(dir, "locations"))
	if err != nil {
		return nil, fmt.Errorf("locations: %w", err)
	}
	startLocFound := false
	for _, entry := range locs {
		if entry.slug == startLoc {
			startLocFound = true
		}
		sc.Locations = append(sc.Locations, LocationDoc{
			FileSlug: entry.slug,
			Title:    entry.doc.Title,
			Body:     documentBody(entry.doc),
		})
	}
	if !startLocFound {
		return nil, fmt.Errorf("scenario %q: start_location %q has no matching locations/<file>.md", id, startLoc)
	}

	// events/*.md (optional).
	evs, err := loadDocsInDir(filepath.Join(dir, "events"))
	if err != nil {
		return nil, fmt.Errorf("events: %w", err)
	}
	for _, entry := range evs {
		sc.Events = append(sc.Events, EventDoc{
			FileSlug: entry.slug,
			Title:    entry.doc.Title,
			Body:     documentBody(entry.doc),
		})
	}

	// worldlines/*.md (optional, opt-in mechanic layer). A missing
	// directory yields no lines — CompileScenarioToWorldLines then returns
	// nil and the scenario degrades to "no hidden thread engine".
	wls, err := loadDocsInDir(filepath.Join(dir, "worldlines"))
	if err != nil {
		return nil, fmt.Errorf("worldlines: %w", err)
	}
	for _, entry := range wls {
		wl, err := parseWorldLineDoc(entry.slug, entry.doc)
		if err != nil {
			return nil, fmt.Errorf("worldlines: %w", err)
		}
		sc.WorldLines = append(sc.WorldLines, wl)
	}

	// Sanity check derived entity IDs at load-time so a malformed slug
	// fails loud here rather than deep inside the runtime validator.
	if err := sanityCheckIDs(sc); err != nil {
		return nil, err
	}
	return sc, nil
}

// LoadStyle reads mod/styles/<id>/ from root and returns the validated
// Style. Validation: persona/narrator.md must be present.
func LoadStyle(root, id string) (*Style, error) {
	if id == "" {
		return nil, fmt.Errorf("style id is required")
	}
	dir := filepath.Join(root, "styles", id)
	if !isDir(dir) {
		return nil, fmt.Errorf("style %q: directory not found at %s", id, dir)
	}

	st := &Style{ID: id}

	narrator, err := loadDoc(filepath.Join(dir, "persona", "narrator.md"))
	if err != nil {
		return nil, fmt.Errorf("persona/narrator.md: %w", err)
	}
	st.NarratorPersona = narrator

	if doc, err := tryLoadDoc(filepath.Join(dir, "persona", "lorekeeper.md")); err != nil {
		return nil, fmt.Errorf("persona/lorekeeper.md: %w", err)
	} else if doc != nil {
		st.LorekeeperPersona = doc
	}

	if doc, err := tryLoadDoc(filepath.Join(dir, "persona", "action_suggester.md")); err != nil {
		return nil, fmt.Errorf("persona/action_suggester.md: %w", err)
	} else if doc != nil {
		st.SuggesterPersona = doc
	}

	if doc, err := tryLoadDoc(filepath.Join(dir, "persona", "intent.md")); err != nil {
		return nil, fmt.Errorf("persona/intent.md: %w", err)
	} else if doc != nil {
		st.IntentPersona = doc
	}

	if doc, err := tryLoadDoc(filepath.Join(dir, "scene", "prologue.md")); err != nil {
		return nil, fmt.Errorf("scene/prologue.md: %w", err)
	} else if doc != nil {
		st.ProloguePrompt = sceneBody(doc)
	}
	if doc, err := tryLoadDoc(filepath.Join(dir, "scene", "recap.md")); err != nil {
		return nil, fmt.Errorf("scene/recap.md: %w", err)
	} else if doc != nil {
		st.RecapPrompt = sceneBody(doc)
	}
	return st, nil
}

type dirEntry struct {
	slug string
	doc  *Document
}

func loadDocsInDir(dir string) ([]dirEntry, error) {
	if !isDir(dir) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]dirEntry, 0, len(names))
	for _, name := range names {
		doc, err := loadDoc(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		slug := strings.TrimSuffix(name, ".md")
		out = append(out, dirEntry{slug: slug, doc: doc})
	}
	return out, nil
}

func loadDoc(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseDocument(string(data))
}

func tryLoadDoc(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return ParseDocument(string(data))
}

// documentBody re-flows lead + H2 sections into a single descriptive
// blob, preserving section headers so structured prose (## 性格 / ## 口吻
// / etc.) carries into the prompt and lorekeeper memory.
func documentBody(d *Document) string {
	var b strings.Builder
	if d.Lead != "" {
		b.WriteString(d.Lead)
	}
	for _, sec := range d.Sections {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## ")
		b.WriteString(sec.Title)
		if sec.Body != "" {
			b.WriteString("\n")
			b.WriteString(sec.Body)
		}
	}
	return strings.TrimSpace(b.String())
}

// sceneBody returns the lead + sections of a scene markdown as a single
// "instruction blob" — the same format the CLI used to inline as a hard
// string. Authors may write multiple H2s; engine just concats.
func sceneBody(d *Document) string {
	return documentBody(d)
}

// splitTokens turns "中国神话、西游记、古典奇幻" into ["中国神话", "西游记",
// "古典奇幻"]. Accepts both Chinese and ASCII delimiters: ，、,/;.
func splitTokens(s string) []string {
	if s == "" {
		return nil
	}
	// Replace all common delimiters with a single sentinel, then split.
	for _, sep := range []string{"，", "、", "/", ";", ";", "\n"} {
		s = strings.ReplaceAll(s, sep, ",")
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimSuffix(p, "。")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// sanityCheckIDs verifies every derived entity / thread / rule / event
// ID round-trips through model.ValidateID. We catch this here so a
// malformed slug (rare for ASCII filenames, common for someone who
// names a character file 中文.md) fails loud at load.
func sanityCheckIDs(sc *Scenario) error {
	for _, ch := range sc.Characters {
		id := characterEntityID(ch)
		if err := model.ValidateID(id); err != nil {
			return fmt.Errorf("character %q: derived id %q is invalid: %w", ch.FileSlug, id, err)
		}
	}
	for _, l := range sc.Locations {
		id := locationEntityID(l)
		if err := model.ValidateID(id); err != nil {
			return fmt.Errorf("location %q: derived id %q is invalid: %w", l.FileSlug, id, err)
		}
	}
	for _, e := range sc.Events {
		id := eventEntityID(e)
		if err := model.ValidateID(id); err != nil {
			return fmt.Errorf("event %q: derived id %q is invalid: %w", e.FileSlug, id, err)
		}
	}
	for _, wl := range sc.WorldLines {
		id := worldLineID(wl)
		if err := model.ValidateID(id); err != nil {
			return fmt.Errorf("worldline %q: derived id %q is invalid: %w", wl.FileSlug, id, err)
		}
	}
	return nil
}
