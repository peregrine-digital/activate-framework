package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

// ── resolvePresetFileEntry tests ────────────────────────────────

func TestResolvePresetFileEntry_RootRelativeDir(t *testing.T) {
	f := resolvePresetFileEntry("adhoc", "/skills/ci-debugger")
	if f.Src != "skills/ci-debugger" {
		t.Fatalf("Src = %q, want %q", f.Src, "skills/ci-debugger")
	}
	if f.Dest != "skills/ci-debugger" {
		t.Fatalf("Dest = %q, want %q", f.Dest, "skills/ci-debugger")
	}
	if !f.IsDir {
		t.Fatal("expected IsDir=true for directory entry without extension")
	}
}

func TestResolvePresetFileEntry_RootRelativeFile(t *testing.T) {
	f := resolvePresetFileEntry("adhoc", "/mcp-servers/github.json")
	if f.Src != "mcp-servers/github.json" {
		t.Fatalf("Src = %q, want %q", f.Src, "mcp-servers/github.json")
	}
	if f.IsDir {
		t.Fatal("expected IsDir=false for file entry with extension")
	}
}

func TestResolvePresetFileEntry_LocalDir(t *testing.T) {
	f := resolvePresetFileEntry("activate", "skills/instruction-authoring")
	if f.Src != "plugins/activate/skills/instruction-authoring" {
		t.Fatalf("Src = %q, want %q", f.Src, "plugins/activate/skills/instruction-authoring")
	}
	if f.Dest != "skills/instruction-authoring" {
		t.Fatalf("Dest = %q, want %q", f.Dest, "skills/instruction-authoring")
	}
	if !f.IsDir {
		t.Fatal("expected IsDir=true for local directory entry")
	}
}

func TestResolvePresetFileEntry_LocalFile(t *testing.T) {
	f := resolvePresetFileEntry("adhoc", "instructions/security.instructions.md")
	if f.Src != "plugins/adhoc/instructions/security.instructions.md" {
		t.Fatalf("Src = %q, want %q", f.Src, "plugins/adhoc/instructions/security.instructions.md")
	}
	if f.IsDir {
		t.Fatal("expected IsDir=false for file entry")
	}
}

func TestResolvePresetFileEntry_CrossPluginDir(t *testing.T) {
	f := resolvePresetFileEntry("myplug", "@otherplugin/skills/foo")
	if f.Src != "plugins/otherplugin/skills/foo" {
		t.Fatalf("Src = %q, want %q", f.Src, "plugins/otherplugin/skills/foo")
	}
	if f.Dest != "skills/foo" {
		t.Fatalf("Dest = %q, want %q", f.Dest, "skills/foo")
	}
	if !f.IsDir {
		t.Fatal("expected IsDir=true for cross-plugin directory entry")
	}
}

func TestResolvePresetFileEntry_CrossPluginFile(t *testing.T) {
	f := resolvePresetFileEntry("myplug", "@otherplugin/agents/test.agent.md")
	if f.IsDir {
		t.Fatal("expected IsDir=false for cross-plugin file entry")
	}
}

// ── expandPresetDirEntries tests ────────────────────────────────

func TestExpandPresetDirEntries_ExpandsDirectory(t *testing.T) {
	// Simulate a GitHub Contents API that returns files for a skill directory.
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries := []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}{
			{Name: "SKILL.md", Type: "file"},
			{Name: "helper.md", Type: "file"},
		}
		json.NewEncoder(w).Encode(entries)
	}))
	defer api.Close()

	raw := httptest.NewServer(http.NotFoundHandler())
	defer raw.Close()
	withTestServers(t, raw, api)

	files := []model.PresetFile{
		{Src: "skills/ci-debugger", Dest: "skills/ci-debugger", IsDir: true},
		{Src: "plugins/adhoc/AGENTS.md", Dest: "AGENTS.md"},
	}
	result, err := expandPresetDirEntries(files, "owner/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect: dir marker + 2 expanded files + 1 regular file = 4
	if len(result) != 4 {
		t.Fatalf("expected 4 entries, got %d: %+v", len(result), result)
	}
	if !result[0].IsDir {
		t.Fatal("first entry should be the IsDir marker")
	}
	if result[1].Src != "skills/ci-debugger/SKILL.md" {
		t.Fatalf("expected expanded Src, got %q", result[1].Src)
	}
	if result[1].Dest != "skills/ci-debugger/SKILL.md" {
		t.Fatalf("expected expanded Dest, got %q", result[1].Dest)
	}
	if result[2].Src != "skills/ci-debugger/helper.md" {
		t.Fatalf("expected expanded Src, got %q", result[2].Src)
	}
	if result[3].Src != "plugins/adhoc/AGENTS.md" {
		t.Fatal("regular file should be preserved")
	}
}

func TestExpandPresetDirEntries_RecursesSubdirectories(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/repos/owner/repo/contents/skills/writing-skills":
			entries := []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			}{
				{Name: "SKILL.md", Type: "file"},
				{Name: "examples", Type: "dir"},
			}
			json.NewEncoder(w).Encode(entries)
		case path == "/repos/owner/repo/contents/skills/writing-skills/examples":
			entries := []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			}{
				{Name: "SAMPLE.md", Type: "file"},
			}
			json.NewEncoder(w).Encode(entries)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer api.Close()

	raw := httptest.NewServer(http.NotFoundHandler())
	defer raw.Close()
	withTestServers(t, raw, api)

	files := []model.PresetFile{
		{Src: "skills/writing-skills", Dest: "skills/writing-skills", IsDir: true},
	}
	result, err := expandPresetDirEntries(files, "owner/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect: dir marker + SKILL.md + examples/SAMPLE.md = 3
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(result), result)
	}
	if result[1].Dest != "skills/writing-skills/SKILL.md" {
		t.Fatalf("expected SKILL.md, got %q", result[1].Dest)
	}
	if result[2].Dest != "skills/writing-skills/examples/SAMPLE.md" {
		t.Fatalf("expected examples/SAMPLE.md, got %q", result[2].Dest)
	}
}

func TestExpandPresetDirEntries_CrossPluginDirPaths(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries := []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}{
			{Name: "SKILL.md", Type: "file"},
		}
		json.NewEncoder(w).Encode(entries)
	}))
	defer api.Close()

	raw := httptest.NewServer(http.NotFoundHandler())
	defer raw.Close()
	withTestServers(t, raw, api)

	// Cross-plugin: Src and Dest differ in prefix
	files := []model.PresetFile{
		{Src: "plugins/other/skills/foo", Dest: "skills/foo", IsDir: true},
	}
	result, err := expandPresetDirEntries(files, "owner/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	// Expanded file: Src = plugins/other/skills/foo/SKILL.md, Dest = skills/foo/SKILL.md
	if result[1].Src != "plugins/other/skills/foo/SKILL.md" {
		t.Fatalf("expected expanded Src, got %q", result[1].Src)
	}
	if result[1].Dest != "skills/foo/SKILL.md" {
		t.Fatalf("expected expanded Dest, got %q", result[1].Dest)
	}
}

func TestExpandPresetDirEntries_NoOpForFiles(t *testing.T) {
	files := []model.PresetFile{
		{Src: "plugins/adhoc/AGENTS.md", Dest: "AGENTS.md"},
		{Src: "mcp-servers/github.json", Dest: "mcp-servers/github.json"},
	}
	result, err := expandPresetDirEntries(files, "owner/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries unchanged, got %d", len(result))
	}
}
