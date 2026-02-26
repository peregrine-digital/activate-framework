package main

import (
	"os"
	"path/filepath"
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

func TestReadFileVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("---\nversion: '0.3.0'\n---\n# Doc"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFileVersion(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.3.0" {
		t.Fatalf("expected 0.3.0, got %q", got)
	}
}

func TestReadFileVersionMissingFile(t *testing.T) {
	_, err := ReadFileVersion("/nonexistent/file.md")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ── fileDisplayName tests ───────────────────────────────────────

func TestFileDisplayNameInstructions(t *testing.T) {
	got := fileDisplayName("instructions/general.instructions.md")
	if got != "general" {
		t.Fatalf("expected 'general', got %q", got)
	}
}

func TestFileDisplayNamePrompt(t *testing.T) {
	got := fileDisplayName("prompts/review.prompt.md")
	if got != "review" {
		t.Fatalf("expected 'review', got %q", got)
	}
}

func TestFileDisplayNameAgent(t *testing.T) {
	got := fileDisplayName("agents/planner.agent.md")
	if got != "planner" {
		t.Fatalf("expected 'planner', got %q", got)
	}
}

func TestFileDisplayNameSkill(t *testing.T) {
	got := fileDisplayName("skills/go-testing/SKILL.md")
	if got != "go-testing" {
		t.Fatalf("expected 'go-testing', got %q", got)
	}
}

func TestFileDisplayNamePlainMd(t *testing.T) {
	got := fileDisplayName("other/README.md")
	if got != "README" {
		t.Fatalf("expected 'README', got %q", got)
	}
}

func TestFileDisplayNameSkillTopLevel(t *testing.T) {
	// SKILL.md at top level (no parent) — should return "SKILL"
	got := fileDisplayName("SKILL.md")
	if got != "SKILL" {
		t.Fatalf("expected 'SKILL', got %q", got)
	}
}

func TestComputeFileStatusesBasic(t *testing.T) {
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	// Create bundled source file with version
	srcPath := filepath.Join(bundleDir, "instructions", "general.md")
	if err := os.MkdirAll(filepath.Dir(srcPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, []byte("---\nversion: '0.5.0'\n---\n# General"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create installed file with older version
	installedPath := filepath.Join(projectDir, ".github", "instructions", "general.md")
	if err := os.MkdirAll(filepath.Dir(installedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# General"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := Manifest{
		ID:       "test",
		Version:  "0.5.0",
		BasePath: bundleDir,
		Files: []ManifestFile{
			{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
			{Src: "skills/test.md", Dest: "skills/test.md", Tier: "ad-hoc"},
		},
	}

	sidecar := &repoSidecar{
		Files: []string{".github/instructions/general.md"},
	}
	cfg := Config{}

	statuses := ComputeFileStatuses(manifest, sidecar, cfg, projectDir)
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	// First file: installed, update available
	s := statuses[0]
	if !s.Installed {
		t.Fatal("expected installed=true for general.md")
	}
	if s.BundledVersion != "0.5.0" {
		t.Fatalf("expected bundled 0.5.0, got %q", s.BundledVersion)
	}
	if s.InstalledVersion != "0.4.0" {
		t.Fatalf("expected installed 0.4.0, got %q", s.InstalledVersion)
	}
	if !s.UpdateAvailable {
		t.Fatal("expected updateAvailable=true")
	}
	if s.Category != "instructions" {
		t.Fatalf("expected category instructions, got %q", s.Category)
	}
	if s.DisplayName != "general" {
		t.Fatalf("expected displayName 'general', got %q", s.DisplayName)
	}

	// Second file: not installed
	s2 := statuses[1]
	if s2.Installed {
		t.Fatal("expected installed=false for test.md")
	}
	if s2.UpdateAvailable {
		t.Fatal("expected updateAvailable=false for uninstalled file")
	}
}

func TestComputeFileStatusesSkipped(t *testing.T) {
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	srcPath := filepath.Join(bundleDir, "instructions", "sec.md")
	if err := os.MkdirAll(filepath.Dir(srcPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, []byte("---\nversion: '0.5.0'\n---\n# Sec"), 0644); err != nil {
		t.Fatal(err)
	}

	installedPath := filepath.Join(projectDir, ".github", "instructions", "sec.md")
	if err := os.MkdirAll(filepath.Dir(installedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# Sec"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := Manifest{
		ID: "test", Version: "0.5.0", BasePath: bundleDir,
		Files: []ManifestFile{{Src: "instructions/sec.md", Dest: "instructions/sec.md", Tier: "core"}},
	}
	sidecar := &repoSidecar{Files: []string{".github/instructions/sec.md"}}
	cfg := Config{SkippedVersions: map[string]string{"instructions/sec.md": "0.5.0"}}

	statuses := ComputeFileStatuses(manifest, sidecar, cfg, projectDir)
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if !statuses[0].Skipped {
		t.Fatal("expected skipped=true")
	}
	if statuses[0].UpdateAvailable {
		t.Fatal("expected updateAvailable=false when skipped")
	}
}

func TestComputeFileStatusesOverrides(t *testing.T) {
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	manifest := Manifest{
		ID: "test", Version: "1.0.0", BasePath: bundleDir,
		Files: []ManifestFile{
			{Src: "a.md", Dest: "a.md", Tier: "core"},
			{Src: "b.md", Dest: "b.md", Tier: "core"},
		},
	}
	cfg := Config{FileOverrides: map[string]string{
		"a.md": "pinned",
		"b.md": "excluded",
	}}

	statuses := ComputeFileStatuses(manifest, nil, cfg, projectDir)
	if statuses[0].Override != "pinned" {
		t.Fatalf("expected pinned override, got %q", statuses[0].Override)
	}
	if statuses[1].Override != "excluded" {
		t.Fatalf("expected excluded override, got %q", statuses[1].Override)
	}
}

func TestComputeFileStatusesNilSidecar(t *testing.T) {
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	manifest := Manifest{
		ID: "test", Version: "1.0.0", BasePath: bundleDir,
		Files: []ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
	}

	statuses := ComputeFileStatuses(manifest, nil, Config{}, projectDir)
	if len(statuses) != 1 {
		t.Fatalf("expected 1, got %d", len(statuses))
	}
	if statuses[0].Installed {
		t.Fatal("expected not installed with nil sidecar")
	}
}

func TestComputeFileStatusesSameVersion(t *testing.T) {
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	srcPath := filepath.Join(bundleDir, "a.md")
	if err := os.WriteFile(srcPath, []byte("---\nversion: '1.0.0'\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	installedPath := filepath.Join(projectDir, ".github", "a.md")
	if err := os.MkdirAll(filepath.Dir(installedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedPath, []byte("---\nversion: '1.0.0'\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := Manifest{
		ID: "test", Version: "1.0.0", BasePath: bundleDir,
		Files: []ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
	}
	sidecar := &repoSidecar{Files: []string{".github/a.md"}}

	statuses := ComputeFileStatuses(manifest, sidecar, Config{}, projectDir)
	if statuses[0].UpdateAvailable {
		t.Fatal("expected no update when versions match")
	}
}
