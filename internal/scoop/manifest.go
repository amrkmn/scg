package scoop

import (
	"encoding/json"
	"os"
)

// Manifest represents a Scoop app manifest JSON file.
// Fields use `any` where the Scoop spec allows multiple types.
type Manifest struct {
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Homepage     string            `json:"homepage"`
	License      any               `json:"license"`      // string or {"identifier":..., "url":...}
	Bin          any               `json:"bin"`          // string | []any | map[string]string
	Depends      any               `json:"depends"`      // string | []string
	Deprecated   any               `json:"deprecated"`   // bool or string (replacement app name)
	Architecture map[string]any    `json:"architecture"` // {"64bit":{...}, "32bit":{...}}
	EnvAddPath   any               `json:"env_add_path"` // string | []string
	EnvSet       map[string]string `json:"env_set"`
	Shortcuts    []any             `json:"shortcuts"` // each: [target, name, args?, icon?]
	Persist      any               `json:"persist"`   // string | []any
	Notes        any               `json:"notes"`     // string | []string
	Suggest      map[string]any    `json:"suggest"`
	Comments     any               `json:"##"`
}

// ReadManifest reads and parses a Scoop manifest JSON file from disk.
func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
