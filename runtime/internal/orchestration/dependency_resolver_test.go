package orchestration

import (
	"errors"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// TestNewDependencyResolver verifies resolver creation.
func TestNewDependencyResolver(t *testing.T) {
	resolver := NewDependencyResolver()
	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}

	// Verify interface compliance
	_, ok := resolver.(contracts.DependencyResolver)
	if !ok {
		t.Fatal("resolver does not implement DependencyResolver interface")
	}
}

// TestBuildDAG_EmptyList tests building DAG from empty task list.
func TestBuildDAG_EmptyList(t *testing.T) {
	resolver := NewDependencyResolver()

	dag, err := resolver.BuildDAG([]contracts.Task{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if dag == nil {
		t.Fatal("expected non-nil DAG")
	}

	if len(dag.Nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(dag.Nodes))
	}

	if len(dag.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(dag.Edges))
	}
}

// TestBuildDAG_NilInput tests building DAG with nil input.
func TestBuildDAG_NilInput(t *testing.T) {
	resolver := NewDependencyResolver()

	dag, err := resolver.BuildDAG(nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}

	if dag != nil {
		t.Fatal("expected nil DAG for nil input")
	}

	if !errors.Is(err, contracts.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// TestBuildDAG_SingleTaskNoDeps tests building DAG with single task and no dependencies.
func TestBuildDAG_SingleTaskNoDeps(t *testing.T) {
	resolver := NewDependencyResolver()

	tasks := []contracts.Task{
		{
			ID:   "task1",
			Deps: []contracts.TaskID{},
		},
	}

	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(dag.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(dag.Nodes))
	}

	node, exists := dag.Nodes["task1"]
	if !exists {
		t.Fatal("expected task1 node in DAG")
	}

	if node.ID != "task1" {
		t.Fatalf("expected node ID task1, got %v", node.ID)
	}

	if len(node.Deps) != 0 {
		t.Fatalf("expected 0 deps, got %d", len(node.Deps))
	}

	if node.Pending != 0 {
		t.Fatalf("expected Pending=0, got %d", node.Pending)
	}

	if len(node.Next) != 0 {
		t.Fatalf("expected 0 Next, got %d", len(node.Next))
	}
}

// TestBuildDAG_LinearDependency tests linear task chain: task1 -> task2 -> task3
func TestBuildDAG_LinearDependency(t *testing.T) {
	resolver := NewDependencyResolver()

	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{"task1"}},
		{ID: "task3", Deps: []contracts.TaskID{"task2"}},
	}

	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check nodes
	if len(dag.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(dag.Nodes))
	}

	// Check task1: no deps, Pending=0, Next=[task2]
	node1 := dag.Nodes["task1"]
	if node1.Pending != 0 {
		t.Fatalf("task1: expected Pending=0, got %d", node1.Pending)
	}
	if len(node1.Next) != 1 || node1.Next[0] != "task2" {
		t.Fatalf("task1: expected Next=[task2], got %v", node1.Next)
	}

	// Check task2: Deps=[task1], Pending=1, Next=[task3]
	node2 := dag.Nodes["task2"]
	if len(node2.Deps) != 1 || node2.Deps[0] != "task1" {
		t.Fatalf("task2: expected Deps=[task1], got %v", node2.Deps)
	}
	if node2.Pending != 1 {
		t.Fatalf("task2: expected Pending=1, got %d", node2.Pending)
	}
	if len(node2.Next) != 1 || node2.Next[0] != "task3" {
		t.Fatalf("task2: expected Next=[task3], got %v", node2.Next)
	}

	// Check task3: Deps=[task2], Pending=1, Next=[]
	node3 := dag.Nodes["task3"]
	if len(node3.Deps) != 1 || node3.Deps[0] != "task2" {
		t.Fatalf("task3: expected Deps=[task2], got %v", node3.Deps)
	}
	if node3.Pending != 1 {
		t.Fatalf("task3: expected Pending=1, got %d", node3.Pending)
	}
	if len(node3.Next) != 0 {
		t.Fatalf("task3: expected Next=[], got %v", node3.Next)
	}

	// Check Edges
	if len(dag.Edges["task1"]) != 1 || dag.Edges["task1"][0] != "task2" {
		t.Fatalf("Edges[task1]: expected [task2], got %v", dag.Edges["task1"])
	}
	if len(dag.Edges["task2"]) != 1 || dag.Edges["task2"][0] != "task3" {
		t.Fatalf("Edges[task2]: expected [task3], got %v", dag.Edges["task2"])
	}
	if len(dag.Edges["task3"]) != 0 {
		t.Fatalf("Edges[task3]: expected [], got %v", dag.Edges["task3"])
	}
}

// TestBuildDAG_MultipleDependencies tests task with multiple dependencies: task3 depends on [task1, task2]
func TestBuildDAG_MultipleDependencies(t *testing.T) {
	resolver := NewDependencyResolver()

	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{}},
		{ID: "task3", Deps: []contracts.TaskID{"task1", "task2"}},
	}

	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check task3: Deps=[task1, task2], Pending=2
	node3 := dag.Nodes["task3"]
	if len(node3.Deps) != 2 {
		t.Fatalf("task3: expected 2 deps, got %d", len(node3.Deps))
	}
	if node3.Pending != 2 {
		t.Fatalf("task3: expected Pending=2, got %d", node3.Pending)
	}

	// Check forward edges
	if len(dag.Edges["task1"]) != 1 || dag.Edges["task1"][0] != "task3" {
		t.Fatalf("Edges[task1]: expected [task3], got %v", dag.Edges["task1"])
	}
	if len(dag.Edges["task2"]) != 1 || dag.Edges["task2"][0] != "task3" {
		t.Fatalf("Edges[task2]: expected [task3], got %v", dag.Edges["task2"])
	}
}

// TestBuildDAG_DAGStructure tests complex DAG with multiple branches
func TestBuildDAG_DAGStructure(t *testing.T) {
	resolver := NewDependencyResolver()

	// Structure:
	//   task1
	//   /    \
	// task2  task3
	//   \    /
	//   task4
	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{"task1"}},
		{ID: "task3", Deps: []contracts.TaskID{"task1"}},
		{ID: "task4", Deps: []contracts.TaskID{"task2", "task3"}},
	}

	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check task1: Next=[task2, task3]
	node1 := dag.Nodes["task1"]
	if len(node1.Next) != 2 {
		t.Fatalf("task1: expected 2 Next, got %d", len(node1.Next))
	}

	// Check task4: Pending=2
	node4 := dag.Nodes["task4"]
	if node4.Pending != 2 {
		t.Fatalf("task4: expected Pending=2, got %d", node4.Pending)
	}
}

// TestBuildDAG_MissingDependency tests error when dependency task is not found.
func TestBuildDAG_MissingDependency(t *testing.T) {
	resolver := NewDependencyResolver()

	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{"task3"}},
		{ID: "task2", Deps: []contracts.TaskID{}},
	}

	dag, err := resolver.BuildDAG(tasks)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}

	if dag != nil {
		t.Fatal("expected nil DAG when error occurs")
	}

	if !errors.Is(err, contracts.ErrDepNotFound) {
		t.Fatalf("expected ErrDepNotFound, got %v", err)
	}
}

// TestBuildDAG_SelfDependency tests self-dependency is allowed in BuildDAG (caught in Validate).
func TestBuildDAG_SelfDependency(t *testing.T) {
	resolver := NewDependencyResolver()

	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{"task1"}},
	}

	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		t.Fatalf("expected no error in BuildDAG, got %v", err)
	}

	// Self-dependency should be in the DAG as it exists in the task list
	if dag == nil {
		t.Fatal("expected non-nil DAG")
	}

	node := dag.Nodes["task1"]
	if node.Pending != 1 {
		t.Fatalf("expected Pending=1 for self-dependency, got %d", node.Pending)
	}

	// But Validate should catch it as a cycle
	err = resolver.Validate(dag)
	if err == nil {
		t.Fatal("expected Validate to detect cycle for self-dependency")
	}

	if !errors.Is(err, contracts.ErrDAGCycle) {
		t.Fatalf("expected ErrDAGCycle, got %v", err)
	}
}

// TestValidate_ValidEmptyDAG tests validation of empty DAG.
func TestValidate_ValidEmptyDAG(t *testing.T) {
	resolver := NewDependencyResolver()

	dag := &contracts.DAG{
		Nodes: make(map[contracts.TaskID]*contracts.DAGNode),
		Edges: make(map[contracts.TaskID][]contracts.TaskID),
	}

	err := resolver.Validate(dag)
	if err != nil {
		t.Fatalf("expected no error for valid empty DAG, got %v", err)
	}
}

// TestValidate_NilInput tests validation with nil DAG.
func TestValidate_NilInput(t *testing.T) {
	resolver := NewDependencyResolver()

	err := resolver.Validate(nil)
	if err == nil {
		t.Fatal("expected error for nil DAG")
	}

	if !errors.Is(err, contracts.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// TestValidate_NilNodes tests validation with nil Nodes.
func TestValidate_NilNodes(t *testing.T) {
	resolver := NewDependencyResolver()

	dag := &contracts.DAG{
		Nodes: nil,
		Edges: make(map[contracts.TaskID][]contracts.TaskID),
	}

	err := resolver.Validate(dag)
	if err == nil {
		t.Fatal("expected error for nil Nodes")
	}

	if !errors.Is(err, contracts.ErrDAGInvalid) {
		t.Fatalf("expected ErrDAGInvalid, got %v", err)
	}
}

// TestValidate_NilEdges tests validation with nil Edges.
func TestValidate_NilEdges(t *testing.T) {
	resolver := NewDependencyResolver()

	dag := &contracts.DAG{
		Nodes: make(map[contracts.TaskID]*contracts.DAGNode),
		Edges: nil,
	}

	err := resolver.Validate(dag)
	if err == nil {
		t.Fatal("expected error for nil Edges")
	}

	if !errors.Is(err, contracts.ErrDAGInvalid) {
		t.Fatalf("expected ErrDAGInvalid, got %v", err)
	}
}

// TestValidate_ValidDAG tests validation of valid DAG.
func TestValidate_ValidDAG(t *testing.T) {
	resolver := NewDependencyResolver()

	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{"task1"}},
		{ID: "task3", Deps: []contracts.TaskID{"task2"}},
	}

	dag, _ := resolver.BuildDAG(tasks)

	err := resolver.Validate(dag)
	if err != nil {
		t.Fatalf("expected no error for valid DAG, got %v", err)
	}
}

// TestValidate_SimpleCycle detects simple cycle: task1 -> task2 -> task1
func TestValidate_SimpleCycle(t *testing.T) {
	resolver := NewDependencyResolver()

	// Manually construct a DAG with a cycle
	dag := &contracts.DAG{
		Nodes: map[contracts.TaskID]*contracts.DAGNode{
			"task1": {
				ID:      "task1",
				Deps:    []contracts.TaskID{"task2"},
				Next:    []contracts.TaskID{"task2"},
				Pending: 1,
			},
			"task2": {
				ID:      "task2",
				Deps:    []contracts.TaskID{"task1"},
				Next:    []contracts.TaskID{"task1"},
				Pending: 1,
			},
		},
		Edges: map[contracts.TaskID][]contracts.TaskID{
			"task1": {"task2"},
			"task2": {"task1"},
		},
	}

	err := resolver.Validate(dag)
	if err == nil {
		t.Fatal("expected error for cycle")
	}

	if !errors.Is(err, contracts.ErrDAGCycle) {
		t.Fatalf("expected ErrDAGCycle, got %v", err)
	}
}

// TestValidate_SelfCycle detects self-cycle: task1 -> task1
func TestValidate_SelfCycle(t *testing.T) {
	resolver := NewDependencyResolver()

	dag := &contracts.DAG{
		Nodes: map[contracts.TaskID]*contracts.DAGNode{
			"task1": {
				ID:      "task1",
				Deps:    []contracts.TaskID{"task1"},
				Next:    []contracts.TaskID{"task1"},
				Pending: 1,
			},
		},
		Edges: map[contracts.TaskID][]contracts.TaskID{
			"task1": {"task1"},
		},
	}

	err := resolver.Validate(dag)
	if err == nil {
		t.Fatal("expected error for self-cycle")
	}

	if !errors.Is(err, contracts.ErrDAGCycle) {
		t.Fatalf("expected ErrDAGCycle, got %v", err)
	}
}

// TestValidate_LongerCycle detects longer cycle: task1 -> task2 -> task3 -> task1
func TestValidate_LongerCycle(t *testing.T) {
	resolver := NewDependencyResolver()

	dag := &contracts.DAG{
		Nodes: map[contracts.TaskID]*contracts.DAGNode{
			"task1": {
				ID:   "task1",
				Next: []contracts.TaskID{"task2"},
			},
			"task2": {
				ID:   "task2",
				Next: []contracts.TaskID{"task3"},
			},
			"task3": {
				ID:   "task3",
				Next: []contracts.TaskID{"task1"},
			},
		},
		Edges: map[contracts.TaskID][]contracts.TaskID{
			"task1": {"task2"},
			"task2": {"task3"},
			"task3": {"task1"},
		},
	}

	err := resolver.Validate(dag)
	if err == nil {
		t.Fatal("expected error for cycle")
	}

	if !errors.Is(err, contracts.ErrDAGCycle) {
		t.Fatalf("expected ErrDAGCycle, got %v", err)
	}
}

// TestValidate_ComplexDAGNoCycle validates complex DAG without cycles
func TestValidate_ComplexDAGNoCycle(t *testing.T) {
	resolver := NewDependencyResolver()

	// Structure:
	//   task1    task2
	//   /   \    /
	// task3  task4
	//   \    /
	//   task5
	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{}},
		{ID: "task3", Deps: []contracts.TaskID{"task1"}},
		{ID: "task4", Deps: []contracts.TaskID{"task1", "task2"}},
		{ID: "task5", Deps: []contracts.TaskID{"task3", "task4"}},
	}

	dag, _ := resolver.BuildDAG(tasks)

	err := resolver.Validate(dag)
	if err != nil {
		t.Fatalf("expected no error for valid DAG, got %v", err)
	}
}

// TestBuildAndValidate_Integration tests BuildDAG followed by Validate
func TestBuildAndValidate_Integration(t *testing.T) {
	resolver := NewDependencyResolver()

	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{"task1"}},
		{ID: "task3", Deps: []contracts.TaskID{"task1"}},
		{ID: "task4", Deps: []contracts.TaskID{"task2", "task3"}},
	}

	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	err = resolver.Validate(dag)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Verify DAG structure
	if len(dag.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(dag.Nodes))
	}

	// Verify Pending counts
	expectedPending := map[contracts.TaskID]int{
		"task1": 0,
		"task2": 1,
		"task3": 1,
		"task4": 2,
	}

	for taskID, expectedP := range expectedPending {
		actualP := dag.Nodes[taskID].Pending
		if actualP != expectedP {
			t.Fatalf("task %s: expected Pending=%d, got %d", taskID, expectedP, actualP)
		}
	}
}

// TestBuildDAG_DependencyOrder tests that Deps order is preserved
func TestBuildDAG_DependencyOrder(t *testing.T) {
	resolver := NewDependencyResolver()

	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{}},
		{ID: "task3", Deps: []contracts.TaskID{}},
		{ID: "task4", Deps: []contracts.TaskID{"task3", "task1", "task2"}},
	}

	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	node4 := dag.Nodes["task4"]
	expectedDeps := []contracts.TaskID{"task3", "task1", "task2"}

	if len(node4.Deps) != len(expectedDeps) {
		t.Fatalf("task4: expected %d deps, got %d", len(expectedDeps), len(node4.Deps))
	}

	for i, expectedDep := range expectedDeps {
		if node4.Deps[i] != expectedDep {
			t.Fatalf("task4 Deps[%d]: expected %s, got %s", i, expectedDep, node4.Deps[i])
		}
	}
}

// TestValidate_DAGWithNilNextSlices validates DAG when Next slices are nil
func TestValidate_DAGWithNilNextSlices(t *testing.T) {
	resolver := NewDependencyResolver()

	// Create a valid DAG and manually set Next to nil
	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{"task1"}},
	}

	dag, _ := resolver.BuildDAG(tasks)

	// Set Next to nil for task1 (edge case)
	dag.Nodes["task1"].Next = nil

	// Validate should still work - nil Next is handled
	err := resolver.Validate(dag)
	if err != nil {
		t.Fatalf("expected no error for nil Next slices, got %v", err)
	}
}

// TestValidate_DAGWithMultipleSources validates DAG with multiple root nodes
func TestValidate_DAGWithMultipleSources(t *testing.T) {
	resolver := NewDependencyResolver()

	// Structure: task1 and task2 are independent roots
	//   task1  task2
	//    |      |
	//   task3  task4
	//     \    /
	//     task5
	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{}},
		{ID: "task3", Deps: []contracts.TaskID{"task1"}},
		{ID: "task4", Deps: []contracts.TaskID{"task2"}},
		{ID: "task5", Deps: []contracts.TaskID{"task3", "task4"}},
	}

	dag, _ := resolver.BuildDAG(tasks)

	err := resolver.Validate(dag)
	if err != nil {
		t.Fatalf("expected no error for multiple sources, got %v", err)
	}
}

// TestBuildDAG_LargeLiddag tests a larger DAG structure
func TestBuildDAG_LargeDAG(t *testing.T) {
	resolver := NewDependencyResolver()

	// Create a larger DAG with 10 tasks
	tasks := make([]contracts.Task, 10)
	for i := 0; i < 10; i++ {
		tasks[i].ID = contracts.TaskID("task" + string(rune('0'+i)))
		if i > 0 {
			// Each task depends on previous task
			tasks[i].Deps = []contracts.TaskID{tasks[i-1].ID}
		}
	}

	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(dag.Nodes) != 10 {
		t.Fatalf("expected 10 nodes, got %d", len(dag.Nodes))
	}

	// Verify linear structure
	for i := 0; i < 10; i++ {
		taskID := contracts.TaskID("task" + string(rune('0'+i)))
		node := dag.Nodes[taskID]

		expectedPending := 0
		if i > 0 {
			expectedPending = 1
		}

		if node.Pending != expectedPending {
			t.Fatalf("task%d: expected Pending=%d, got %d", i, expectedPending, node.Pending)
		}
	}

	// Validate the large DAG
	err = resolver.Validate(dag)
	if err != nil {
		t.Fatalf("expected no error validating large DAG, got %v", err)
	}
}

// TestValidate_CycleWithMultipleRoots detects cycle in DAG with multiple roots
func TestValidate_CycleWithMultipleRoots(t *testing.T) {
	resolver := NewDependencyResolver()

	// Structure has a cycle: task3 -> task4 -> task3
	// And independent root task1, task2
	dag := &contracts.DAG{
		Nodes: map[contracts.TaskID]*contracts.DAGNode{
			"task1": {ID: "task1", Next: []contracts.TaskID{}},
			"task2": {ID: "task2", Next: []contracts.TaskID{}},
			"task3": {ID: "task3", Next: []contracts.TaskID{"task4"}},
			"task4": {ID: "task4", Next: []contracts.TaskID{"task3"}},
		},
		Edges: map[contracts.TaskID][]contracts.TaskID{
			"task1": {},
			"task2": {},
			"task3": {"task4"},
			"task4": {"task3"},
		},
	}

	err := resolver.Validate(dag)
	if err == nil {
		t.Fatal("expected error for cycle")
	}

	if !errors.Is(err, contracts.ErrDAGCycle) {
		t.Fatalf("expected ErrDAGCycle, got %v", err)
	}
}

// TestBuildDAG_MultipleIndependentTasks tests multiple independent tasks
func TestBuildDAG_MultipleIndependentTasks(t *testing.T) {
	resolver := NewDependencyResolver()

	tasks := []contracts.Task{
		{ID: "task1", Deps: []contracts.TaskID{}},
		{ID: "task2", Deps: []contracts.TaskID{}},
		{ID: "task3", Deps: []contracts.TaskID{}},
	}

	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// All tasks should have Pending=0
	for _, task := range tasks {
		node := dag.Nodes[task.ID]
		if node.Pending != 0 {
			t.Fatalf("task %s: expected Pending=0, got %d", task.ID, node.Pending)
		}

		if len(node.Next) != 0 {
			t.Fatalf("task %s: expected no Next, got %v", task.ID, node.Next)
		}
	}
}
