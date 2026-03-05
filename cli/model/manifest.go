package model

import (
	"fmt"
	"strings"
)

// ManifestFile represents a single file entry in a manifest.
type ManifestFile struct {
	Src         string `json:"src"`
	Dest        string `json:"dest"`
	Tier        string `json:"tier"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
}

// TierDef is a tier definition within a manifest.
type TierDef struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// Manifest is the fully resolved in-memory representation.
type Manifest struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	BasePath    string         `json:"basePath"` // resolved absolute path (local) or relative prefix (remote)
	Tiers       []TierDef      `json:"tiers,omitempty"`
	Files       []ManifestFile `json:"files"`
}

// FormatManifestList produces a human-readable summary of manifests.
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
