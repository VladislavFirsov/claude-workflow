package contracts

// Run represents a single execution run containing multiple tasks.
type Run struct {
	ID        RunID
	State     RunState
	Policy    RunPolicy
	DAG       *DAG
	Tasks     map[TaskID]*Task
	Usage     Usage
	Memory    map[string]string // short-term memory for the run
	CreatedAt Timestamp
	UpdatedAt Timestamp
}

// Task represents a single unit of work within a run.
type Task struct {
	ID           TaskID
	State        TaskState
	Inputs       *TaskInput
	Deps         []TaskID
	Outputs      *TaskResult
	Error        *TaskError
	Model        ModelID
	EstimatedUse Usage
	ActualUse    Usage
}

// DAG represents the directed acyclic graph of task dependencies.
type DAG struct {
	Nodes map[TaskID]*DAGNode
	Edges map[TaskID][]TaskID
}

// DAGNode represents a node in the dependency graph.
type DAGNode struct {
	ID      TaskID
	Deps    []TaskID
	Next    []TaskID
	Pending int
}

// Usage represents token and cost usage.
type Usage struct {
	Tokens TokenCount
	Cost   Cost
}

// Cost represents a monetary cost.
type Cost struct {
	Amount   float64
	Currency Currency
}

// TaskInput represents the input to a task.
type TaskInput struct {
	Prompt   string
	Inputs   map[string]string
	Metadata map[string]string
}

// TaskResult represents the output of a completed task.
type TaskResult struct {
	Output   string
	Outputs  map[string]string
	Usage    Usage
	Metadata map[string]string
}

// TaskError represents an error that occurred during task execution.
type TaskError struct {
	Code    string
	Message string
}

// ContextBundle represents the context passed to a task.
type ContextBundle struct {
	Messages []string
	Memory   map[string]string
	Tools    map[string]string
}

// ContextPolicy defines how context should be managed.
type ContextPolicy struct {
	MaxTokens TokenCount
	Strategy  string
	KeepLastN int
	// TruncateTo removed - out of scope V1
}

// RunPolicy defines execution constraints for a run.
type RunPolicy struct {
	TimeoutMs      int64
	MaxParallelism int
	BudgetLimit    Cost
	ContextPolicy  ContextPolicy
}
