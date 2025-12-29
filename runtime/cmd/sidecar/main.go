// Package main provides the entry point for the runtime sidecar binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anthropics/claude-workflow/runtime/api"
	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func main() {
	// Parse flags
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	log.Printf("Starting runtime sidecar on %s", *addr)

	// Create executor (mock for now)
	executor := mockExecutor

	// Create and start server
	server := api.NewServer(*addr, executor)

	// Handle graceful shutdown
	done := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
		close(done)
	}()

	// Start server
	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	<-done
	log.Println("Server stopped")
}

// mockExecutor is a placeholder executor for testing.
// In production, this would call an LLM API.
func mockExecutor(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
	// Simulate some processing time
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	return &contracts.TaskResult{
		Output: fmt.Sprintf("mock result for task %s", task.ID),
		Usage: contracts.Usage{
			Tokens: 100,
			Cost:   contracts.Cost{Amount: 0.001, Currency: "USD"},
		},
	}, nil
}
