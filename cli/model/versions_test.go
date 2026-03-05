package model

import (
	"testing"
)

func TestParseFrontmatterVersionBasic(t *testing.T) {
	content := []byte("---\nversion: '0.5.0'\ntitle: Test\n---\n# Hello")
	got := ParseFrontmatterVersion(content)
	if got != "0.5.0" {
		t.Fatalf("expected 0.5.0, got %q", got)
	}
}

func TestParseFrontmatterVersionDoubleQuotes(t *testing.T) {
	content := []byte("---\ntitle: Foo\nversion: \"1.2.3\"\n---\nbody")
	got := ParseFrontmatterVersion(content)
	if got != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", got)
	}
}

func TestParseFrontmatterVersionUnquoted(t *testing.T) {
	content := []byte("---\nversion: 2.0.0\n---\n")
	got := ParseFrontmatterVersion(content)
	if got != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %q", got)
	}
}

func TestParseFrontmatterVersionMissing(t *testing.T) {
	content := []byte("---\ntitle: No version here\n---\nbody")
	got := ParseFrontmatterVersion(content)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestParseFrontmatterVersionNoFrontmatter(t *testing.T) {
	content := []byte("# Just a markdown file\nNo frontmatter at all.")
	got := ParseFrontmatterVersion(content)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestParseFrontmatterVersionEmptyContent(t *testing.T) {
	got := ParseFrontmatterVersion([]byte{})
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestParseFrontmatterVersionWithWhitespace(t *testing.T) {
	content := []byte("---\nversion:   3.1.4  \n---\n")
	got := ParseFrontmatterVersion(content)
	if got != "3.1.4" {
		t.Fatalf("expected 3.1.4, got %q", got)
	}
}

func TestParseFrontmatterVersionNotAtStart(t *testing.T) {
	// Frontmatter must be at the very start of the file
	content := []byte("\n---\nversion: '1.0.0'\n---\n")
	got := ParseFrontmatterVersion(content)
	if got != "" {
		t.Fatalf("expected empty (not at start), got %q", got)
	}
}

func TestParseFrontmatterVersionIgnoresBodyVersion(t *testing.T) {
	content := []byte("---\ntitle: Test\n---\nversion: 9.9.9\n")
	got := ParseFrontmatterVersion(content)
	if got != "" {
		t.Fatalf("expected empty (version outside frontmatter), got %q", got)
	}
}

// ── FileDisplayName tests ───────────────────────────────────────

func TestFileDisplayNameInstructions(t *testing.T) {
	got := FileDisplayName("instructions/general.instructions.md")
	if got != "general" {
		t.Fatalf("expected 'general', got %q", got)
	}
}

func TestFileDisplayNamePrompt(t *testing.T) {
	got := FileDisplayName("prompts/review.prompt.md")
	if got != "review" {
		t.Fatalf("expected 'review', got %q", got)
	}
}

func TestFileDisplayNameAgent(t *testing.T) {
	got := FileDisplayName("agents/planner.agent.md")
	if got != "planner" {
		t.Fatalf("expected 'planner', got %q", got)
	}
}

func TestFileDisplayNameSkill(t *testing.T) {
	got := FileDisplayName("skills/go-testing/SKILL.md")
	if got != "go-testing" {
		t.Fatalf("expected 'go-testing', got %q", got)
	}
}

func TestFileDisplayNamePlainMd(t *testing.T) {
	got := FileDisplayName("other/README.md")
	if got != "README" {
		t.Fatalf("expected 'README', got %q", got)
	}
}

func TestFileDisplayNameSkillTopLevel(t *testing.T) {
	// SKILL.md at top level (no parent) — should return "SKILL"
	got := FileDisplayName("SKILL.md")
	if got != "SKILL" {
		t.Fatalf("expected 'SKILL', got %q", got)
	}
}
