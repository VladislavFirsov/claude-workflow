// Package contracts defines the core types and interfaces for the Runtime Layer.
package contracts

// RunID uniquely identifies a run.
type RunID string

// TaskID uniquely identifies a task within a run.
type TaskID string

// ModelID identifies an LLM model (e.g., "gpt-4", "claude-3-opus").
type ModelID string

// TokenCount represents a count of tokens.
type TokenCount int64

// Currency represents a currency code (e.g., "USD").
type Currency string

// Timestamp represents a Unix timestamp in milliseconds.
type Timestamp int64
