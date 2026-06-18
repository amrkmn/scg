package scoop

import (
	"os"

	jsoniter "github.com/json-iterator/go"
)

var jsonFast = jsoniter.ConfigCompatibleWithStandardLibrary

// Manifest represents a Scoop app manifest JSON file.
// Fields use `any` where the Scoop spec allows multiple types.
type Manifest struct {
	Version       string            `json:"version"`
	Description   string            `json:"description"`
	Homepage      string            `json:"homepage"`
	License       any               `json:"license"`      // string or {"identifier":..., "url":...}
	URL           any               `json:"url"`          // string or []string
	Hash          any               `json:"hash"`         // string or []string
	Bin           any               `json:"bin"`          // string | []any | map[string]string
	Depends       any               `json:"depends"`      // string | []string
	Deprecated    any               `json:"deprecated"`   // bool or string (replacement app name)
	Architecture  map[string]any    `json:"architecture"` // {"64bit":{...}, "32bit":{...}}
	EnvAddPath    any               `json:"env_add_path"` // string | []string
	EnvSet        map[string]string `json:"env_set"`
	Shortcuts     []any             `json:"shortcuts"` // each: [target, name, args?, icon?]
	ExtractDir    string            `json:"extract_dir"`
	ExtractTo     string            `json:"extract_to"`
	Persist       any               `json:"persist"` // string | []any
	Notes         any               `json:"notes"`   // string | []string
	Suggest       map[string]any    `json:"suggest"`
	Installer     any               `json:"installer"` // map with script property
	Uninstaller   any               `json:"uninstaller"`
	PreInstall    any               `json:"pre_install"`    // string or []string
	PostInstall   any               `json:"post_install"`   // string or []string
	PreUninstall  any               `json:"pre_uninstall"`  // string or []string
	PostUninstall any               `json:"post_uninstall"` // string or []string
	Comments      any               `json:"##"`
}

// ReadManifest reads and parses a Scoop manifest JSON file from disk.
func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseManifestBytes(data)
}

// ParseManifestBytes parses a Scoop manifest from raw JSON bytes.
func ParseManifestBytes(data []byte) (*Manifest, error) {
	var m Manifest
	if err := jsonFast.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// GetDependencies returns the list of dependency app names from a manifest's depends field.
func GetDependencies(depends any) []string {
	if depends == nil {
		return nil
	}
	switch v := depends.(type) {
	case string:
		return []string{v}
	case []any:
		var deps []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				deps = append(deps, s)
			}
		}
		return deps
	default:
		return nil
	}
}
