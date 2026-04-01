package model

import (
	"fmt"
	"strings"
)

// ── Preset types ────────────────────────────────────────────────

// Preset is the in-memory representation of a resolved preset.
type Preset struct {
	ID          string       `json:"id"`                    // e.g. "adhoc/standard"
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Plugin      string       `json:"plugin"`                // e.g. "adhoc"
	Extends     string       `json:"extends,omitempty"`     // parent preset ID
	Files       []PresetFile `json:"files"`                 // resolved file list (after inheritance)
}

// PresetFile represents a single file entry in a resolved preset.
type PresetFile struct {
	Src         string `json:"src"`                   // full path relative to repo root (e.g. "plugins/adhoc/AGENTS.md")
	Dest        string `json:"dest"`                  // install path under .github/ (e.g. "AGENTS.md")
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	IsDir       bool   `json:"isDir,omitempty"`       // true if this entry represents a directory
}

// FormatPresetList produces a human-readable summary of presets.
func FormatPresetList(presets []Preset) string {
	var b strings.Builder
	for _, p := range presets {
		fmt.Fprintf(&b, "  %s\n", p.ID)
		fmt.Fprintf(&b, "    %s — %d files\n", p.Name, len(p.Files))
		if p.Description != "" {
			fmt.Fprintf(&b, "    %s\n", p.Description)
		}
	}
	return b.String()
}

// ── Deprecated: old manifest types, will be removed ─────────────

// Deprecated: use Preset instead.
type ManifestFile struct {
	Src         string `json:"src"`
	Dest        string `json:"dest"`
	Tier        string `json:"tier"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
}

// Deprecated: use Preset instead.
type TierDef struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// Deprecated: use Preset instead.
type Manifest struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	BasePath    string         `json:"basePath"`
	Tiers       []TierDef      `json:"tiers,omitempty"`
	Files       []ManifestFile `json:"files"`
}

// Deprecated: use FormatPresetList instead.
func FormatManifestList(manifests []Manifest) string {
	var b strings.Builder
	for _, m := range manifests {
		fmt.Fprintf(&b, "  %s\n", m.ID)
		fmt.Fprintf(&b, "    %s — %d files\n", m.Name, len(m.Files))
		if m.Description != "" {
			fmt.Fprintf(&b, "    %s\n", m.Description)
		}
	}
	return b.String()
}
