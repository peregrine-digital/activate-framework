package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/peregrine-digital/activate-framework/cli/commands"
	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// ── Shared test helpers (used by all screens test files) ────────

func setupTestStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := storage.ActivateBaseDir
	storage.ActivateBaseDir = dir
	t.Cleanup(func() { storage.ActivateBaseDir = old })
	return dir
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

// ── File-specific test helpers ──────────────────────────────────

// isolatedFileSvc creates an ActivateService with test isolation.
func isolatedFileSvc(t *testing.T) *commands.ActivateService {
	t.Helper()
	dir := setupTestStore(t)
	return &commands.ActivateService{Config: model.Config{Manifest: "alpha", Tier: "standard"}, ProjectDir: dir}
}

func testFileStatuses() []model.FileStatus {
	return []model.FileStatus{
		{Dest: "instructions/setup.md", DisplayName: "setup", Category: "Instructions",
			Installed: true, BundledVersion: "1.0.0", InstalledVersion: "1.0.0"},
		{Dest: "instructions/security.md", DisplayName: "security", Category: "Instructions",
			Installed: true, BundledVersion: "1.3.0", InstalledVersion: "1.0.0", UpdateAvailable: true},
		{Dest: "agents/planner.md", DisplayName: "planner", Category: "Agents",
			Installed: false, BundledVersion: "2.0.0"},
		{Dest: "skills/tdd/SKILL.md", DisplayName: "tdd", Category: "Skills",
			Installed: false, BundledVersion: "1.0.0", Override: "excluded"},
		{Dest: "prompts/build.md", DisplayName: "build", Category: "Prompts",
			Installed: true, BundledVersion: "1.0.0", InstalledVersion: "1.0.0", Override: "pinned"},
		{Dest: "instructions/outdated-skipped.md", DisplayName: "outdated-skipped", Category: "Instructions",
			Installed: true, BundledVersion: "2.0.0", InstalledVersion: "1.0.0",
			UpdateAvailable: true, Skipped: true},
	}
}

func optionValues(opts []huh.Option[string]) []string {
	values := make([]string, len(opts))
	for i, o := range opts {
		values[i] = o.Value
	}
	return values
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, v := range values {
		if v == want {
			return
		}
	}
	t.Fatalf("expected %q in %v", want, values)
}

func assertNotContains(t *testing.T, values []string, notWant string) {
	t.Helper()
	for _, v := range values {
		if v == notWant {
			t.Fatalf("did not expect %q in %v", notWant, values)
		}
	}
}

// ── File browser form builder ───────────────────────────────────

func TestGroupFilesByCategory(t *testing.T) {
	files := testFileStatuses()
	groups := groupFilesByCategory(files)

	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}

	// First group should be Instructions (first seen category)
	if groups[0].category != "Instructions" {
		t.Fatalf("expected first group=Instructions, got %q", groups[0].category)
	}
	if len(groups[0].files) != 3 {
		t.Fatalf("expected 3 files in Instructions, got %d", len(groups[0].files))
	}
}

func TestFileStatusIcon(t *testing.T) {
	tests := []struct {
		name string
		fs   model.FileStatus
		want string
	}{
		{"installed current", model.FileStatus{Installed: true}, "✓"},
		{"update available", model.FileStatus{Installed: true, UpdateAvailable: true}, "⬆"},
		{"skipped update", model.FileStatus{Installed: true, UpdateAvailable: true, Skipped: true}, "⏭"},
		{"not installed", model.FileStatus{}, "○"},
		{"excluded", model.FileStatus{Override: "excluded"}, "🚫"},
		{"pinned", model.FileStatus{Override: "pinned", Installed: true}, "📌"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileStatusIcon(tt.fs)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestFileVersionLabel(t *testing.T) {
	tests := []struct {
		name string
		fs   model.FileStatus
		want string
	}{
		{"installed current", model.FileStatus{Installed: true, InstalledVersion: "1.0.0"}, "1.0.0"},
		{"update available", model.FileStatus{Installed: true, InstalledVersion: "1.0.0",
			BundledVersion: "2.0.0", UpdateAvailable: true}, "1.0.0 → 2.0.0"},
		{"not installed", model.FileStatus{BundledVersion: "1.0.0"}, "1.0.0"},
		{"excluded", model.FileStatus{Override: "excluded"}, "excluded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileVersionLabel(tt.fs)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestFileStatusLine(t *testing.T) {
	fs := model.FileStatus{
		Installed: true, InstalledVersion: "1.0.0", BundledVersion: "2.0.0",
		UpdateAvailable: true, Skipped: true,
	}
	line := fileStatusLine(fs)
	if !strings.Contains(line, "update available") {
		t.Fatal("expected 'update available' in line")
	}
	if !strings.Contains(line, "version skipped") {
		t.Fatal("expected 'version skipped' in line")
	}
}

func TestFormatFileOption(t *testing.T) {
	fs := model.FileStatus{
		Dest: "instructions/setup.md", DisplayName: "setup",
		Category: "Instructions", Installed: true,
		InstalledVersion: "1.0.0", BundledVersion: "1.0.0",
	}
	label := formatFileOption(fs)
	if !strings.Contains(label, "✓") {
		t.Fatal("expected ✓ in label")
	}
	if !strings.Contains(label, "setup") {
		t.Fatal("expected 'setup' in label")
	}
}

// ── File actions ────────────────────────────────────────────────

func TestFileActionsForStatus_InstalledCurrent(t *testing.T) {
	fs := model.FileStatus{Installed: true, InstalledVersion: "1.0.0", BundledVersion: "1.0.0"}
	opts := fileActionsForStatus(fs)

	values := optionValues(opts)
	assertContains(t, values, "diff")
	assertContains(t, values, "uninstall")
	assertContains(t, values, "pin")
	assertContains(t, values, "exclude")
	assertContains(t, values, "back")
	assertNotContains(t, values, "update")
	assertNotContains(t, values, "skip")
}

func TestFileActionsForStatus_InstalledOutdated(t *testing.T) {
	fs := model.FileStatus{
		Installed: true, InstalledVersion: "1.0.0",
		BundledVersion: "2.0.0", UpdateAvailable: true,
	}
	opts := fileActionsForStatus(fs)
	values := optionValues(opts)
	assertContains(t, values, "update")
	assertContains(t, values, "diff")
	assertContains(t, values, "skip")
	assertContains(t, values, "uninstall")
}

func TestFileActionsForStatus_NotInstalled(t *testing.T) {
	fs := model.FileStatus{BundledVersion: "1.0.0"}
	opts := fileActionsForStatus(fs)
	values := optionValues(opts)
	assertContains(t, values, "install")
	assertContains(t, values, "exclude")
	assertContains(t, values, "back")
	assertNotContains(t, values, "uninstall")
	assertNotContains(t, values, "diff")
}

func TestFileActionsForStatus_Excluded(t *testing.T) {
	fs := model.FileStatus{Override: "excluded"}
	opts := fileActionsForStatus(fs)
	values := optionValues(opts)
	assertContains(t, values, "clear-override")
	assertContains(t, values, "back")
	if len(values) != 2 {
		t.Fatalf("expected only 2 options for excluded, got %d: %v", len(values), values)
	}
}

func TestFileActionsForStatus_Pinned(t *testing.T) {
	fs := model.FileStatus{Installed: true, InstalledVersion: "1.0.0", Override: "pinned"}
	opts := fileActionsForStatus(fs)
	values := optionValues(opts)
	assertContains(t, values, "clear-override")
	assertNotContains(t, values, "pin")
	assertNotContains(t, values, "exclude")
}

// ── Browse form navigation ──────────────────────────────────────

func TestBuildFileBrowseForm_IncludesBackOption(t *testing.T) {
	files := testFileStatuses()
	selected := ""
	form := buildFileBrowseForm(files, &selected)
	form.Init()

	// Navigate to last item (should be _back)
	for i := 0; i < len(files); i++ {
		form = sendKey(form, tea.KeyDown)
	}
	if selected != "_back" {
		t.Fatalf("expected last item=_back, got %q", selected)
	}
}

func TestBuildFileBrowseForm_NavigatesToFiles(t *testing.T) {
	files := testFileStatuses()
	selected := ""
	form := buildFileBrowseForm(files, &selected)
	form.Init()

	// Files are grouped by category: Instructions (3), Agents (1), Skills (1), Prompts (1)
	// Instructions: setup, security, outdated-skipped
	// Agents: planner
	// Skills: tdd
	// Prompts: build
	expectedOrder := []string{
		"instructions/setup.md",
		"instructions/security.md",
		"instructions/outdated-skipped.md",
		"agents/planner.md",
		"skills/tdd/SKILL.md",
		"prompts/build.md",
		"_back",
	}

	if selected != expectedOrder[0] {
		t.Fatalf("expected first file=%q, got %q", expectedOrder[0], selected)
	}

	for i := 1; i < len(expectedOrder); i++ {
		form = sendKey(form, tea.KeyDown)
		if selected != expectedOrder[i] {
			t.Fatalf("after down %d: expected %q, got %q", i, expectedOrder[i], selected)
		}
	}
}

func TestBuildFileActionsForm_Navigation(t *testing.T) {
	fs := model.FileStatus{
		Installed: true, InstalledVersion: "1.0.0",
		BundledVersion: "2.0.0", UpdateAvailable: true,
	}
	action := ""
	form := buildFileActionsForm(fs, &action)
	form.Init()

	// First option should be "update"
	if action != "update" {
		t.Fatalf("expected first action=update, got %q", action)
	}

	// Navigate to "diff"
	form = sendKey(form, tea.KeyDown)
	if action != "diff" {
		t.Fatalf("expected diff, got %q", action)
	}
}

// ── File browser model ──────────────────────────────────────────

func TestFileBrowserModel_BrowseToBack(t *testing.T) {
	files := testFileStatuses()
	vals := &fileBrowserValues{}
	form := buildFileBrowseForm(files, &vals.selectedFile)

	m := fileBrowserModel{
		files: files,
		vals:  vals,
		form:  form,
		mode:  "browse",
	}

	// Navigate to last item (back) and select
	keys := make([]tea.Msg, 0, len(files)+1)
	for i := 0; i < len(files); i++ {
		keys = append(keys, tea.KeyMsg{Type: tea.KeyDown})
	}
	keys = append(keys, tea.KeyMsg{Type: tea.KeyEnter})

	result := simulateRuntime(m, keys).(fileBrowserModel)
	if !result.done {
		t.Fatal("expected done=true after selecting back")
	}
}

func TestFileBrowserModel_EscFromBrowseExits(t *testing.T) {
	files := testFileStatuses()
	vals := &fileBrowserValues{}
	form := buildFileBrowseForm(files, &vals.selectedFile)

	m := fileBrowserModel{
		files: files,
		vals:  vals,
		form:  form,
		mode:  "browse",
	}

	result := simulateRuntime(m, []tea.Msg{
		tea.KeyMsg{Type: tea.KeyEscape},
	}).(fileBrowserModel)
	if !result.done {
		t.Fatal("expected done=true after esc in browse mode")
	}
}

func TestFileBrowserModel_ViewContainsBanner(t *testing.T) {
	files := testFileStatuses()
	vals := &fileBrowserValues{}
	form := buildFileBrowseForm(files, &vals.selectedFile)

	svc := isolatedFileSvc(t)

	m := fileBrowserModel{
		files: files,
		vals:  vals,
		form:  form,
		mode:  "browse",
		svc:   svc,
	}

	view := m.View()
	if !strings.Contains(view, "Manage Files") {
		t.Fatal("expected 'Manage Files' in view")
	}
	if !strings.Contains(view, "files") {
		t.Fatal("expected file count in subtitle")
	}
}

func TestFileBrowserModel_TextModeRoundTrip(t *testing.T) {
	files := testFileStatuses()
	vals := &fileBrowserValues{}
	form := buildFileBrowseForm(files, &vals.selectedFile)

	m := fileBrowserModel{
		files:     files,
		vals:      vals,
		form:      form,
		mode:      "text",
		textTitle: "Test",
		textBody:  "body",
		svc:       isolatedFileSvc(t),
	}

	// Verify text mode view content
	view := m.View()
	if !strings.Contains(view, "Test") {
		t.Fatal("expected title in text view")
	}
	if !strings.Contains(view, "body") {
		t.Fatal("expected body in text view")
	}

	// Enter in text mode returns to browse
	// (switchToBrowse calls svc.GetState which returns empty with this stub)
	result := simulateRuntime(m, []tea.Msg{
		tea.KeyMsg{Type: tea.KeyEnter},
	}).(fileBrowserModel)

	if result.mode != "browse" {
		t.Fatalf("expected mode=browse after enter in text, got %q", result.mode)
	}
}
