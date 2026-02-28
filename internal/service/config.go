package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfigService manages the scg/scoop configuration file.
type ConfigService struct {
	ctx AppContext
}

// NewConfigService creates a ConfigService.
func NewConfigService(ctx AppContext) *ConfigService {
	return &ConfigService{ctx: ctx}
}

// configPath returns the path to the config file.
func (s *ConfigService) configPath() string {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		profile = os.Getenv("HOME")
	}
	return filepath.Join(profile, ".config", "scoop", "config.json")
}

// Load reads and parses the config file. Returns an empty map on any error.
func (s *ConfigService) Load() (map[string]any, error) {
	data, err := os.ReadFile(s.configPath())
	if err != nil {
		return map[string]any{}, nil
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return map[string]any{}, nil
	}
	return config, nil
}

// Save writes the config map to disk as formatted JSON.
func (s *ConfigService) Save(config map[string]any) error {
	path := s.configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Get returns the value for key and whether it exists.
func (s *ConfigService) Get(key string) (any, bool) {
	config, _ := s.Load()
	val, ok := config[key]
	return val, ok
}

// Set sets key to value in the config file.
func (s *ConfigService) Set(key string, value any) error {
	config, _ := s.Load()
	config[key] = value
	return s.Save(config)
}

// Delete removes key from the config file.
func (s *ConfigService) Delete(key string) error {
	config, _ := s.Load()
	delete(config, key)
	return s.Save(config)
}

// CoerceValue converts a string representation to the most appropriate Go type.
// "true"/"false" -> bool, "null" -> nil, numeric -> int or float64, else string.
func CoerceValue(value string) any {
	switch strings.ToLower(value) {
	case "true":
		return true
	case "false":
		return false
	case "null", "undefined":
		return nil
	}
	// Try integer.
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return n
	}
	// Try float.
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return value
}
