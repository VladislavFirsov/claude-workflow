package context

import (
	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// contextRouter routes context between tasks, passing output from one task to another.
type contextRouter struct{}

// NewContextRouter creates a new ContextRouter.
func NewContextRouter() contracts.ContextRouter {
	return &contextRouter{}
}

// Route passes output from one task to another by storing the source task's output
// in the target task's Inputs.Inputs map, keyed by the source task ID.
// It validates that both tasks exist in the run and handles nil maps gracefully.
func (cr *contextRouter) Route(run *contracts.Run, from contracts.TaskID, to contracts.TaskID, output *contracts.TaskResult) error {
	// Validate inputs
	if run == nil {
		return contracts.ErrInvalidInput
	}

	// Validate source task exists
	_, ok := run.Tasks[from]
	if !ok {
		return contracts.ErrTaskNotFound
	}

	// Validate target task exists
	toTask, ok := run.Tasks[to]
	if !ok {
		return contracts.ErrTaskNotFound
	}

	// Initialize target task inputs if nil
	if toTask.Inputs == nil {
		toTask.Inputs = &contracts.TaskInput{}
	}

	// Initialize Inputs map if nil
	if toTask.Inputs.Inputs == nil {
		toTask.Inputs.Inputs = make(map[string]string)
	}

	// Store the output in the target task's Inputs map, keyed by source task ID
	var outputValue string
	if output != nil {
		outputValue = output.Output
	}

	toTask.Inputs.Inputs[string(from)] = outputValue

	return nil
}
