package install

import (
	"encoding/json"
	"os"
)

// InstallInfo represents the install.json file stored in each app's version directory.
// Scoop's install.json only contains architecture and bucket.
type InstallInfo struct {
	Architecture string `json:"architecture"`
	Bucket       string `json:"bucket"`
}

// WriteInstallInfo writes an InstallInfo struct as JSON to the given path.
func WriteInstallInfo(path string, info *InstallInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// WriteManifest writes a manifest (as raw JSON bytes) to the given path.
// This preserves the original manifest format.
func WriteManifest(path string, m any) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
