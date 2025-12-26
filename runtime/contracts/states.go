package contracts

// RunState represents the state of a run.
type RunState int

const (
	RunPending RunState = iota
	RunRunning
	RunCompleted
	RunFailed
	RunAborted
)

func (s RunState) String() string {
	switch s {
	case RunPending:
		return "pending"
	case RunRunning:
		return "running"
	case RunCompleted:
		return "completed"
	case RunFailed:
		return "failed"
	case RunAborted:
		return "aborted"
	default:
		return "unknown"
	}
}

// TaskState represents the state of a task.
type TaskState int

const (
	TaskPending TaskState = iota
	TaskReady
	TaskRunning
	TaskCompleted
	TaskFailed
	TaskSkipped
)

func (s TaskState) String() string {
	switch s {
	case TaskPending:
		return "pending"
	case TaskReady:
		return "ready"
	case TaskRunning:
		return "running"
	case TaskCompleted:
		return "completed"
	case TaskFailed:
		return "failed"
	case TaskSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}
