package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Loader loads and parses workflow configuration files.
type Loader struct{}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	return &Loader{}
}

// LoadFromFile loads and parses a workflow configuration from a JSON file.
// Returns the validated WorkflowConfig or an error.
// File errors are wrapped with context (use os.IsNotExist to check for missing file).
func (l *Loader) LoadFromFile(path string) (*WorkflowConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg, err := l.LoadFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("loading config %s: %w", path, err)
	}

	return cfg, nil
}

// LoadFromBytes parses workflow configuration from raw JSON bytes.
// Returns the validated WorkflowConfig or an error.
// Empty data (len==0) returns ErrConfigEmpty.
// Parse errors are wrapped (use json.SyntaxError to check for parse failures).
func (l *Loader) LoadFromBytes(data []byte) (*WorkflowConfig, error) {
	if len(data) == 0 {
		return nil, ErrConfigEmpty
	}

	var config WorkflowConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	// Validate the configuration
	validator := NewValidator()
	if err := validator.Validate(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
