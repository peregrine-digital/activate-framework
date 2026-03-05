package model

import (
	"regexp"
	"strings"
)

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

// FileDisplayName derives a short display name from a dest path.
func FileDisplayName(dest string) string {
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
