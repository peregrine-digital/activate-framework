package model

// FindPresetByID locates a preset by ID in a slice.
func FindPresetByID(presets []Preset, presetID string) *Preset {
	for i := range presets {
		if presets[i].ID == presetID {
			return &presets[i]
		}
	}
	return nil
}

// FindPresetFile locates a file entry by dest path.
func FindPresetFile(files []PresetFile, dest string) *PresetFile {
	for i, f := range files {
		if f.Dest == dest {
			return &files[i]
		}
	}
	return nil
}

// Deprecated: use FindPresetByID instead.
func FindManifestByID(manifests []Manifest, manifestID string) *Manifest {
	for i := range manifests {
		if manifests[i].ID == manifestID {
			return &manifests[i]
		}
	}
	return nil
}

// Deprecated: use FindPresetFile instead.
func FindManifestFile(files []ManifestFile, name string) *ManifestFile {
	for i, f := range files {
		if f.Dest == name || f.Src == name {
			return &files[i]
		}
	}
	return nil
}

// ContainsString checks if a string slice contains a value.
func ContainsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
