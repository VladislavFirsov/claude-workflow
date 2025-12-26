package contracts

// =============================================================================
// Orchestration Interfaces
// =============================================================================

// Scheduler determines which tasks are ready to execute and tracks completion.
type Scheduler interface {
	// NextReady returns task IDs that are ready to execute (all deps satisfied).
	NextReady(run *Run) ([]TaskID, error)

	// MarkComplete marks a task as completed and updates the run state.
	MarkComplete(run *Run, taskID TaskID, result *TaskResult) error
}

// DependencyResolver builds and validates the task dependency graph.
type DependencyResolver interface {
	// BuildDAG constructs a DAG from a list of tasks.
	BuildDAG(tasks []Task) (*DAG, error)

	// Validate checks the DAG for cycles and missing dependencies.
	Validate(dag *DAG) error
}

// ParallelExecutor executes tasks with bounded concurrency.
type ParallelExecutor interface {
	// Execute runs a task and returns its result.
	Execute(run *Run, taskID TaskID) (*TaskResult, error)
}

// QueueManager manages the queue of tasks ready for execution.
type QueueManager interface {
	// Enqueue adds a task to the ready queue.
	Enqueue(taskID TaskID)

	// Dequeue removes and returns the next task from the queue.
	Dequeue() (TaskID, bool)

	// Len returns the number of tasks in the queue.
	Len() int
}

// =============================================================================
// Cost Control Interfaces
// =============================================================================

// TokenEstimator estimates the number of tokens for a task before execution.
type TokenEstimator interface {
	// Estimate returns the estimated token count for a task.
	Estimate(input *TaskInput, ctx *ContextBundle) (TokenCount, error)
}

// CostCalculator calculates the cost based on token usage and model.
type CostCalculator interface {
	// Estimate returns the estimated cost for the given tokens and model.
	Estimate(tokens TokenCount, model ModelID) (Cost, error)
}

// BudgetEnforcer enforces budget limits for runs.
type BudgetEnforcer interface {
	// Allow checks if the estimated cost is within budget. Returns error if not.
	Allow(run *Run, estimate Cost) error

	// Record records actual cost and updates the run usage.
	Record(run *Run, actual Cost) error
}

// UsageTracker tracks token and cost usage for a run.
type UsageTracker interface {
	// Add adds usage to the run's total.
	Add(run *Run, usage Usage)

	// Snapshot returns the current usage for the run.
	Snapshot(run *Run) Usage
}

// =============================================================================
// Context Management Interfaces
// =============================================================================

// ContextBuilder builds the context bundle for a task.
type ContextBuilder interface {
	// Build constructs the context bundle for a task within a run.
	Build(run *Run, taskID TaskID) (*ContextBundle, error)
}

// ContextCompactor compacts context to fit within token limits.
type ContextCompactor interface {
	// Compact reduces the context bundle according to the policy.
	Compact(bundle *ContextBundle, policy ContextPolicy) (*ContextBundle, error)
}

// ContextRouter routes context between tasks.
type ContextRouter interface {
	// Route passes output from one task to another.
	Route(run *Run, from TaskID, to TaskID, output *TaskResult) error
}

// MemoryManager manages short-term memory within a run.
type MemoryManager interface {
	// Get retrieves a value from memory.
	Get(run *Run, key string) (string, bool)

	// Put stores a value in memory.
	Put(run *Run, key string, value string)
}
