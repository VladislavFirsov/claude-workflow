// Package main provides a CLI client for the runtime sidecar.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "submit":
		submitCmd(os.Args[2:])
	case "status":
		statusCmd(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  workflow-client submit --file <path> --addr <url>
  workflow-client status --id <run-id> --addr <url>
`)
}

// submitCmd: POST /api/v1/runs
func submitCmd(args []string) {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)
	file := fs.String("file", "", "JSON file path (StartRunRequest)")
	addr := fs.String("addr", "http://localhost:8080", "Sidecar address")
	fs.Parse(args)

	if *file == "" {
		fmt.Fprintln(os.Stderr, "error: --file is required")
		os.Exit(1)
	}

	// Read JSON file
	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// POST request
	resp, err := http.Post(*addr+"/api/v1/runs", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		printAPIError(body, resp.StatusCode)
		os.Exit(1)
	}

	// Parse response
	var run runResponse
	if err := json.Unmarshal(body, &run); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("run_id=%s state=%s\n", run.ID, run.State)
}

// statusCmd: GET /api/v1/runs/{id}
func statusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	id := fs.String("id", "", "Run ID")
	addr := fs.String("addr", "http://localhost:8080", "Sidecar address")
	fs.Parse(args)

	if *id == "" {
		fmt.Fprintln(os.Stderr, "error: --id is required")
		os.Exit(1)
	}

	// GET request
	resp, err := http.Get(*addr + "/api/v1/runs/" + *id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		printAPIError(body, resp.StatusCode)
		os.Exit(1)
	}

	// Parse response
	var run runResponse
	if err := json.Unmarshal(body, &run); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("run_id=%s state=%s\n", run.ID, run.State)

	// Print tasks summary (with error codes for failed tasks)
	if len(run.Tasks) > 0 {
		var parts []string
		// Sort for deterministic output
		taskIDs := make([]string, 0, len(run.Tasks))
		for id := range run.Tasks {
			taskIDs = append(taskIDs, id)
		}
		sort.Strings(taskIDs)

		for _, id := range taskIDs {
			task := run.Tasks[id]
			if task.State == "failed" && task.Error != nil {
				parts = append(parts, fmt.Sprintf("%s=failed(%s)", id, task.Error.Code))
			} else {
				parts = append(parts, fmt.Sprintf("%s=%s", id, task.State))
			}
		}
		fmt.Printf("tasks: %s\n", strings.Join(parts, ", "))
	}

	// Print run-level error if present
	if run.Error != nil {
		fmt.Printf("error: [%s] %s\n", run.Error.Code, run.Error.Message)
	}
}

func printAPIError(body []byte, statusCode int) {
	// API returns flat ErrorDTO: {"code":"...","message":"..."}
	var errResp errorDTO
	if json.Unmarshal(body, &errResp) == nil && errResp.Code != "" {
		fmt.Fprintf(os.Stderr, "error: [%s] %s\n", errResp.Code, errResp.Message)
	} else {
		fmt.Fprintf(os.Stderr, "error: HTTP %d: %s\n", statusCode, string(body))
	}
}

// runResponse mirrors api.RunResponse (minimal fields)
type runResponse struct {
	ID    string                   `json:"id"`
	State string                   `json:"state"`
	Tasks map[string]taskStatusDTO `json:"tasks,omitempty"`
	Error *errorDTO                `json:"error,omitempty"`
}

type taskStatusDTO struct {
	State string    `json:"state"`
	Error *errorDTO `json:"error,omitempty"`
}

type errorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
