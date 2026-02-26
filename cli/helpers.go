package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// resolveExeDir returns the directory containing the running binary.
func resolveExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func findManifestByID(manifests []Manifest, manifestID string) *Manifest {
	for i := range manifests {
		if manifests[i].ID == manifestID {
			return &manifests[i]
		}
	}
	return nil
}

func findManifestFile(files []ManifestFile, name string) *ManifestFile {
	for i, f := range files {
		if f.Dest == name || f.Src == name {
			return &files[i]
		}
	}
	return nil
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
