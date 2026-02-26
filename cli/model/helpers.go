package model

// FindManifestByID locates a manifest by ID in a slice.
func FindManifestByID(manifests []Manifest, manifestID string) *Manifest {
	for i := range manifests {
		if manifests[i].ID == manifestID {
			return &manifests[i]
		}
	}
	return nil
}

// FindManifestFile locates a file entry by dest or src name.
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
