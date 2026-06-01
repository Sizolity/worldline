package mod

import (
	"strings"
	"testing"
)

func TestParseDocument_FrontmatterAndH1(t *testing.T) {
	src := `---
role: player
start_location: changan_gate
---

# 孙悟空

齐天大圣，火眼金睛。

## 性格

烈、傲、孝。
`
	doc, err := ParseDocument(src)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if got := doc.FrontmatterString("role"); got != "player" {
		t.Errorf("role = %q, want %q", got, "player")
	}
	if got := doc.FrontmatterString("start_location"); got != "changan_gate" {
		t.Errorf("start_location = %q, want %q", got, "changan_gate")
	}
	if doc.Title != "孙悟空" {
		t.Errorf("title = %q, want 孙悟空", doc.Title)
	}
	if !strings.Contains(doc.Lead, "齐天大圣") {
		t.Errorf("lead missing canonical prose: %q", doc.Lead)
	}
	if len(doc.Sections) != 1 || doc.Sections[0].Title != "性格" {
		t.Fatalf("sections = %+v", doc.Sections)
	}
	if !strings.Contains(doc.Sections[0].Body, "烈、傲、孝") {
		t.Errorf("section body missing prose: %q", doc.Sections[0].Body)
	}
}

func TestParseDocument_NoFrontmatter(t *testing.T) {
	src := `## 西天取经

三藏师徒奉旨西行。

## 师徒嫌隙

紧箍咒一念一痛。
`
	doc, err := ParseDocument(src)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if doc.Frontmatter != nil {
		t.Errorf("expected no frontmatter, got %v", doc.Frontmatter)
	}
	if doc.Title != "" {
		t.Errorf("expected no H1, got %q", doc.Title)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(doc.Sections))
	}
	if doc.Sections[0].Title != "西天取经" || doc.Sections[1].Title != "师徒嫌隙" {
		t.Errorf("sections in wrong order: %+v", doc.Sections)
	}
}

func TestParseDocument_MalformedFrontmatter(t *testing.T) {
	src := `---
unterminated: yaml

# Title
`
	if _, err := ParseDocument(src); err == nil {
		t.Fatal("expected error for unterminated frontmatter")
	}
}

func TestParseDocument_InvalidYAML(t *testing.T) {
	src := "---\n: : :\n---\n\n# T\n"
	if _, err := ParseDocument(src); err == nil {
		t.Fatal("expected error for malformed YAML frontmatter")
	}
}

func TestSectionByTitle(t *testing.T) {
	src := "## A\n\nbody-a\n\n## B\n\nbody-b\n"
	doc, _ := ParseDocument(src)
	if sec := doc.SectionByTitle("A"); sec == nil || sec.Body != "body-a" {
		t.Errorf("section A = %+v", sec)
	}
	if sec := doc.SectionByTitle("missing"); sec != nil {
		t.Errorf("expected nil for missing section, got %+v", sec)
	}
}
