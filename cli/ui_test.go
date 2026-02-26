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

// sendToForm sends a tea.Msg to a huh.Form and drains resulting commands
// so that state transitions (e.g. StateCompleted) propagate properly.
func sendToForm(form *huh.Form, msg tea.Msg) *huh.Form {
	updated, cmd := form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		form = f
	}
	for i := 0; i < 5 && cmd != nil; i++ {
		m := cmd()
		if m == nil {
			break
		}
		if _, ok := m.(tea.QuitMsg); ok {
			break
		}
		updated, cmd = form.Update(m)
		if f, ok := updated.(*huh.Form); ok {
			form = f
		}
	}
	return form
}

// sendKey is a convenience wrapper for sending a single key to a form.
func sendKey(form *huh.Form, key tea.KeyType) *huh.Form {
	return sendToForm(form, tea.KeyMsg{Type: key})
}

// updateModel sends a message through a Bubble Tea model, then repeatedly
// drains returned commands (up to 10 levels) so form state transitions
// (e.g. huh.StateCompleted) fully propagate through the model.
func updateModel(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	for i := 0; i < 10 && cmd != nil; i++ {
		innerMsg := cmd()
		if innerMsg == nil {
			break
		}
		if _, ok := innerMsg.(tea.QuitMsg); ok {
			// Return a synthetic quit command so callers can detect it
			return updated, tea.Quit
		}
		updated, cmd = updated.Update(innerMsg)
	}
	return updated, cmd
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
	if m.vals.manifestID != "alpha" {
		t.Fatalf("expected manifestID=alpha, got %q", m.vals.manifestID)
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
	if m.vals.manifestID != "alpha" {
		t.Fatalf("expected manifestID from config, got %q", m.vals.manifestID)
	}
}

func TestInitialModel_UnknownManifestFallsToFirst(t *testing.T) {
	manifests := testManifests()
	cfg := Config{Manifest: "nonexistent", Tier: "core"}
	m := initialModel(manifests, cfg)

	if m.vals.manifestID != "alpha" {
		t.Fatalf("expected fallback to first manifest, got %q", m.vals.manifestID)
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
	if result.vals.confirm {
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
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)

	if form == nil {
		t.Fatal("expected form to be created")
	}
	// Fresh state: no project config, no install marker
	// Should offer: "New setup", "Add managed files", "Show frameworks", "Show current state", "Exit"
}

func TestBuildMainMenuForm_WithProjectConfig(t *testing.T) {
	state := InstallState{HasProjectConfig: true}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)

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
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)

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
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	m := mainMenuModel{form: form, mode: "menu", vals: vals}

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
	vals := &menuValues{choice: "list"}
	form := buildMainMenuForm(state, &vals.choice)
	m := mainMenuModel{
		form:     form,
		mode:     "text",
		state:    state,
		vals:     vals,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	result := updated.(mainMenuModel)

	if result.mode != "menu" {
		t.Fatalf("expected mode=menu after esc from text, got %q", result.mode)
	}
}

func TestMainMenuModel_TextModeEnterReturnsToMenu(t *testing.T) {
	state := InstallState{}
	vals := &menuValues{choice: "state"}
	form := buildMainMenuForm(state, &vals.choice)
	m := mainMenuModel{
		form:   form,
		mode:   "text",
		state:  state,
		vals:   vals,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(mainMenuModel)

	if result.mode != "menu" {
		t.Fatalf("expected mode=menu after enter from text, got %q", result.mode)
	}
}

func TestMainMenuModel_WindowSize(t *testing.T) {
	state := InstallState{}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
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
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
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
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
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
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
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

// ════════════════════════════════════════════════════════════════
// Keyboard Navigation Tests — real key sequences through forms
// ════════════════════════════════════════════════════════════════

// ── Manifest select form: arrow key navigation ──────────────────

func TestManifestForm_ArrowNavSelectsManifest(t *testing.T) {
	manifests := testManifests()
	m := initialModel(manifests, testConfig())

	// Should start at phaseManifest with default "alpha"
	if m.vals.manifestID != "alpha" {
		t.Fatalf("expected default alpha, got %q", m.vals.manifestID)
	}

	// Init the form so it can accept key events
	m.form.Init()

	// Press down to move to "beta"
	m.form = sendKey(m.form, tea.KeyDown)
	if m.vals.manifestID != "beta" {
		t.Fatalf("after ↓: expected beta, got %q", m.vals.manifestID)
	}

	// Press up to go back to "alpha"
	m.form = sendKey(m.form, tea.KeyUp)
	if m.vals.manifestID != "alpha" {
		t.Fatalf("after ↑: expected alpha, got %q", m.vals.manifestID)
	}
}

func TestManifestForm_EnterCompletesSelection(t *testing.T) {
	manifests := testManifests()
	m := initialModel(manifests, testConfig())
	m.form.Init()

	// Navigate to beta
	m.form = sendKey(m.form, tea.KeyDown)
	if m.vals.manifestID != "beta" {
		t.Fatalf("expected beta after ↓, got %q", m.vals.manifestID)
	}

	// Press enter to confirm selection
	m.form = sendKey(m.form, tea.KeyEnter)
	if m.form.State != huh.StateCompleted {
		t.Fatalf("expected form completed after enter, got state %d", m.form.State)
	}
}

// ── Manifest form: completing triggers phase transition ─────────

func TestManifestForm_CompletionAdvancesToConfigure(t *testing.T) {
	manifests := testManifests()
	m := initialModel(manifests, testConfig())
	m.form.Init()

	// Navigate to beta and confirm
	m.form = sendKey(m.form, tea.KeyDown)

	// Send enter through the model (not just the form) so phase transition fires
	updated, _ := updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(model)

	if result.phase != phaseConfigure {
		t.Fatalf("expected phaseConfigure after manifest select, got %d", result.phase)
	}
	if result.vals.manifestID != "beta" {
		t.Fatalf("expected manifestID=beta, got %q", result.vals.manifestID)
	}
	if result.chosen.ID != "beta" {
		t.Fatalf("expected chosen.ID=beta, got %q", result.chosen.ID)
	}
}

// ── Configure form: tier selection ──────────────────────────────

func TestConfigureForm_TierNavigation(t *testing.T) {
	manifests := testManifests()[:1] // alpha has 2 tiers: core, ad-hoc
	cfg := Config{Manifest: "alpha", Tier: "core"}
	m := initialModel(manifests, cfg)

	// Should start in configure phase (single manifest)
	if m.phase != phaseConfigure {
		t.Fatalf("expected phaseConfigure, got %d", m.phase)
	}
	if m.vals.tierID != "core" {
		t.Fatalf("expected default tier=core, got %q", m.vals.tierID)
	}

	m.form.Init()

	// Down arrow selects the second tier
	m.form = sendKey(m.form, tea.KeyDown)
	if m.vals.tierID != "ad-hoc" {
		t.Fatalf("expected ad-hoc after ↓, got %q", m.vals.tierID)
	}

	// Up arrow goes back
	m.form = sendKey(m.form, tea.KeyUp)
	if m.vals.tierID != "core" {
		t.Fatalf("expected core after ↑, got %q", m.vals.tierID)
	}
}

func TestConfigureForm_TierSelectAndAdvance(t *testing.T) {
	manifests := testManifests()[:1]
	cfg := Config{Manifest: "alpha", Tier: "core"}
	m := initialModel(manifests, cfg)
	m.form.Init()

	// Select second tier and press enter to advance to target dir input
	m.form = sendKey(m.form, tea.KeyDown)
	if m.vals.tierID != "ad-hoc" {
		t.Fatalf("expected ad-hoc, got %q", m.vals.tierID)
	}

	// Enter advances from tier select to target dir input field
	m.form = sendKey(m.form, tea.KeyEnter)
	// Form should NOT be completed yet (still has input + confirm fields)
	if m.form.State == huh.StateCompleted {
		t.Fatal("form should not complete after first field enter")
	}
}

// ── Configure form: complete full wizard flow ───────────────────

func TestConfigureForm_FullFlowWithConfirm(t *testing.T) {
	manifests := testManifests()[:1]
	cfg := Config{Manifest: "alpha", Tier: "core"}
	m := initialModel(manifests, cfg)
	m.form.Init()

	// 1) Select tier (accept default core, press enter)
	m.form = sendKey(m.form, tea.KeyEnter)

	// 2) Target dir input (accept default, press enter)
	m.form = sendKey(m.form, tea.KeyEnter)

	// 3) Confirm: default is Cancel (false), press left to select Install
	m.form = sendToForm(m.form, tea.KeyMsg{Type: tea.KeyLeft})
	m.form = sendKey(m.form, tea.KeyEnter)

	if m.form.State != huh.StateCompleted {
		t.Fatalf("expected form completed after full flow, got state %d", m.form.State)
	}
	if m.vals.tierID != "core" {
		t.Fatalf("expected tier=core, got %q", m.vals.tierID)
	}
	if !m.vals.confirm {
		t.Fatal("expected confirm=true")
	}
}

func TestConfigureForm_CancelFlow(t *testing.T) {
	manifests := testManifests()[:1]
	cfg := Config{Manifest: "alpha", Tier: "core"}
	m := initialModel(manifests, cfg)
	m.form.Init()

	// 1) Accept tier
	m.form = sendKey(m.form, tea.KeyEnter)
	// 2) Accept target dir
	m.form = sendKey(m.form, tea.KeyEnter)
	// 3) On confirm field, default is Cancel (false) — just press enter
	m.form = sendKey(m.form, tea.KeyEnter)

	if m.form.State != huh.StateCompleted {
		t.Fatalf("expected form completed, got state %d", m.form.State)
	}
	if m.vals.confirm {
		t.Fatal("expected confirm=false after selecting Cancel")
	}
}

// ── Full installer wizard: manifest → configure phase transition ──

func TestInstallerWizard_FullKeyboardFlow(t *testing.T) {
	manifests := testManifests()
	cfg := testConfig()
	m := initialModel(manifests, cfg)
	m.form.Init()

	// Phase 1: manifest selection — select "beta" (↓ then enter)
	m.form = sendKey(m.form, tea.KeyDown)
	if m.vals.manifestID != "beta" {
		t.Fatalf("expected beta after ↓, got %q", m.vals.manifestID)
	}

	// Press enter through the full model to trigger phase transition
	updated, _ := updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if m.phase != phaseConfigure {
		t.Fatalf("expected phaseConfigure, got %d", m.phase)
	}
	if m.chosen.ID != "beta" {
		t.Fatalf("expected chosen=beta, got %q", m.chosen.ID)
	}

	// Phase 2: configure — beta has only 1 tier, so no tier select
	// Form has: target dir input + confirm
	m.form.Init()

	// Accept default target dir
	m.form = sendKey(m.form, tea.KeyEnter)
	// Confirm install
	m.form = sendKey(m.form, tea.KeyEnter)

	if m.form.State != huh.StateCompleted {
		t.Fatalf("expected completed, got state %d", m.form.State)
	}
}

// ── Main menu: keyboard navigation through menu items ───────────

func TestMainMenu_NavigateToSpecificOption(t *testing.T) {
	state := InstallState{}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	form.Init()

	// Fresh state menu: [guided-install, repo-add, list, state, exit]
	if vals.choice != "guided-install" {
		t.Fatalf("expected default=guided-install, got %q", vals.choice)
	}

	// Navigate down to "list" (3rd item: guided-install → repo-add → list)
	form = sendKey(form, tea.KeyDown)
	form = sendKey(form, tea.KeyDown)
	if vals.choice != "list" {
		t.Fatalf("expected list after 2 ↓, got %q", vals.choice)
	}

	// Navigate down once more to "state"
	form = sendKey(form, tea.KeyDown)
	if vals.choice != "state" {
		t.Fatalf("expected state after 3 ↓, got %q", vals.choice)
	}

	// Press enter to select "state"
	form = sendKey(form, tea.KeyEnter)
	if form.State != huh.StateCompleted {
		t.Fatalf("expected completed, got state %d", form.State)
	}
	if vals.choice != "state" {
		t.Fatalf("expected choice=state after enter, got %q", vals.choice)
	}
}

func TestMainMenu_NavigateToExit(t *testing.T) {
	state := InstallState{}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	form.Init()

	// Fresh state: [guided-install, repo-add, list, state, exit] = 5 items
	for i := 0; i < 4; i++ {
		form = sendKey(form, tea.KeyDown)
	}
	if vals.choice != "exit" {
		t.Fatalf("expected exit after 4 ↓, got %q", vals.choice)
	}

	form = sendKey(form, tea.KeyEnter)
	if vals.choice != "exit" {
		t.Fatalf("expected choice=exit, got %q", vals.choice)
	}
}

func TestMainMenu_InstalledStateHasRemoveOption(t *testing.T) {
	state := InstallState{
		HasProjectConfig:  true,
		HasInstallMarker:  true,
		InstalledManifest: "alpha",
		InstalledVersion:  "1.0.0",
	}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	form.Init()

	// Installed state: [quick-install, guided-install, repo-add, repo-remove, list, state, exit]
	var items []string
	items = append(items, vals.choice)
	for i := 0; i < 6; i++ {
		form = sendKey(form, tea.KeyDown)
		items = append(items, vals.choice)
	}

	expected := []string{"quick-install", "guided-install", "repo-add", "repo-remove", "list", "state", "exit"}
	if len(items) != len(expected) {
		t.Fatalf("expected %d items, got %d: %v", len(expected), len(items), items)
	}
	for i, exp := range expected {
		if items[i] != exp {
			t.Fatalf("item %d: expected %q, got %q\nall: %v", i, exp, items[i], items)
		}
	}
}

// ── Main menu model: selection triggers action ──────────────────

func TestMainMenuModel_SelectListShowsFrameworks(t *testing.T) {
	manifests := testManifests()
	state := InstallState{}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	form.Init()

	m := mainMenuModel{
		form:      form,
		mode:      "menu",
		manifests: manifests,
		state:     state,
		vals: vals,
	}

	// Navigate to "list" (2 downs from guided-install)
	m.form = sendKey(m.form, tea.KeyDown)
	m.form = sendKey(m.form, tea.KeyDown)

	// Press enter through the model to trigger action handling
	updated, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(mainMenuModel)

	if result.mode != "text" {
		t.Fatalf("expected mode=text after selecting list, got %q", result.mode)
	}
	if result.textTitle != "Frameworks" {
		t.Fatalf("expected title=Frameworks, got %q", result.textTitle)
	}
	if !strings.Contains(result.textBody, "Alpha Framework") {
		t.Fatal("expected framework names in text body")
	}
	if cmd != nil {
		t.Fatal("selecting list should not quit — it shows text inline")
	}
}

func TestMainMenuModel_SelectStateShowsBody(t *testing.T) {
	state := InstallState{HasProjectConfig: true, HasInstallMarker: true, InstalledManifest: "alpha", InstalledVersion: "1.0.0"}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	form.Init()

	m := mainMenuModel{
		form:       form,
		mode:       "menu",
		state:      state,
		cfg:        testConfig(),
		projectDir: "/tmp/test",
		vals: vals,
	}

	// Installed: [quick-install, guided-install, repo-add, repo-remove, list, state, exit]
	// Navigate to "state" (5 downs)
	for i := 0; i < 5; i++ {
		m.form = sendKey(m.form, tea.KeyDown)
	}

	updated, _ := updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(mainMenuModel)

	if result.mode != "text" {
		t.Fatalf("expected text mode, got %q", result.mode)
	}
	if result.textTitle != "Current State" {
		t.Fatalf("expected title=Current State, got %q", result.textTitle)
	}
	if !strings.Contains(result.textBody, "/tmp/test") {
		t.Fatal("expected project dir in state body")
	}
}

func TestMainMenuModel_SelectGuidedInstallQuits(t *testing.T) {
	state := InstallState{}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	form.Init()

	m := mainMenuModel{
		form:   form,
		mode:   "menu",
		state:  state,
		vals: vals,
	}

	// First item is already "guided-install" — just enter
	updated, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(mainMenuModel)

	if result.action != "guided-install" {
		t.Fatalf("expected action=guided-install, got %q", result.action)
	}
	if cmd == nil {
		t.Fatal("expected quit command for action that exits menu loop")
	}
}

func TestMainMenuModel_SelectExitQuits(t *testing.T) {
	state := InstallState{}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	form.Init()

	m := mainMenuModel{
		form:   form,
		mode:   "menu",
		state:  state,
		vals: vals,
	}

	// Navigate to exit (4 downs)
	for i := 0; i < 4; i++ {
		m.form = sendKey(m.form, tea.KeyDown)
	}

	updated, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(mainMenuModel)

	if result.action != "exit" {
		t.Fatalf("expected action=exit, got %q", result.action)
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// ── Main menu model: text mode keyboard navigation ──────────────

func TestMainMenuModel_TextModeFullCycle(t *testing.T) {
	manifests := testManifests()
	state := InstallState{}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	form.Init()

	m := mainMenuModel{
		form:      form,
		mode:      "menu",
		manifests: manifests,
		state:     state,
		vals: vals,
	}

	// Navigate to "list" and select it
	m.form = sendKey(m.form, tea.KeyDown)
	m.form = sendKey(m.form, tea.KeyDown)
	updated, _ := updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(mainMenuModel)

	if m.mode != "text" {
		t.Fatalf("expected text mode, got %q", m.mode)
	}

	// View should show framework list
	view := m.View()
	if !strings.Contains(view, "Frameworks") {
		t.Fatal("expected Frameworks in text mode view")
	}

	// Press escape to return to menu
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(mainMenuModel)

	if m.mode != "menu" {
		t.Fatalf("expected menu mode after esc, got %q", m.mode)
	}

	// View should show menu form again
	view = m.View()
	if !strings.Contains(view, "navigate") {
		t.Fatal("expected menu footer hint after returning from text mode")
	}
}

// ── Installer wizard via model.Update: full e2e ─────────────────

func TestInstallerWizard_ModelUpdatePhaseTransitions(t *testing.T) {
	manifests := testManifests()
	m := initialModel(manifests, testConfig())
	m.form.Init()

	// Start in manifest phase
	if m.phase != phaseManifest {
		t.Fatalf("expected phaseManifest, got %d", m.phase)
	}

	// Select beta (↓) and confirm (enter) — this should trigger phase transition
	m.form = sendKey(m.form, tea.KeyDown)
	if m.vals.manifestID != "beta" {
		t.Fatalf("expected beta, got %q", m.vals.manifestID)
	}

	// Send enter through model.Update to trigger state machine
	var updated tea.Model
	updated, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	// Should now be in configure phase with beta selected
	if m.phase != phaseConfigure {
		t.Fatalf("expected phaseConfigure, got %d", m.phase)
	}
	if m.chosen.ID != "beta" {
		t.Fatalf("expected chosen=beta, got %q", m.chosen.ID)
	}

	// Beta has 1 tier, so form should have just target dir + confirm
	// Init the new form
	m.form.Init()

	// Accept target dir
	m.form = sendKey(m.form, tea.KeyEnter)
	// On confirm: default is Cancel (false), press left for Install
	m.form = sendToForm(m.form, tea.KeyMsg{Type: tea.KeyLeft})
	m.form = sendKey(m.form, tea.KeyEnter)

	// Send final state through model
	updated, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if !m.quitting {
		t.Fatal("expected quitting=true after full wizard completion")
	}
	if m.vals.confirm != true {
		t.Fatal("expected confirm=true")
	}
	if m.chosen.ID != "beta" {
		t.Fatal("expected chosen manifest to stay as beta")
	}
}

// ── Wrap-around / boundary: navigate past end ───────────────────

func TestManifestForm_WraparoundBehavior(t *testing.T) {
	manifests := testManifests()
	m := initialModel(manifests, testConfig())
	m.form.Init()

	// With 2 manifests, pressing down twice should wrap or stop
	m.form = sendKey(m.form, tea.KeyDown)
	first := m.vals.manifestID
	m.form = sendKey(m.form, tea.KeyDown)
	second := m.vals.manifestID

	// Either it wraps (second == alpha) or stays at end (second == beta)
	// Just verify it doesn't crash and produces a valid value
	if second != "alpha" && second != "beta" {
		t.Fatalf("unexpected manifestID after 2 downs: %q", second)
	}
	_ = first
}

func TestConfigureForm_SingleTierSkipsTierSelect(t *testing.T) {
	// Beta has only 1 tier — form should skip tier selection
	betaOnly := []Manifest{testManifests()[1]} // beta
	cfg := Config{Manifest: "beta", Tier: "foundation"}
	m := initialModel(betaOnly, cfg)
	m.form.Init()

	if m.vals.tierID != "foundation" {
		t.Fatalf("expected auto-set tier=foundation, got %q", m.vals.tierID)
	}

	// First field should be target dir (not tier select), then confirm
	// Enter twice should complete (target + confirm)
	m.form = sendKey(m.form, tea.KeyEnter)
	m.form = sendKey(m.form, tea.KeyEnter)

	if m.form.State != huh.StateCompleted {
		t.Fatalf("expected completed for single-tier form, got state %d", m.form.State)
	}
}

// ════════════════════════════════════════════════════════════════
// Full runtime simulation — ALL keys routed through model.Update
// ════════════════════════════════════════════════════════════════

// simulateRuntime mirrors Bubble Tea's event loop: Init → drain, then
// every key goes through model.Update with full command draining.
func simulateRuntime(m tea.Model, keys []tea.Msg) tea.Model {
	// Process Init
	cmd := m.Init()
	for i := 0; i < 20 && cmd != nil; i++ {
		msg := cmd()
		if msg == nil {
			break
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			return m
		}
		m, cmd = m.Update(msg)
	}

	// Process each key event
	for _, key := range keys {
		m, cmd = m.Update(key)
		for i := 0; i < 20 && cmd != nil; i++ {
			msg := cmd()
			if msg == nil {
				break
			}
			if _, ok := msg.(tea.QuitMsg); ok {
				return m
			}
			m, cmd = m.Update(msg)
		}
	}
	return m
}

func TestMainMenuModel_ExitViaModelUpdate(t *testing.T) {
	// Installed state: 7 menu items
	// [quick-install, guided-install, repo-add, repo-remove, list, state, exit]
	state := InstallState{
		HasProjectConfig:  true,
		HasInstallMarker:  true,
		InstalledManifest: "alpha",
		InstalledVersion:  "1.0.0",
	}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	m := mainMenuModel{
		form:      form,
		mode:      "menu",
		state:     state,
		vals:      vals,
		manifests: testManifests(),
	}

	// Navigate to Exit: 6 downs + enter, ALL through model.Update
	keys := make([]tea.Msg, 0, 7)
	for i := 0; i < 6; i++ {
		keys = append(keys, tea.KeyMsg{Type: tea.KeyDown})
	}
	keys = append(keys, tea.KeyMsg{Type: tea.KeyEnter})

	result := simulateRuntime(m, keys).(mainMenuModel)
	if result.action != "exit" {
		t.Fatalf("expected action=exit, got action=%q, mode=%q, vals.choice=%q",
			result.action, result.mode, result.vals.choice)
	}
}

func TestMainMenuModel_ShowFrameworksViaModelUpdate(t *testing.T) {
	// Installed state: navigate to "Show frameworks" (item 5 = 4 downs)
	state := InstallState{
		HasProjectConfig:  true,
		HasInstallMarker:  true,
		InstalledManifest: "alpha",
		InstalledVersion:  "1.0.0",
	}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	m := mainMenuModel{
		form:      form,
		mode:      "menu",
		state:     state,
		vals:      vals,
		manifests: testManifests(),
	}

	keys := make([]tea.Msg, 0, 5)
	for i := 0; i < 4; i++ {
		keys = append(keys, tea.KeyMsg{Type: tea.KeyDown})
	}
	keys = append(keys, tea.KeyMsg{Type: tea.KeyEnter})

	result := simulateRuntime(m, keys).(mainMenuModel)
	if result.mode != "text" {
		t.Fatalf("expected mode=text for frameworks, got mode=%q, action=%q, vals.choice=%q",
			result.mode, result.action, result.vals.choice)
	}
	if result.textTitle != "Frameworks" {
		t.Fatalf("expected title=Frameworks, got %q", result.textTitle)
	}
}

func TestMainMenuModel_ChoiceTracksDuringNavigation(t *testing.T) {
	// Verify vals.choice updates on every Down arrow through model.Update
	state := InstallState{
		HasProjectConfig:  true,
		HasInstallMarker:  true,
		InstalledManifest: "alpha",
		InstalledVersion:  "1.0.0",
	}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	var m tea.Model = mainMenuModel{
		form:  form,
		mode:  "menu",
		state: state,
		vals:  vals,
	}

	// Init
	cmd := m.Init()
	for i := 0; i < 20 && cmd != nil; i++ {
		msg := cmd()
		if msg == nil {
			break
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			break
		}
		m, cmd = m.Update(msg)
	}

	expected := []string{
		"quick-install", "guided-install", "repo-add", "repo-remove",
		"list", "state", "exit",
	}

	// Check initial value after Init
	mm := m.(mainMenuModel)
	if mm.vals.choice != expected[0] {
		t.Fatalf("after init: expected %q, got %q", expected[0], mm.vals.choice)
	}

	// Navigate through all items
	for i := 1; i < len(expected); i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		mm = m.(mainMenuModel)
		if mm.vals.choice != expected[i] {
			t.Fatalf("after down %d: expected %q, got %q", i, expected[i], mm.vals.choice)
		}
	}
}

func TestMainMenuModel_FrameworksRoundTrip_ThenExit(t *testing.T) {
	// Reproduce reported bug: show frameworks → return → select exit → exits
	state := InstallState{
		HasProjectConfig:  true,
		HasInstallMarker:  true,
		InstalledManifest: "alpha",
		InstalledVersion:  "1.0.0",
	}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	var m tea.Model = mainMenuModel{
		form:      form,
		mode:      "menu",
		state:     state,
		vals:      vals,
		manifests: testManifests(),
	}

	// Step 1: navigate to "Show frameworks" (item 5 = 4 downs) + enter
	keys := []tea.Msg{}
	for i := 0; i < 4; i++ {
		keys = append(keys, tea.KeyMsg{Type: tea.KeyDown})
	}
	keys = append(keys, tea.KeyMsg{Type: tea.KeyEnter})

	m = simulateRuntime(m, keys)
	mm := m.(mainMenuModel)
	if mm.mode != "text" {
		t.Fatalf("step 1: expected text mode, got %q", mm.mode)
	}

	// Step 2: press Enter to return to menu
	m = simulateRuntime(m, []tea.Msg{tea.KeyMsg{Type: tea.KeyEnter}})
	mm = m.(mainMenuModel)
	if mm.mode != "menu" {
		t.Fatalf("step 2: expected menu mode, got %q", mm.mode)
	}

	// Step 3: navigate to "Exit" (item 7 = 6 downs) + enter
	keys2 := []tea.Msg{}
	for i := 0; i < 6; i++ {
		keys2 = append(keys2, tea.KeyMsg{Type: tea.KeyDown})
	}
	keys2 = append(keys2, tea.KeyMsg{Type: tea.KeyEnter})

	m = simulateRuntime(m, keys2)
	mm = m.(mainMenuModel)
	if mm.action != "exit" {
		t.Fatalf("step 3: expected action=exit, got action=%q, mode=%q, choice=%q",
			mm.action, mm.mode, mm.vals.choice)
	}
}

func TestMainMenuModel_StateRoundTrip_ThenExit(t *testing.T) {
	// show state → return → exit
	state := InstallState{HasProjectConfig: true, HasInstallMarker: true}
	vals := &menuValues{}
	form := buildMainMenuForm(state, &vals.choice)
	var m tea.Model = mainMenuModel{
		form:      form,
		mode:      "menu",
		state:     state,
		vals:      vals,
		manifests: testManifests(),
	}

	// Navigate to "Show current state" (item 6 = 5 downs) + enter
	keys := make([]tea.Msg, 0, 6)
	for i := 0; i < 5; i++ {
		keys = append(keys, tea.KeyMsg{Type: tea.KeyDown})
	}
	keys = append(keys, tea.KeyMsg{Type: tea.KeyEnter})
	m = simulateRuntime(m, keys)
	mm := m.(mainMenuModel)
	if mm.mode != "text" || mm.textTitle != "Current State" {
		t.Fatalf("expected text mode with 'Current State', got mode=%q title=%q", mm.mode, mm.textTitle)
	}

	// Return to menu
	m = simulateRuntime(m, []tea.Msg{tea.KeyMsg{Type: tea.KeyEnter}})
	mm = m.(mainMenuModel)
	if mm.mode != "menu" {
		t.Fatalf("expected menu mode after return, got %q", mm.mode)
	}

	// Exit (6 downs + enter)
	keys2 := make([]tea.Msg, 0, 7)
	for i := 0; i < 6; i++ {
		keys2 = append(keys2, tea.KeyMsg{Type: tea.KeyDown})
	}
	keys2 = append(keys2, tea.KeyMsg{Type: tea.KeyEnter})
	m = simulateRuntime(m, keys2)
	mm = m.(mainMenuModel)
	if mm.action != "exit" {
		t.Fatalf("expected action=exit, got action=%q choice=%q", mm.action, mm.vals.choice)
	}
}
