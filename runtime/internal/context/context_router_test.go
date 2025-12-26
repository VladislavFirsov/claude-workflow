package context

import (
	"errors"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestContextRouter_Route_Success(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID: "task-1",
				Inputs: &contracts.TaskInput{
					Prompt: "Task 1 prompt",
					Inputs: make(map[string]string),
				},
			},
			"task-2": {
				ID: "task-2",
				Inputs: &contracts.TaskInput{
					Prompt: "Task 2 prompt",
					Inputs: make(map[string]string),
				},
			},
		},
	}

	output := &contracts.TaskResult{
		Output: "Hello from Task 1",
		Outputs: map[string]string{
			"key1": "value1",
		},
	}

	err := router.Route(run, "task-1", "task-2", output)

	if err != nil {
		t.Errorf("Route() error = %v, want nil", err)
	}

	// Verify output was stored in task-2's inputs
	if run.Tasks["task-2"].Inputs.Inputs["task-1"] != "Hello from Task 1" {
		t.Errorf("Route() stored output = %v, want 'Hello from Task 1'", run.Tasks["task-2"].Inputs.Inputs["task-1"])
	}
}

func TestContextRouter_Route_NilRun(t *testing.T) {
	router := NewContextRouter()

	output := &contracts.TaskResult{
		Output: "test",
	}

	err := router.Route(nil, "task-1", "task-2", output)

	if !errors.Is(err, contracts.ErrInvalidInput) {
		t.Errorf("Route() error = %v, want ErrInvalidInput", err)
	}
}

func TestContextRouter_Route_SourceTaskNotFound(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-2": {
				ID: "task-2",
				Inputs: &contracts.TaskInput{
					Inputs: make(map[string]string),
				},
			},
		},
	}

	output := &contracts.TaskResult{
		Output: "test",
	}

	err := router.Route(run, "task-1", "task-2", output)

	if !errors.Is(err, contracts.ErrTaskNotFound) {
		t.Errorf("Route() error = %v, want ErrTaskNotFound", err)
	}
}

func TestContextRouter_Route_TargetTaskNotFound(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID: "task-1",
				Inputs: &contracts.TaskInput{
					Inputs: make(map[string]string),
				},
			},
		},
	}

	output := &contracts.TaskResult{
		Output: "test",
	}

	err := router.Route(run, "task-1", "task-2", output)

	if !errors.Is(err, contracts.ErrTaskNotFound) {
		t.Errorf("Route() error = %v, want ErrTaskNotFound", err)
	}
}

func TestContextRouter_Route_NilOutput(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID: "task-1",
				Inputs: &contracts.TaskInput{
					Inputs: make(map[string]string),
				},
			},
			"task-2": {
				ID: "task-2",
				Inputs: &contracts.TaskInput{
					Inputs: make(map[string]string),
				},
			},
		},
	}

	// Pass nil output
	err := router.Route(run, "task-1", "task-2", nil)

	if err != nil {
		t.Errorf("Route() error = %v, want nil", err)
	}

	// Verify empty string was stored for nil output
	if run.Tasks["task-2"].Inputs.Inputs["task-1"] != "" {
		t.Errorf("Route() stored output = %v, want empty string", run.Tasks["task-2"].Inputs.Inputs["task-1"])
	}
}

func TestContextRouter_Route_NilTaskInputs(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID: "task-1",
				Inputs: &contracts.TaskInput{
					Inputs: make(map[string]string),
				},
			},
			"task-2": {
				ID:     "task-2",
				Inputs: nil, // Nil TaskInput
			},
		},
	}

	output := &contracts.TaskResult{
		Output: "Hello from Task 1",
	}

	err := router.Route(run, "task-1", "task-2", output)

	if err != nil {
		t.Errorf("Route() error = %v, want nil", err)
	}

	// Verify TaskInput was created
	if run.Tasks["task-2"].Inputs == nil {
		t.Errorf("Route() did not create TaskInput")
	}

	// Verify output was stored correctly
	if run.Tasks["task-2"].Inputs.Inputs["task-1"] != "Hello from Task 1" {
		t.Errorf("Route() stored output = %v, want 'Hello from Task 1'", run.Tasks["task-2"].Inputs.Inputs["task-1"])
	}
}

func TestContextRouter_Route_NilInputsMap(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID: "task-1",
				Inputs: &contracts.TaskInput{
					Inputs: make(map[string]string),
				},
			},
			"task-2": {
				ID: "task-2",
				Inputs: &contracts.TaskInput{
					Prompt: "Task 2 prompt",
					Inputs: nil, // Nil Inputs map
				},
			},
		},
	}

	output := &contracts.TaskResult{
		Output: "Hello from Task 1",
	}

	err := router.Route(run, "task-1", "task-2", output)

	if err != nil {
		t.Errorf("Route() error = %v, want nil", err)
	}

	// Verify Inputs map was created
	if run.Tasks["task-2"].Inputs.Inputs == nil {
		t.Errorf("Route() did not create Inputs map")
	}

	// Verify output was stored correctly
	if run.Tasks["task-2"].Inputs.Inputs["task-1"] != "Hello from Task 1" {
		t.Errorf("Route() stored output = %v, want 'Hello from Task 1'", run.Tasks["task-2"].Inputs.Inputs["task-1"])
	}
}

func TestContextRouter_Route_MultipleOutputs(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID:     "task-1",
				Inputs: &contracts.TaskInput{Inputs: make(map[string]string)},
			},
			"task-2": {
				ID:     "task-2",
				Inputs: &contracts.TaskInput{Inputs: make(map[string]string)},
			},
			"task-3": {
				ID:     "task-3",
				Inputs: &contracts.TaskInput{Inputs: make(map[string]string)},
			},
		},
	}

	// Route from task-1 to task-3
	output1 := &contracts.TaskResult{Output: "Output from task-1"}
	err1 := router.Route(run, "task-1", "task-3", output1)

	if err1 != nil {
		t.Errorf("First Route() error = %v, want nil", err1)
	}

	// Route from task-2 to task-3
	output2 := &contracts.TaskResult{Output: "Output from task-2"}
	err2 := router.Route(run, "task-2", "task-3", output2)

	if err2 != nil {
		t.Errorf("Second Route() error = %v, want nil", err2)
	}

	// Verify both outputs are in task-3's inputs
	if run.Tasks["task-3"].Inputs.Inputs["task-1"] != "Output from task-1" {
		t.Errorf("Route() stored task-1 output = %v, want 'Output from task-1'", run.Tasks["task-3"].Inputs.Inputs["task-1"])
	}

	if run.Tasks["task-3"].Inputs.Inputs["task-2"] != "Output from task-2" {
		t.Errorf("Route() stored task-2 output = %v, want 'Output from task-2'", run.Tasks["task-3"].Inputs.Inputs["task-2"])
	}
}

func TestContextRouter_Route_OverwriteExistingInput(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID: "task-1",
				Inputs: &contracts.TaskInput{
					Inputs: make(map[string]string),
				},
			},
			"task-2": {
				ID: "task-2",
				Inputs: &contracts.TaskInput{
					Inputs: map[string]string{
						"task-1": "Old output",
					},
				},
			},
		},
	}

	output := &contracts.TaskResult{
		Output: "New output",
	}

	err := router.Route(run, "task-1", "task-2", output)

	if err != nil {
		t.Errorf("Route() error = %v, want nil", err)
	}

	// Verify output was overwritten
	if run.Tasks["task-2"].Inputs.Inputs["task-1"] != "New output" {
		t.Errorf("Route() stored output = %v, want 'New output'", run.Tasks["task-2"].Inputs.Inputs["task-1"])
	}
}

func TestContextRouter_Route_EmptyOutput(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID:     "task-1",
				Inputs: &contracts.TaskInput{Inputs: make(map[string]string)},
			},
			"task-2": {
				ID:     "task-2",
				Inputs: &contracts.TaskInput{Inputs: make(map[string]string)},
			},
		},
	}

	// Output with empty string
	output := &contracts.TaskResult{
		Output: "",
	}

	err := router.Route(run, "task-1", "task-2", output)

	if err != nil {
		t.Errorf("Route() error = %v, want nil", err)
	}

	// Verify empty string was stored
	if run.Tasks["task-2"].Inputs.Inputs["task-1"] != "" {
		t.Errorf("Route() stored output = %v, want empty string", run.Tasks["task-2"].Inputs.Inputs["task-1"])
	}
}

func TestContextRouter_Route_PreservesExistingInputData(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID:     "task-1",
				Inputs: &contracts.TaskInput{Inputs: make(map[string]string)},
			},
			"task-2": {
				ID: "task-2",
				Inputs: &contracts.TaskInput{
					Prompt: "Original prompt",
					Inputs: map[string]string{
						"existing-key": "existing-value",
					},
				},
			},
		},
	}

	output := &contracts.TaskResult{
		Output: "New output",
	}

	err := router.Route(run, "task-1", "task-2", output)

	if err != nil {
		t.Errorf("Route() error = %v, want nil", err)
	}

	// Verify existing data is preserved
	if run.Tasks["task-2"].Inputs.Prompt != "Original prompt" {
		t.Errorf("Route() modified Prompt = %v, want 'Original prompt'", run.Tasks["task-2"].Inputs.Prompt)
	}

	if run.Tasks["task-2"].Inputs.Inputs["existing-key"] != "existing-value" {
		t.Errorf("Route() modified existing input = %v, want 'existing-value'", run.Tasks["task-2"].Inputs.Inputs["existing-key"])
	}

	// Verify new data was added
	if run.Tasks["task-2"].Inputs.Inputs["task-1"] != "New output" {
		t.Errorf("Route() stored output = %v, want 'New output'", run.Tasks["task-2"].Inputs.Inputs["task-1"])
	}
}

func TestContextRouter_Route_WithComplexOutput(t *testing.T) {
	router := NewContextRouter()

	run := &contracts.Run{
		ID: "run-1",
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {
				ID:     "task-1",
				Inputs: &contracts.TaskInput{Inputs: make(map[string]string)},
			},
			"task-2": {
				ID:     "task-2",
				Inputs: &contracts.TaskInput{Inputs: make(map[string]string)},
			},
		},
	}

	output := &contracts.TaskResult{
		Output: "Complex output with special characters: !@#$%^&*()_+-=[]{}|;:',.<>?/",
		Outputs: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		Metadata: map[string]string{
			"source": "task-1",
		},
	}

	err := router.Route(run, "task-1", "task-2", output)

	if err != nil {
		t.Errorf("Route() error = %v, want nil", err)
	}

	// Verify complex output was stored correctly
	expectedOutput := "Complex output with special characters: !@#$%^&*()_+-=[]{}|;:',.<>?/"
	if run.Tasks["task-2"].Inputs.Inputs["task-1"] != expectedOutput {
		t.Errorf("Route() stored output = %v, want %v", run.Tasks["task-2"].Inputs.Inputs["task-1"], expectedOutput)
	}
}

func TestNewContextRouter(t *testing.T) {
	router := NewContextRouter()

	if router == nil {
		t.Errorf("NewContextRouter() returned nil")
	}

	// Verify it implements the interface
	var _ contracts.ContextRouter = router
}
