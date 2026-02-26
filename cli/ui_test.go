package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// ── Test helpers ────────────────────────────────────────────────

func testManifests() []Manifest {
	return []Manifest{
		{
			ID: "alpha", Name: "Alpha Framework", Version: "1.0.0",
			Description: "First framework",
			Files: []ManifestFile{
				{Src: "instructions/a.md", Dest: "instructions/a.md", Tier: "core", Category: "instructions"},
				{Src: "prompts/b.md", Dest: "prompts/b.md", Tier: "ad-hoc", Category: "prompts"},
			},
			Tiers: []TierDef{
				{ID: "core", Label: "Core"},
				{ID: "ad-hoc", Label: "Standard"},
			},
		},
		{
			ID: "beta", Name: "Beta Framework", Version: "2.0.0",
			Description: "Second framework",
			Files: []ManifestFile{
				{Src: "skills/c.md", Dest: "skills/c.md", Tier: "foundation", Category: "skills"},
			},
			Tiers: []TierDef{
				{ID: "foundation", Label: "Foundation"},
			},
		},
	}
}

func testConfig() Config {
	return Config{Manifest: "alpha", Tier: "core"}
}

// ── resolveTargetPath ───────────────────────────────────────────

func TestResolveTargetPath_Empty(t *testing.T) {
	result := resolveTargetPath("")
	if result == "" {
		t.Fatal("expected non-empty default path")
	}
	if !filepath.IsAbs(result) {
		t.Fatalf("expected absolute path, got %q", result)
	}
}

func TestResolveTargetPath_Absolute(t *testing.T) {
	result := resolveTargetPath("/tmp/test-target")
	if result != "/tmp/test-target" {
		t.Fatalf("expected /tmp/test-target, got %q", result)
	}
}

func TestResolveTargetPath_Tilde(t *testing.T) {
	result := resolveTargetPath("~/projects/test")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "projects/test")
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestResolveTargetPath_Relative(t *testing.T) {
	result := resolveTargetPath("my-project")
	if !filepath.IsAbs(result) {
		t.Fatalf("expected absolute path for relative input, got %q", result)
	}
	if !strings.HasSuffix(result, "my-project") {
		t.Fatalf("expected path ending in my-project, got %q", result)
	}
}

func TestDefaultTargetDir(t *testing.T) {
	d := defaultTargetDir()
	if d == "" {
		t.Fatal("expected non-empty default target dir")
	}
}

// ── formatGroups ────────────────────────────────────────────────

func TestFormatGroups_Empty(t *testing.T) {
	result := formatGroups(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil groups, got %q", result)
	}
}

func TestFormatGroups_Single(t *testing.T) {
	groups := []CategoryGroup{
		{
			Label: "Instructions",
			Files: []ManifestFile{
				{Src: "instructions/test.md", Dest: "instructions/test.md", Tier: "core", Description: "Test file"},
			},
		},
	}
	result := formatGroups(groups)
	if !strings.Contains(result, "Instructions (1)") {
		t.Fatalf("expected 'Instructions (1)', got:\n%s", result)
	}
	if !strings.Contains(result, "test") {
		t.Fatalf("expected file name in output, got:\n%s", result)
	}
	if !strings.Contains(result, "Test file") {
		t.Fatalf("expected description in output, got:\n%s", result)
	}
	if !strings.Contains(result, "tier: core") {
		t.Fatalf("expected tier info in output, got:\n%s", result)
	}
}

func TestFormatGroups_Multiple(t *testing.T) {
	groups := []CategoryGroup{
		{Label: "Instructions", Files: []ManifestFile{{Dest: "instructions/a.md", Tier: "core"}}},
		{Label: "Prompts", Files: []ManifestFile{{Dest: "prompts/b.md", Tier: "ad-hoc"}, {Dest: "prompts/c.md", Tier: "ad-hoc"}}},
	}
	result := formatGroups(groups)
	if !strings.Contains(result, "Instructions (1)") {
		t.Fatal("missing Instructions header")
	}
	if !strings.Contains(result, "Prompts (2)") {
		t.Fatal("missing Prompts header")
	}
}

// ── renderBanner / renderFalconLogo ─────────────────────────────

func TestRenderBanner_NotEmpty(t *testing.T) {
	b := renderBanner()
	if len(b) == 0 {
		t.Fatal("expected non-empty banner")
	}
	// Banner uses block-art wordmark (█ characters) and subtitle
	if !strings.Contains(b, "DIGITAL SERVICES") {
		t.Fatal("expected 'DIGITAL SERVICES' subtitle in banner")
	}
}

func TestRenderFalconLogo_NotEmpty(t *testing.T) {
	logo := renderFalconLogo()
	if len(logo) == 0 {
		t.Fatal("expected non-empty logo")
	}
}

// ── initialModel ────────────────────────────────────────────────

func TestInitialModel_SingleManifest(t *testing.T) {
	manifests := testManifests()[:1]
	cfg := testConfig()
	m := initialModel(manifests, cfg)

	if m.phase != phaseConfigure {
		t.Fatalf("expected phaseConfigure with single manifest, got %d", m.phase)
	}
	if m.manifestID != "alpha" {
		t.Fatalf("expected manifestID=alpha, got %q", m.manifestID)
	}
	if m.chosen.ID != "alpha" {
		t.Fatalf("expected chosen=alpha, got %q", m.chosen.ID)
	}
	if m.form == nil {
		t.Fatal("expected form to be set")
	}
}

func TestInitialModel_MultipleManifests(t *testing.T) {
	manifests := testManifests()
	cfg := testConfig()
	m := initialModel(manifests, cfg)

	if m.phase != phaseManifest {
		t.Fatalf("expected phaseManifest with multiple manifests, got %d", m.phase)
	}
	if m.manifestID != "alpha" {
		t.Fatalf("expected manifestID from config, got %q", m.manifestID)
	}
}

func TestInitialModel_UnknownManifestFallsToFirst(t *testing.T) {
	manifests := testManifests()
	cfg := Config{Manifest: "nonexistent", Tier: "core"}
	m := initialModel(manifests, cfg)

	if m.manifestID != "alpha" {
		t.Fatalf("expected fallback to first manifest, got %q", m.manifestID)
	}
}

// ── model.Update — state machine transitions ────────────────────

func TestModelUpdate_CtrlCQuits(t *testing.T) {
	manifests := testManifests()
	m := initialModel(manifests, testConfig())

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(model)

	if !result.quitting {
		t.Fatal("expected quitting=true after ctrl+c")
	}
	if result.confirm {
		t.Fatal("expected confirm=false after ctrl+c")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestModelUpdate_WindowSizeMsg(t *testing.T) {
	manifests := testManifests()[:1]
	m := initialModel(manifests, testConfig())

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := updated.(model)

	if result.width != 120 || result.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", result.width, result.height)
	}
}

// ── model.View ──────────────────────────────────────────────────

func TestModelView_ManifestPhase(t *testing.T) {
	manifests := testManifests()
	m := initialModel(manifests, testConfig())

	view := m.View()
	if !strings.Contains(view, "Step 1 of 2") {
		t.Fatal("expected 'Step 1 of 2' in manifest phase view")
	}
	if !strings.Contains(view, "navigate") {
		t.Fatal("expected footer hint in view")
	}
}

func TestModelView_ConfigurePhase(t *testing.T) {
	manifests := testManifests()[:1]
	m := initialModel(manifests, testConfig())
	// Single manifest skips to configure
	view := m.View()
	if !strings.Contains(view, "Step 2 of 2") {
		t.Fatal("expected 'Step 2 of 2' in configure phase view")
	}
}

func TestModelView_QuittingReturnsEmpty(t *testing.T) {
	manifests := testManifests()[:1]
	m := initialModel(manifests, testConfig())
	m.quitting = true

	view := m.View()
	if view != "" {
		t.Fatalf("expected empty view when quitting, got %q", view)
	}
}

// ── fullscreenTextModel ─────────────────────────────────────────

func TestFullscreenTextModel_CtrlCQuits(t *testing.T) {
	m := fullscreenTextModel{title: "Test", body: "Hello"}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command on ctrl+c")
	}
}

func TestFullscreenTextModel_EnterQuits(t *testing.T) {
	m := fullscreenTextModel{title: "Test", body: "Hello"}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected quit command on enter")
	}
}

func TestFullscreenTextModel_EscQuits(t *testing.T) {
	m := fullscreenTextModel{title: "Test", body: "Hello"}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected quit command on esc")
	}
}

func TestFullscreenTextModel_QKeyQuits(t *testing.T) {
	m := fullscreenTextModel{title: "Test", body: "Hello"}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command on 'q'")
	}
}

func TestFullscreenTextModel_WindowSize(t *testing.T) {
	m := fullscreenTextModel{title: "Test", body: "Hello"}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	result := updated.(fullscreenTextModel)
	if result.width != 80 || result.height != 24 {
		t.Fatalf("expected 80x24, got %dx%d", result.width, result.height)
	}
}

func TestFullscreenTextModel_View(t *testing.T) {
	m := fullscreenTextModel{
		title:    "Error Report",
		subtitle: "Something failed",
		body:     "Details here",
		width:    80,
		height:   24,
	}
	view := m.View()
	if !strings.Contains(view, "Error Report") {
		t.Fatal("expected title in view")
	}
	if !strings.Contains(view, "Something failed") {
		t.Fatal("expected subtitle in view")
	}
	if !strings.Contains(view, "Details here") {
		t.Fatal("expected body in view")
	}
}

// ── fullscreenFormModel ─────────────────────────────────────────

func TestFullscreenFormModel_CtrlCQuits(t *testing.T) {
	var dummy string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("test").Value(&dummy),
		),
	)
	m := fullscreenFormModel{form: form, title: "Test"}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command on ctrl+c")
	}
}

func TestFullscreenFormModel_WindowSize(t *testing.T) {
	var dummy string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("test").Value(&dummy),
		),
	)
	m := fullscreenFormModel{form: form, title: "Test"}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	result := updated.(fullscreenFormModel)
	if result.width != 100 || result.height != 30 {
		t.Fatalf("expected 100x30, got %dx%d", result.width, result.height)
	}
}

func TestFullscreenFormModel_View(t *testing.T) {
	var dummy string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("test").Value(&dummy),
		),
	)
	m := fullscreenFormModel{form: form, title: "My Title", subtitle: "My Subtitle"}

	view := m.View()
	if !strings.Contains(view, "My Title") {
		t.Fatal("expected title in view")
	}
	if !strings.Contains(view, "My Subtitle") {
		t.Fatal("expected subtitle in view")
	}
}

// ── buildMainMenuForm ───────────────────────────────────────────

func TestBuildMainMenuForm_Fresh(t *testing.T) {
	state := InstallState{}
	var choice string
	form := buildMainMenuForm(state, &choice)

	if form == nil {
		t.Fatal("expected form to be created")
	}
	// Fresh state: no project config, no install marker
	// Should offer: "New setup", "Add managed files", "Show frameworks", "Show current state", "Exit"
}

func TestBuildMainMenuForm_WithProjectConfig(t *testing.T) {
	state := InstallState{HasProjectConfig: true}
	var choice string
	form := buildMainMenuForm(state, &choice)

	if form == nil {
		t.Fatal("expected form to be created")
	}
	// With config: "Install using saved settings", "Change settings and install", etc.
}

func TestBuildMainMenuForm_Installed(t *testing.T) {
	state := InstallState{
		HasProjectConfig:  true,
		HasInstallMarker:  true,
		InstalledManifest: "alpha",
		InstalledVersion:  "1.0.0",
	}
	var choice string
	form := buildMainMenuForm(state, &choice)

	if form == nil {
		t.Fatal("expected form to be created")
	}
	// Installed: has "Reinstall", "Remove", etc.
}

// ── mainMenuModel.stateText ─────────────────────────────────────

func TestStateText_NoConfig(t *testing.T) {
	m := mainMenuModel{state: InstallState{}}
	text := m.stateText()
	if !strings.Contains(text, "no project config detected") {
		t.Fatalf("expected 'no project config', got %q", text)
	}
}

func TestStateText_WithConfig(t *testing.T) {
	m := mainMenuModel{state: InstallState{HasProjectConfig: true}}
	text := m.stateText()
	if !strings.Contains(text, "saved config detected") {
		t.Fatalf("expected 'saved config detected', got %q", text)
	}
}

func TestStateText_Installed(t *testing.T) {
	m := mainMenuModel{state: InstallState{
		HasProjectConfig:  true,
		HasInstallMarker:  true,
		InstalledManifest: "alpha",
		InstalledVersion:  "1.0.0",
	}}
	text := m.stateText()
	if !strings.Contains(text, "installed alpha v1.0.0") {
		t.Fatalf("expected install info, got %q", text)
	}
}

// ── mainMenuModel.stateBody ─────────────────────────────────────

func TestStateBody_ShowsConfig(t *testing.T) {
	m := mainMenuModel{
		projectDir: "/test/project",
		state: InstallState{
			HasGlobalConfig:   true,
			HasProjectConfig:  true,
			HasInstallMarker:  true,
			InstalledManifest: "alpha",
			InstalledVersion:  "1.0.0",
		},
		cfg: Config{Manifest: "alpha", Tier: "standard"},
	}
	body := m.stateBody()
	if !strings.Contains(body, "/test/project") {
		t.Fatal("expected project dir in body")
	}
	if !strings.Contains(body, "manifest: alpha") {
		t.Fatal("expected manifest in body")
	}
	if !strings.Contains(body, "tier: standard") {
		t.Fatal("expected tier in body")
	}
	if !strings.Contains(body, "Install marker: true") {
		t.Fatal("expected install marker in body")
	}
}

// ── mainMenuModel.Update ────────────────────────────────────────

func TestMainMenuModel_CtrlCExits(t *testing.T) {
	state := InstallState{}
	var choice string
	form := buildMainMenuForm(state, &choice)
	m := mainMenuModel{form: form, mode: "menu", choice: ""}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(mainMenuModel)

	if result.action != "exit" {
		t.Fatalf("expected action=exit, got %q", result.action)
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestMainMenuModel_TextModeEscReturnsToMenu(t *testing.T) {
	state := InstallState{}
	var choice string
	form := buildMainMenuForm(state, &choice)
	m := mainMenuModel{
		form:     form,
		mode:     "text",
		state:    state,
		choice:   "list",
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	result := updated.(mainMenuModel)

	if result.mode != "menu" {
		t.Fatalf("expected mode=menu after esc from text, got %q", result.mode)
	}
}

func TestMainMenuModel_TextModeEnterReturnsToMenu(t *testing.T) {
	state := InstallState{}
	var choice string
	form := buildMainMenuForm(state, &choice)
	m := mainMenuModel{
		form:   form,
		mode:   "text",
		state:  state,
		choice: "state",
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(mainMenuModel)

	if result.mode != "menu" {
		t.Fatalf("expected mode=menu after enter from text, got %q", result.mode)
	}
}

func TestMainMenuModel_WindowSize(t *testing.T) {
	state := InstallState{}
	var choice string
	form := buildMainMenuForm(state, &choice)
	m := mainMenuModel{form: form, mode: "menu"}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	result := updated.(mainMenuModel)

	if result.width != 100 || result.height != 50 {
		t.Fatalf("expected 100x50, got %dx%d", result.width, result.height)
	}
}

// ── mainMenuModel.View ──────────────────────────────────────────

func TestMainMenuModel_View_MenuMode(t *testing.T) {
	state := InstallState{HasProjectConfig: true, HasInstallMarker: true, InstalledManifest: "alpha", InstalledVersion: "1.0.0"}
	var choice string
	form := buildMainMenuForm(state, &choice)
	m := mainMenuModel{
		form:       form,
		mode:       "menu",
		state:      state,
		cfg:        testConfig(),
		width:      80,
		height:     24,
	}

	view := m.View()
	if !strings.Contains(view, "DIGITAL SERVICES") {
		t.Fatal("expected 'DIGITAL SERVICES' in menu view")
	}
}

func TestMainMenuModel_View_TextMode(t *testing.T) {
	state := InstallState{}
	var choice string
	form := buildMainMenuForm(state, &choice)
	m := mainMenuModel{
		form:      form,
		mode:      "text",
		textTitle: "Test Title",
		textBody:  "Test Body Content",
		width:     80,
		height:    24,
	}

	view := m.View()
	if !strings.Contains(view, "Test Title") {
		t.Fatal("expected title in text mode view")
	}
	if !strings.Contains(view, "Test Body Content") {
		t.Fatal("expected body in text mode view")
	}
}

// ── RunList ─────────────────────────────────────────────────────

func TestRunList_JSONOverview(t *testing.T) {
	old := activateBaseDir
	activateBaseDir = t.TempDir()
	t.Cleanup(func() { activateBaseDir = old })

	manifests := testManifests()
	cfg := testConfig()
	svc := NewService(t.TempDir(), manifests, cfg, false, "", "")

	// Capture stdout
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := RunList(svc, "", "", "", true)

	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatal(err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\noutput: %s", err, output)
	}
	manifList, ok := result["manifests"].([]interface{})
	if !ok {
		t.Fatal("expected manifests array in JSON")
	}
	if len(manifList) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifList))
	}
}

func TestRunList_JSONDetail(t *testing.T) {
	old := activateBaseDir
	activateBaseDir = t.TempDir()
	t.Cleanup(func() { activateBaseDir = old })

	manifests := testManifests()
	cfg := testConfig()
	svc := NewService(t.TempDir(), manifests, cfg, false, "", "")

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := RunList(svc, "alpha", "core", "", true)

	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatal(err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput: %s", err, output)
	}
	if result["manifest"] != "alpha" {
		t.Fatalf("expected manifest=alpha, got %v", result["manifest"])
	}
}

func TestRunList_HumanOverview(t *testing.T) {
	old := activateBaseDir
	activateBaseDir = t.TempDir()
	t.Cleanup(func() { activateBaseDir = old })

	manifests := testManifests()
	cfg := testConfig()
	svc := NewService(t.TempDir(), manifests, cfg, false, "", "")

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := RunList(svc, "", "", "", false)

	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatal(err)
	}

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "Alpha Framework") {
		t.Fatalf("expected 'Alpha Framework' in human output, got:\n%s", output)
	}
	if !strings.Contains(output, "Beta Framework") {
		t.Fatalf("expected 'Beta Framework' in human output, got:\n%s", output)
	}
}

func TestRunList_UnknownManifest(t *testing.T) {
	old := activateBaseDir
	activateBaseDir = t.TempDir()
	t.Cleanup(func() { activateBaseDir = old })

	manifests := testManifests()
	cfg := testConfig()
	svc := NewService(t.TempDir(), manifests, cfg, false, "", "")

	err := RunList(svc, "nonexistent", "", "", true)
	if err == nil {
		t.Fatal("expected error for unknown manifest")
	}
}

// ── buildConfigureForm ──────────────────────────────────────────

func TestBuildConfigureForm(t *testing.T) {
	manifests := testManifests()[:1]
	cfg := testConfig()
	m := initialModel(manifests, cfg)

	form := m.buildConfigureForm()
	if form == nil {
		t.Fatal("expected configure form")
	}
}

// ── buildManifestForm ───────────────────────────────────────────

func TestBuildManifestForm(t *testing.T) {
	manifests := testManifests()
	cfg := testConfig()
	m := initialModel(manifests, cfg)

	form := m.buildManifestForm()
	if form == nil {
		t.Fatal("expected manifest form")
	}
}

// ── End-to-end install wizard model transitions ─────────────────

func TestInstallerWizard_SingleManifestSkipsPhase(t *testing.T) {
	manifests := testManifests()[:1]
	m := initialModel(manifests, testConfig())

	// Single manifest → should start in configure phase
	if m.phase != phaseConfigure {
		t.Fatalf("expected phaseConfigure, got %d", m.phase)
	}
	if m.chosen.ID != "alpha" {
		t.Fatalf("expected chosen=alpha, got %q", m.chosen.ID)
	}
}

func TestInstallerWizard_MultipleManifestsStartsAtSelection(t *testing.T) {
	manifests := testManifests()
	m := initialModel(manifests, testConfig())

	if m.phase != phaseManifest {
		t.Fatalf("expected phaseManifest, got %d", m.phase)
	}
}

// ── Edge cases ──────────────────────────────────────────────────

func TestFullscreenTextModel_Init(t *testing.T) {
	m := fullscreenTextModel{}
	cmd := m.Init()
	if cmd != nil {
		t.Fatal("expected nil init cmd for text model")
	}
}

func TestModelView_VerticalCentering(t *testing.T) {
	manifests := testManifests()[:1]
	m := initialModel(manifests, testConfig())
	m.height = 100

	view := m.View()
	// Should have padding at top
	if !strings.HasPrefix(view, "\n") {
		t.Fatal("expected vertical padding for tall terminal")
	}
}

func TestFullscreenTextModel_VerticalCentering(t *testing.T) {
	m := fullscreenTextModel{
		title:  "Test",
		body:   "Short body",
		width:  80,
		height: 100,
	}
	view := m.View()
	if !strings.HasPrefix(view, "\n") {
		t.Fatal("expected vertical padding for tall terminal")
	}
}

func TestMainMenuModel_View_ExitAction(t *testing.T) {
	state := InstallState{}
	var choice string
	form := buildMainMenuForm(state, &choice)
	m := mainMenuModel{
		form:   form,
		mode:   "menu",
		state:  state,
		action: "exit",
	}
	// Should still render something (not crash)
	view := m.View()
	if len(view) == 0 {
		t.Fatal("expected non-empty view even with exit action pending")
	}
}

// ── RunInteractiveInstall setup validation ──────────────────────

func TestRunInteractiveInstall_TargetPath(t *testing.T) {
	// Verify resolveTargetPath works for various inputs used by installer
	cases := []struct {
		input    string
		checkAbs bool
	}{
		{"", true},
		{"/absolute/path", true},
		{"~/relative", true},
		{"just-a-name", true},
	}
	for _, c := range cases {
		result := resolveTargetPath(c.input)
		if c.checkAbs && !filepath.IsAbs(result) {
			t.Errorf("resolveTargetPath(%q) = %q, want absolute", c.input, result)
		}
	}
}
