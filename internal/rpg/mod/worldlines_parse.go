package mod

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// parseWorldLineDoc turns a parsed worldlines/<slug>.md Document into the
// author-facing WorldLineDoc. It extracts human handles only (thread
// title, band keyword, target names / words); the numeric drift, tension
// thresholds and engine IDs are resolved later by
// CompileScenarioToWorldLines. Numbers never appear in mod files.
func parseWorldLineDoc(slug string, doc *Document) (WorldLineDoc, error) {
	wl := WorldLineDoc{
		FileSlug:   slug,
		Title:      doc.Title,
		ThreadName: strings.TrimSpace(doc.FrontmatterString("thread")),
		Visibility: strings.TrimSpace(doc.FrontmatterString("visibility")),
		Stage:      strings.TrimSpace(doc.FrontmatterString("stage")),
		Tempo:      strings.TrimSpace(doc.FrontmatterString("tempo")),
	}
	if wl.ThreadName == "" {
		return WorldLineDoc{}, fmt.Errorf("worldline %q: frontmatter `thread` is required", slug)
	}
	for _, sec := range doc.Sections {
		ms, err := parseMilestoneSection(sec)
		if err != nil {
			return WorldLineDoc{}, fmt.Errorf("worldline %q: milestone %q: %w", slug, sec.Title, err)
		}
		wl.Milestones = append(wl.Milestones, ms)
	}
	return wl, nil
}

// parseMilestoneSection reads one H2 block. Its body is expected to carry
// a `触发：<...>·<band>` line and a `结果：` list of `- 目标：词` bullets.
// Parsing is intentionally line-oriented since the shared markdown parser
// is flat (it does not understand H3 / nested lists).
func parseMilestoneSection(sec Section) (MilestoneSpec, error) {
	ms := MilestoneSpec{Title: sec.Title}
	for _, raw := range strings.Split(sec.Body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "触发"):
			band := parseTriggerBand(line)
			if band == "" {
				return MilestoneSpec{}, fmt.Errorf("触发 line has no band keyword: %q", line)
			}
			ms.Band = band
		case isBullet(line):
			target, word, ok := splitColon(stripBulletMarker(line))
			if !ok {
				return MilestoneSpec{}, fmt.Errorf("结果 bullet must be `- 目标：词`, got %q", line)
			}
			if target == "" || word == "" {
				return MilestoneSpec{}, fmt.Errorf("结果 bullet has empty target or word: %q", line)
			}
			ms.Outcomes = append(ms.Outcomes, OutcomeSpec{Target: target, Word: word})
		default:
			// 结果 label line (and any free prose) is descriptive; skip.
		}
	}
	if ms.Band == "" {
		return MilestoneSpec{}, fmt.Errorf("missing 触发 line")
	}
	return ms, nil
}

// parseTriggerBand extracts the trailing band keyword from a 触发 line.
// "触发：师徒张力·初现" → "初现". The descriptive prefix before the
// interpunct ("师徒张力") and the 触发 label are discarded. Robust to the
// keyword appearing alone with no interpunct.
func parseTriggerBand(line string) string {
	_, rest, ok := splitColon(line)
	if !ok {
		return ""
	}
	return trailingToken(rest)
}

// midDots are the interpunct runes an author might type between the
// descriptive prefix and the band keyword. We split on any of them.
const midDots = "·・‧•"

// trailingToken returns the substring after the last interpunct rune,
// trimmed; if none is present it returns the whole trimmed string.
func trailingToken(s string) string {
	if idx := strings.LastIndexAny(s, midDots); idx >= 0 {
		_, size := utf8.DecodeRuneInString(s[idx:])
		s = s[idx+size:]
	}
	return strings.TrimSpace(s)
}

func isBullet(line string) bool {
	return strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "・")
}

func stripBulletMarker(line string) string {
	line = strings.TrimSpace(line)
	for _, m := range []string{"-", "*", "・"} {
		if strings.HasPrefix(line, m) {
			return strings.TrimSpace(line[len(m):])
		}
	}
	return line
}

// splitColon splits on the first ASCII ':' or full-width '：', returning
// the trimmed left and right halves.
func splitColon(s string) (left, right string, ok bool) {
	for _, sep := range []string{"：", ":"} {
		if idx := strings.Index(s, sep); idx >= 0 {
			return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+len(sep):]), true
		}
	}
	return "", "", false
}
