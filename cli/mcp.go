package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const mcpConfigRel = ".vscode/mcp.json"

// mcpConfig represents the .vscode/mcp.json file structure.
type mcpConfig struct {
	Servers map[string]json.RawMessage `json:"servers,omitempty"`
	Inputs  json.RawMessage            `json:"inputs,omitempty"`
}

// ReadMcpConfig reads .vscode/mcp.json from the project directory.
// Returns an empty config if the file doesn't exist.
func ReadMcpConfig(projectDir string) (*mcpConfig, error) {
	path := filepath.Join(projectDir, mcpConfigRel)
	data, err := os.ReadFile(path)
	if err != nil {
		return &mcpConfig{Servers: make(map[string]json.RawMessage)}, nil
	}
	var cfg mcpConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]json.RawMessage)
	}
	return &cfg, nil
}

// WriteMcpConfig writes the .vscode/mcp.json file.
func WriteMcpConfig(projectDir string, cfg *mcpConfig) error {
	path := filepath.Join(projectDir, mcpConfigRel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// LoadMcpServerConfig reads an MCP server JSON file and returns
// the server name → config map.
func LoadMcpServerConfig(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("parse MCP config %s: %w", path, err)
	}
	return servers, nil
}

// MergeMcpServers merges managed servers into .vscode/mcp.json,
// removing previously managed servers that are no longer in the set.
func MergeMcpServers(projectDir string, managedServers map[string]json.RawMessage, previousNames []string) ([]string, error) {
	cfg, err := ReadMcpConfig(projectDir)
	if err != nil {
		return nil, err
	}

	// Remove stale managed servers
	newNames := make(map[string]bool)
	for name := range managedServers {
		newNames[name] = true
	}
	for _, oldName := range previousNames {
		if !newNames[oldName] {
			delete(cfg.Servers, oldName)
		}
	}

	// Merge in managed servers
	var injected []string
	for name, serverCfg := range managedServers {
		cfg.Servers[name] = serverCfg
		injected = append(injected, name)
	}

	if err := WriteMcpConfig(projectDir, cfg); err != nil {
		return nil, err
	}
	return injected, nil
}

// RemoveMcpServers removes the specified servers from .vscode/mcp.json.
func RemoveMcpServers(projectDir string, serverNames []string) error {
	cfg, err := ReadMcpConfig(projectDir)
	if err != nil {
		return err
	}

	for _, name := range serverNames {
		delete(cfg.Servers, name)
	}

	return WriteMcpConfig(projectDir, cfg)
}

// InjectMcpFromManifest processes MCP server files from a manifest,
// merging them into .vscode/mcp.json and returning the managed server names.
func InjectMcpFromManifest(files []ManifestFile, basePath, projectDir string, previousNames []string) ([]string, error) {
	allServers := make(map[string]json.RawMessage)

	for _, f := range files {
		cat := f.Category
		if cat == "" {
			cat = InferCategory(f.Src)
		}
		if cat != "mcp-servers" {
			continue
		}

		srcPath := filepath.Join(basePath, f.Src)
		servers, err := LoadMcpServerConfig(srcPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗  MCP %s: %s\n", f.Src, err)
			continue
		}
		for name, cfg := range servers {
			allServers[name] = cfg
		}
	}

	if len(allServers) == 0 && len(previousNames) == 0 {
		return nil, nil
	}

	return MergeMcpServers(projectDir, allServers, previousNames)
}
