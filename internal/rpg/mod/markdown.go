package mod

import (
	"bufio"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Document is the parsed in-memory representation of a single mod markdown
// file. v1 uses a deliberately narrow markdown convention:
//
//   - Optional YAML frontmatter between `---` fences at top of file.
//   - Optional single H1 line as title.
//   - A "lead" paragraph: any prose between the H1 and the first H2.
//   - Zero or more H2 sections, each with title + body.
//
// This is not a full CommonMark AST. Anything fancier than the above
// (nested headings, embedded HTML, complex tables) is left in the
// section body as plain text.
type Document struct {
	Frontmatter map[string]any
	Title       string    // H1 text without leading "# " marker; empty if no H1
	Lead        string    // prose between H1 and first H2 (trimmed)
	Sections    []Section // H2 sections in source order
}

// Section is one H2 block: title plus body prose (trimmed).
type Section struct {
	Title string
	Body  string
}

// SectionByTitle returns the first section whose Title matches name, or
// nil if none.
func (d *Document) SectionByTitle(name string) *Section {
	if d == nil {
		return nil
	}
	for i := range d.Sections {
		if d.Sections[i].Title == name {
			return &d.Sections[i]
		}
	}
	return nil
}

// ParseDocument parses src into a Document. Errors only on malformed
// frontmatter; missing frontmatter, missing H1, or zero sections are all
// valid.
func ParseDocument(src string) (*Document, error) {
	doc := &Document{}
	scanner := bufio.NewScanner(strings.NewReader(src))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read markdown: %w", err)
	}

	idx := 0

	// Optional frontmatter.
	if idx < len(lines) && strings.TrimSpace(lines[idx]) == "---" {
		fmStart := idx + 1
		fmEnd := -1
		for j := fmStart; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "---" {
				fmEnd = j
				break
			}
		}
		if fmEnd < 0 {
			return nil, fmt.Errorf("frontmatter not terminated by `---`")
		}
		fmText := strings.Join(lines[fmStart:fmEnd], "\n")
		if strings.TrimSpace(fmText) != "" {
			fm := map[string]any{}
			if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
				return nil, fmt.Errorf("frontmatter yaml: %w", err)
			}
			doc.Frontmatter = fm
		}
		idx = fmEnd + 1
	}

	// Skip leading blank lines after frontmatter.
	for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
		idx++
	}

	// Optional H1.
	if idx < len(lines) && strings.HasPrefix(lines[idx], "# ") {
		doc.Title = strings.TrimSpace(strings.TrimPrefix(lines[idx], "# "))
		idx++
	}

	// Walk remaining lines collecting Lead (until first H2) then sections.
	var lead strings.Builder
	var cur *Section
	flush := func() {
		if cur != nil {
			doc.Sections = append(doc.Sections, Section{
				Title: cur.Title,
				Body:  strings.TrimSpace(cur.Body),
			})
			cur = nil
		}
	}
	for ; idx < len(lines); idx++ {
		line := lines[idx]
		if strings.HasPrefix(line, "## ") {
			flush()
			cur = &Section{Title: strings.TrimSpace(strings.TrimPrefix(line, "## "))}
			continue
		}
		if cur != nil {
			cur.Body += line + "\n"
		} else {
			lead.WriteString(line)
			lead.WriteString("\n")
		}
	}
	flush()
	doc.Lead = strings.TrimSpace(lead.String())
	return doc, nil
}

// FrontmatterString returns a string field from frontmatter, or "" if
// the key is missing or not a string.
func (d *Document) FrontmatterString(key string) string {
	if d == nil || d.Frontmatter == nil {
		return ""
	}
	v, ok := d.Frontmatter[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
