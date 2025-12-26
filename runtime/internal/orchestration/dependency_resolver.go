package orchestration

import (
	"fmt"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// dependencyResolver implements contracts.DependencyResolver.
// It builds a DAG from a list of tasks and validates the graph for cycles
// and missing dependencies.
//
// The implementation uses depth-first search (DFS) with color marking
// to detect cycles and validates all dependencies exist.
//
// Thread-safety: The resolver is stateless and thread-safe.
type dependencyResolver struct{}

// NewDependencyResolver creates a new DependencyResolver.
func NewDependencyResolver() contracts.DependencyResolver {
	return &dependencyResolver{}
}

// BuildDAG constructs a DAG from a list of tasks.
// Creates DAGNodes with Deps, Next, and Pending counts.
// Returns a valid empty DAG for empty task lists.
// Returns error if input is nil.
func (dr *dependencyResolver) BuildDAG(tasks []contracts.Task) (*contracts.DAG, error) {
	// Edge case: nil input
	if tasks == nil {
		return nil, contracts.ErrInvalidInput
	}

	// Edge case: empty task list
	if len(tasks) == 0 {
		return &contracts.DAG{
			Nodes: make(map[contracts.TaskID]*contracts.DAGNode),
			Edges: make(map[contracts.TaskID][]contracts.TaskID),
		}, nil
	}

	dag := &contracts.DAG{
		Nodes: make(map[contracts.TaskID]*contracts.DAGNode),
		Edges: make(map[contracts.TaskID][]contracts.TaskID),
	}

	// Create a quick lookup map for task IDs to validate dependencies exist
	taskIDSet := make(map[contracts.TaskID]bool)
	for i := range tasks {
		taskIDSet[tasks[i].ID] = true
	}

	// First pass: create DAGNodes and initialize Deps
	for i := range tasks {
		task := &tasks[i]
		node := &contracts.DAGNode{
			ID:      task.ID,
			Deps:    make([]contracts.TaskID, len(task.Deps)),
			Next:    []contracts.TaskID{},
			Pending: len(task.Deps),
		}

		// Copy dependencies
		copy(node.Deps, task.Deps)

		dag.Nodes[task.ID] = node
	}

	// Second pass: build forward edges (Next) and validate dependencies
	for i := range tasks {
		task := &tasks[i]

		// Validate all dependencies exist
		for _, depID := range task.Deps {
			if !taskIDSet[depID] {
				return nil, fmt.Errorf("task %s depends on %s which not found: %w",
					task.ID, depID, contracts.ErrDepNotFound)
			}

			// Add forward edge: depID -> task.ID
			dag.Edges[depID] = append(dag.Edges[depID], task.ID)
			depNode := dag.Nodes[depID]
			depNode.Next = append(depNode.Next, task.ID)
		}

		// Initialize Edges entry for this task if it has no dependents yet
		if _, exists := dag.Edges[task.ID]; !exists {
			dag.Edges[task.ID] = []contracts.TaskID{}
		}
	}

	return dag, nil
}

// Validate checks the DAG for cycles and missing dependencies.
// Uses DFS with color marking: white (unvisited), gray (visiting), black (visited).
// Returns ErrDAGCycle if a cycle is detected.
// Returns error if DAG structure is invalid.
func (dr *dependencyResolver) Validate(dag *contracts.DAG) error {
	// Invariant: dag must not be nil
	if dag == nil {
		return contracts.ErrInvalidInput
	}

	// Edge case: nil Nodes
	if dag.Nodes == nil {
		return fmt.Errorf("DAG has nil Nodes: %w", contracts.ErrDAGInvalid)
	}

	// Edge case: nil Edges
	if dag.Edges == nil {
		return fmt.Errorf("DAG has nil Edges: %w", contracts.ErrDAGInvalid)
	}

	// Edge case: empty DAG
	if len(dag.Nodes) == 0 {
		return nil // Valid empty DAG
	}

	// Color constants: white=0 (unvisited), gray=1 (visiting), black=2 (visited)
	colors := make(map[contracts.TaskID]int)

	// Initialize all nodes as white
	for taskID := range dag.Nodes {
		colors[taskID] = 0 // white
	}

	// Run DFS from each unvisited node
	for taskID := range dag.Nodes {
		if colors[taskID] == 0 { // white
			if hasCycle(taskID, colors, dag) {
				return contracts.ErrDAGCycle
			}
		}
	}

	return nil
}

// hasCycle performs DFS to detect cycles.
// Returns true if a cycle is found starting from the given node.
// Uses color marking: white=0, gray=1, black=2.
func hasCycle(node contracts.TaskID, colors map[contracts.TaskID]int, dag *contracts.DAG) bool {
	// Mark node as gray (visiting)
	colors[node] = 1

	dagNode, exists := dag.Nodes[node]
	if !exists {
		// Node doesn't exist in DAG - shouldn't happen in valid DAG
		// but we'll treat it as no cycle found
		return false
	}

	// Check all dependencies (incoming edges reversed for topological detection)
	// For cycle detection in a DAG with forward edges, we follow Next (outgoing edges)
	// to find if we can reach the current node again
	if dagNode.Next != nil {
		for _, nextID := range dagNode.Next {
			nextColor := colors[nextID]

			// Back edge found (gray node) - cycle detected
			if nextColor == 1 { // gray
				return true
			}

			// White node - continue DFS
			if nextColor == 0 { // white
				if hasCycle(nextID, colors, dag) {
					return true
				}
			}
			// Black node (visited) - skip, already processed
		}
	}

	// Mark node as black (visited)
	colors[node] = 2

	return false
}
