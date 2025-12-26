package context

import (
	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// contextBuilder implements contracts.ContextBuilder for constructing context bundles for tasks.
type contextBuilder struct{}

// NewContextBuilder creates a new ContextBuilder.
func NewContextBuilder() contracts.ContextBuilder {
	return &contextBuilder{}
}

// Build constructs the context bundle for a task within a run.
// It includes:
// - Messages from outputs of all completed dependencies
// - Memory copied from run.Memory
// - Tools as an empty map (placeholder for future extensibility)
//
// Returns an error if:
// - run is nil
// - task is not found in run.Tasks
// - any dependency task is not found (dependency is skipped, not errored)
func (cb *contextBuilder) Build(run *contracts.Run, taskID contracts.TaskID) (*contracts.ContextBundle, error) {
	// Validate run
	if run == nil {
		return nil, contracts.ErrInvalidInput
	}

	// Check if task exists
	task, exists := run.Tasks[taskID]
	if !exists {
		return nil, contracts.ErrTaskNotFound
	}

	// Build the context bundle
	bundle := &contracts.ContextBundle{
		Messages: []string{},
		Memory:   make(map[string]string),
		Tools:    make(map[string]string),
	}

	// Add messages from completed dependencies
	for _, depID := range task.Deps {
		depTask, depExists := run.Tasks[depID]
		if !depExists {
			// Skip missing dependency (don't error out)
			continue
		}

		// Only include outputs from completed dependencies
		if depTask.Outputs != nil {
			if depTask.Outputs.Output != "" {
				bundle.Messages = append(bundle.Messages, depTask.Outputs.Output)
			}
		}
	}

	// Copy memory from run.Memory
	if run.Memory != nil {
		for key, value := range run.Memory {
			bundle.Memory[key] = value
		}
	}

	// Tools is empty map (placeholder for future)

	return bundle, nil
}
