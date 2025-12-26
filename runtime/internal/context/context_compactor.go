package context

import (
	"fmt"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

const (
	// StrategyTruncate removes oldest messages until within limit.
	StrategyTruncate = "truncate"
	// StrategyKeepLastN keeps only the last N messages.
	StrategyKeepLastN = "keep_last_n"
	// StrategyNone does no compaction (default).
	StrategyNone = "none"

	// defaultCharsPerToken for token estimation.
	defaultCharsPerToken = 4
)

// contextCompactor implements contracts.ContextCompactor.
// CRITICAL: This component reduces context size. Errors mean information loss.
//
// Strategies:
// - "truncate": Remove oldest messages until tokens <= MaxTokens
// - "keep_last_n": Keep only last N messages (from policy.KeepLastN)
// - "none": No compaction (may error if context too large)
type contextCompactor struct {
	charsPerToken int
}

// NewContextCompactor creates a new ContextCompactor.
func NewContextCompactor() contracts.ContextCompactor {
	return &contextCompactor{
		charsPerToken: defaultCharsPerToken,
	}
}

// NewContextCompactorWithRatio creates a ContextCompactor with custom token ratio.
func NewContextCompactorWithRatio(charsPerToken int) contracts.ContextCompactor {
	if charsPerToken <= 0 {
		charsPerToken = defaultCharsPerToken
	}
	return &contextCompactor{
		charsPerToken: charsPerToken,
	}
}

// Compact reduces the context bundle according to the policy.
// Returns error if:
// - bundle is nil (ErrInvalidInput)
// - policy.MaxTokens is set and context exceeds it after compaction (ErrContextTooLarge)
//
// Note: Memory and Tools are not compacted, only Messages.
func (c *contextCompactor) Compact(bundle *contracts.ContextBundle, policy contracts.ContextPolicy) (*contracts.ContextBundle, error) {
	if bundle == nil {
		return nil, contracts.ErrInvalidInput
	}

	// Create a copy to avoid mutating the original
	result := c.copyBundle(bundle)

	// Apply strategy
	switch policy.Strategy {
	case StrategyKeepLastN:
		result = c.applyKeepLastN(result, policy.KeepLastN)

	case StrategyTruncate:
		result = c.applyTruncate(result, policy.MaxTokens)

	case StrategyNone, "":
		// No compaction

	default:
		// Unknown strategy, treat as none
	}

	// Final size check if MaxTokens is set
	if policy.MaxTokens > 0 {
		tokens := c.estimateTokens(result)
		if tokens > policy.MaxTokens {
			return nil, fmt.Errorf("context has %d tokens after compaction, exceeds limit %d: %w",
				tokens, policy.MaxTokens, contracts.ErrContextTooLarge)
		}
	}

	return result, nil
}

// copyBundle creates a deep copy of the bundle.
func (c *contextCompactor) copyBundle(bundle *contracts.ContextBundle) *contracts.ContextBundle {
	result := &contracts.ContextBundle{
		Messages: make([]string, len(bundle.Messages)),
		Memory:   make(map[string]string),
		Tools:    make(map[string]string),
	}

	copy(result.Messages, bundle.Messages)

	for k, v := range bundle.Memory {
		result.Memory[k] = v
	}
	for k, v := range bundle.Tools {
		result.Tools[k] = v
	}

	return result
}

// applyKeepLastN keeps only the last N messages.
func (c *contextCompactor) applyKeepLastN(bundle *contracts.ContextBundle, n int) *contracts.ContextBundle {
	if n <= 0 || n >= len(bundle.Messages) {
		return bundle
	}

	// Keep last N messages
	startIdx := len(bundle.Messages) - n
	bundle.Messages = bundle.Messages[startIdx:]
	return bundle
}

// applyTruncate removes oldest messages until within token limit.
func (c *contextCompactor) applyTruncate(bundle *contracts.ContextBundle, maxTokens contracts.TokenCount) *contracts.ContextBundle {
	if maxTokens <= 0 {
		return bundle
	}

	for c.estimateTokens(bundle) > maxTokens && len(bundle.Messages) > 0 {
		// Remove oldest message
		bundle.Messages = bundle.Messages[1:]
	}

	return bundle
}

// estimateTokens estimates the token count for a bundle.
func (c *contextCompactor) estimateTokens(bundle *contracts.ContextBundle) contracts.TokenCount {
	var totalChars int

	for _, msg := range bundle.Messages {
		totalChars += len(msg)
	}
	for _, v := range bundle.Memory {
		totalChars += len(v)
	}
	for _, v := range bundle.Tools {
		totalChars += len(v)
	}

	return contracts.TokenCount(totalChars / c.charsPerToken)
}
