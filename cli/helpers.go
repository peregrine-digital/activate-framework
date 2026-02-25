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
