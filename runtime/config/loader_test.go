package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_LoadFromBytes_ValidJSON(t *testing.T) {
	l := NewLoader()
	data := []byte(`{
		"workflow": {
			"name": "test-flow",
			"steps": [
				{"id": "a", "role": "spec-analyst"},
				{"id": "b", "role": "spec-architect", "depends_on": ["a"]},
				{"id": "c", "role": "spec-developer", "depends_on": ["b"]},
				{"id": "d", "role": "spec-validator", "depends_on": ["c"]}
			]
		}
	}`)

	cfg, err := l.LoadFromBytes(data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Workflow.Name != "test-flow" {
		t.Fatalf("expected name=test-flow, got %s", cfg.Workflow.Name)
	}

	if len(cfg.Workflow.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(cfg.Workflow.Steps))
	}
}

func TestLoader_LoadFromBytes_EmptyData(t *testing.T) {
	l := NewLoader()
	_, err := l.LoadFromBytes([]byte{})
	if !errors.Is(err, ErrConfigEmpty) {
		t.Fatalf("expected ErrConfigEmpty, got %v", err)
	}
}

func TestLoader_LoadFromBytes_InvalidJSON(t *testing.T) {
	l := NewLoader()
	data := []byte(`{invalid json}`)

	_, err := l.LoadFromBytes(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	// Check that underlying error is json.SyntaxError
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected json.SyntaxError, got %T: %v", err, err)
	}
}

func TestLoader_LoadFromBytes_EmptyObject(t *testing.T) {
	l := NewLoader()
	// Empty JSON object {} should fail validation (name empty, no steps)
	data := []byte(`{}`)

	_, err := l.LoadFromBytes(data)
	if !errors.Is(err, ErrWorkflowNameEmpty) {
		t.Fatalf("expected ErrWorkflowNameEmpty for empty object, got %v", err)
	}
}

func TestLoader_LoadFromBytes_EmptyWorkflow(t *testing.T) {
	l := NewLoader()
	// Workflow with name but no steps
	data := []byte(`{"workflow": {"name": "test"}}`)

	_, err := l.LoadFromBytes(data)
	if !errors.Is(err, ErrNoSteps) {
		t.Fatalf("expected ErrNoSteps, got %v", err)
	}
}

func TestLoader_LoadFromBytes_WithOutputs(t *testing.T) {
	l := NewLoader()
	data := []byte(`{
		"workflow": {
			"name": "output-flow",
			"steps": [
				{"id": "analysis", "role": "spec-analyst", "outputs": ["requirements.md", "user-stories.md"]},
				{"id": "architecture", "role": "spec-architect", "depends_on": ["analysis"], "outputs": ["architecture.md"]},
				{"id": "implementation", "role": "spec-developer", "depends_on": ["architecture"]},
				{"id": "validation", "role": "spec-validator", "depends_on": ["implementation"]}
			]
		}
	}`)

	cfg, err := l.LoadFromBytes(data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(cfg.Workflow.Steps[0].Outputs) != 2 {
		t.Fatalf("expected 2 outputs for first step, got %d", len(cfg.Workflow.Steps[0].Outputs))
	}
}

func TestLoader_LoadFromFile_NotFound(t *testing.T) {
	l := NewLoader()
	_, err := l.LoadFromFile("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}

	// Check that underlying error is file not found (os.ErrNotExist in chain)
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("expected os.PathError in chain, got %v", err)
	}
	if !os.IsNotExist(pathErr) {
		t.Fatalf("expected os.IsNotExist to be true, got error: %v", pathErr)
	}
}

func TestLoader_LoadFromFile_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "workflow.json")

	data := []byte(`{
		"workflow": {
			"name": "file-test",
			"steps": [
				{"id": "a", "role": "spec-analyst"},
				{"id": "b", "role": "spec-architect", "depends_on": ["a"]},
				{"id": "c", "role": "spec-developer", "depends_on": ["b"]},
				{"id": "d", "role": "spec-validator", "depends_on": ["c"]}
			]
		}
	}`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	l := NewLoader()
	cfg, err := l.LoadFromFile(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Workflow.Name != "file-test" {
		t.Fatalf("expected name=file-test, got %s", cfg.Workflow.Name)
	}
}

func TestLoader_LoadFromFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(path, []byte(`{broken`), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadFromFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON file")
	}

	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected json.SyntaxError in chain, got %v", err)
	}
}

func TestLoader_LoadFromFile_ValidationError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid-workflow.json")

	// Valid JSON but invalid workflow (cycle)
	data := []byte(`{
		"workflow": {
			"name": "cycle-test",
			"steps": [
				{"id": "a", "role": "spec-analyst", "depends_on": ["b"]},
				{"id": "b", "role": "spec-architect", "depends_on": ["a"]}
			]
		}
	}`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadFromFile(path)
	if !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
}
