package main

import (
	"os"
	"regexp"
	"strings"
)

// ── Frontmatter version parsing ─────────────────────────────────

var (
	fmBlockRe   = regexp.MustCompile(`(?s)\A---[ \t]*\n(.*?)\n---`)
	fmVersionRe = regexp.MustCompile(`(?m)^version:\s*['"]?([^'"\n]+)['"]?\s*$`)
)

// ParseFrontmatterVersion extracts the version field from YAML
// frontmatter at the top of a file. Returns "" if not found.
func ParseFrontmatterVersion(content []byte) string {
	block := fmBlockRe.FindSubmatch(content)
	if block == nil {
		return ""
	}
	ver := fmVersionRe.FindSubmatch(block[1])
	if ver == nil {
		return ""
	}
	return strings.TrimSpace(string(ver[1]))
}

// ReadFileVersion reads a file and extracts its frontmatter version.
func ReadFileVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return ParseFrontmatterVersion(data), nil
}

// ── Per-file status ─────────────────────────────────────────────

// FileStatus describes the install/version state of a single manifest file.
type FileStatus struct {
	Dest             string `json:"dest"`
	DisplayName      string `json:"displayName"`
	Category         string `json:"category"`
	Tier             string `json:"tier"`
	Description      string `json:"description,omitempty"`
	Installed        bool   `json:"installed"`
	BundledVersion   string `json:"bundledVersion,omitempty"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	UpdateAvailable  bool   `json:"updateAvailable"`
	Skipped          bool   `json:"skipped"`
	Override         string `json:"override,omitempty"` // "pinned", "excluded", or ""
}

// ComputeFileStatuses builds a status list for every file in the manifest.
func ComputeFileStatuses(m Manifest, sidecar *repoSidecar, cfg Config, projectDir string) []FileStatus {
	installedSet := make(map[string]bool)
	if sidecar != nil {
		for _, f := range sidecar.Files {
			installedSet[f] = true
		}
	}

	result := make([]FileStatus, 0, len(m.Files))
	for _, f := range m.Files {
		destRel := ".github/" + f.Dest

		cat := f.Category
		if cat == "" {
			cat = InferCategory(f.Src)
		}

		fs := FileStatus{
			Dest:        f.Dest,
			DisplayName: fileDisplayName(f.Dest),
			Category:    cat,
			Tier:        f.Tier,
		}
		if f.Description != "" {
			fs.Description = f.Description
		}

		// Override from config
		if ov, ok := cfg.FileOverrides[f.Dest]; ok {
			fs.Override = ov
		}

		// Installed?
		fs.Installed = installedSet[destRel]

		// Bundled version
		if m.BasePath != "" {
			bv, _ := ReadFileVersion(m.BasePath + "/" + f.Src)
			fs.BundledVersion = bv
		}

		// Installed version
		if fs.Installed {
			iv, _ := ReadFileVersion(projectDir + "/" + destRel)
			fs.InstalledVersion = iv
		}

		// Update available?
		if fs.Installed && fs.BundledVersion != "" && fs.InstalledVersion != "" && fs.BundledVersion != fs.InstalledVersion {
			fs.UpdateAvailable = true
		}

		// Skipped?
		if sv, ok := cfg.SkippedVersions[f.Dest]; ok && sv == fs.BundledVersion {
			fs.Skipped = true
			fs.UpdateAvailable = false
		}

		result = append(result, fs)
	}
	return result
}

// fileDisplayName derives a short display name from a dest path.
// Examples:
//
//	"instructions/general.instructions.md" → "general"
//	"skills/go-testing/SKILL.md" → "go-testing"
//	"agents/planner.agent.md" → "planner"
//	"prompts/review.prompt.md" → "review"
func fileDisplayName(dest string) string {
	parts := strings.Split(dest, "/")
	filename := parts[len(parts)-1]
	if filename == "SKILL.md" && len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	name := strings.TrimSuffix(filename, ".md")
	for _, suffix := range []string{".instructions", ".prompt", ".agent"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return name
}
