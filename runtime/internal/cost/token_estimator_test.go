package cost

import (
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestTokenEstimator_Estimate(t *testing.T) {
	estimator := NewTokenEstimator()

	tests := []struct {
		name     string
		input    *contracts.TaskInput
		ctx      *contracts.ContextBundle
		wantMin  contracts.TokenCount
		wantMax  contracts.TokenCount
		wantErr  error
	}{
		{
			name:    "nil input returns error",
			input:   nil,
			ctx:     nil,
			wantErr: contracts.ErrInvalidInput,
		},
		{
			name: "empty input returns zero",
			input: &contracts.TaskInput{
				Prompt: "",
			},
			ctx:     nil,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "short text returns minimum 1 token",
			input: &contracts.TaskInput{
				Prompt: "Hi", // 2 chars < 4 chars/token, but non-empty → 1 token
			},
			ctx:     nil,
			wantMin: 1,
			wantMax: 1,
		},
		{
			name: "single char returns minimum 1 token",
			input: &contracts.TaskInput{
				Prompt: "X", // 1 char < 4 chars/token, but non-empty → 1 token
			},
			ctx:     nil,
			wantMin: 1,
			wantMax: 1,
		},
		{
			name: "prompt only",
			input: &contracts.TaskInput{
				Prompt: "Hello, world!", // 13 chars → 3 tokens
			},
			ctx:     nil,
			wantMin: 3,
			wantMax: 3,
		},
		{
			name: "prompt with inputs",
			input: &contracts.TaskInput{
				Prompt: "Hello", // 5 chars
				Inputs: map[string]string{
					"key1": "value1", // 6 chars
					"key2": "value2", // 6 chars
				},
			},
			ctx:     nil,
			wantMin: 4, // 17 chars / 4 = 4
			wantMax: 4,
		},
		{
			name: "with context bundle",
			input: &contracts.TaskInput{
				Prompt: "test", // 4 chars
			},
			ctx: &contracts.ContextBundle{
				Messages: []string{"msg1", "msg2"}, // 8 chars
				Memory: map[string]string{
					"mem": "data", // 4 chars
				},
				Tools: map[string]string{
					"tool": "definition", // 10 chars
				},
			},
			wantMin: 6, // 26 chars / 4 = 6
			wantMax: 6,
		},
		{
			name: "nil context is handled",
			input: &contracts.TaskInput{
				Prompt: "test prompt here", // 16 chars → 4 tokens
			},
			ctx:     nil,
			wantMin: 4,
			wantMax: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := estimator.Estimate(tt.input, tt.ctx)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Estimate() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Estimate() unexpected error = %v", err)
				return
			}

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("Estimate() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestTokenEstimator_CustomRatio(t *testing.T) {
	// 2 chars per token
	estimator := NewTokenEstimatorWithRatio(2)

	input := &contracts.TaskInput{
		Prompt: "Hello!", // 6 chars → 3 tokens with ratio 2
	}

	got, err := estimator.Estimate(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != 3 {
		t.Errorf("Estimate() = %v, want 3", got)
	}
}

func TestTokenEstimator_InvalidRatio(t *testing.T) {
	// Zero ratio should default to 4
	estimator := NewTokenEstimatorWithRatio(0)

	input := &contracts.TaskInput{
		Prompt: "12345678", // 8 chars → 2 tokens with default ratio 4
	}

	got, err := estimator.Estimate(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != 2 {
		t.Errorf("Estimate() = %v, want 2 (default ratio should be 4)", got)
	}
}
