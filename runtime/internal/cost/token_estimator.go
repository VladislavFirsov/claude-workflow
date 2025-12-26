package cost

import (
	"github.com/anthropics/claude-workflow/runtime/contracts"
)

const defaultCharsPerToken = 4

// tokenEstimator implements contracts.TokenEstimator using character-based heuristic.
type tokenEstimator struct {
	charsPerToken int
}

// NewTokenEstimator creates a new TokenEstimator with default settings.
func NewTokenEstimator() contracts.TokenEstimator {
	return &tokenEstimator{charsPerToken: defaultCharsPerToken}
}

// NewTokenEstimatorWithRatio creates a TokenEstimator with custom chars-per-token ratio.
func NewTokenEstimatorWithRatio(charsPerToken int) contracts.TokenEstimator {
	if charsPerToken <= 0 {
		charsPerToken = defaultCharsPerToken
	}
	return &tokenEstimator{charsPerToken: charsPerToken}
}

// Estimate returns the estimated token count for a task.
func (e *tokenEstimator) Estimate(input *contracts.TaskInput, ctx *contracts.ContextBundle) (contracts.TokenCount, error) {
	if input == nil {
		return 0, contracts.ErrInvalidInput
	}

	var totalChars int

	// Count input prompt
	totalChars += len(input.Prompt)

	// Count input values
	for _, v := range input.Inputs {
		totalChars += len(v)
	}

	// Count metadata values
	for _, v := range input.Metadata {
		totalChars += len(v)
	}

	// Count context if provided
	if ctx != nil {
		// Count messages
		for _, msg := range ctx.Messages {
			totalChars += len(msg)
		}

		// Count memory values
		for _, v := range ctx.Memory {
			totalChars += len(v)
		}

		// Count tool definitions
		for _, v := range ctx.Tools {
			totalChars += len(v)
		}
	}

	tokens := totalChars / e.charsPerToken

	// Minimum 1 token for non-empty input (prevents budget bypass on small requests)
	if totalChars > 0 && tokens == 0 {
		tokens = 1
	}

	return contracts.TokenCount(tokens), nil
}
