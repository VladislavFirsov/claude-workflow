package context

import (
	"errors"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestContextCompactor_Compact(t *testing.T) {
	compactor := NewContextCompactor()

	tests := []struct {
		name         string
		bundle       *contracts.ContextBundle
		policy       contracts.ContextPolicy
		wantErr      error
		wantMsgCount int
	}{
		{
			name:    "nil bundle returns error",
			bundle:  nil,
			wantErr: contracts.ErrInvalidInput,
		},
		{
			name: "empty bundle with no policy",
			bundle: &contracts.ContextBundle{
				Messages: []string{},
			},
			policy:       contracts.ContextPolicy{},
			wantMsgCount: 0,
		},
		{
			name: "no compaction without strategy",
			bundle: &contracts.ContextBundle{
				Messages: []string{"msg1", "msg2", "msg3"},
			},
			policy:       contracts.ContextPolicy{Strategy: ""},
			wantMsgCount: 3,
		},
		{
			name: "keep_last_n keeps only last N",
			bundle: &contracts.ContextBundle{
				Messages: []string{"msg1", "msg2", "msg3", "msg4", "msg5"},
			},
			policy:       contracts.ContextPolicy{Strategy: StrategyKeepLastN, KeepLastN: 2},
			wantMsgCount: 2,
		},
		{
			name: "keep_last_n with N > len keeps all",
			bundle: &contracts.ContextBundle{
				Messages: []string{"msg1", "msg2"},
			},
			policy:       contracts.ContextPolicy{Strategy: StrategyKeepLastN, KeepLastN: 10},
			wantMsgCount: 2,
		},
		{
			name: "keep_last_n with N <= 0 keeps all",
			bundle: &contracts.ContextBundle{
				Messages: []string{"msg1", "msg2", "msg3"},
			},
			policy:       contracts.ContextPolicy{Strategy: StrategyKeepLastN, KeepLastN: 0},
			wantMsgCount: 3,
		},
		{
			name: "truncate removes oldest until within limit",
			bundle: &contracts.ContextBundle{
				Messages: []string{
					"1234567890123456", // 16 chars = 4 tokens
					"1234567890123456", // 16 chars = 4 tokens
					"1234567890123456", // 16 chars = 4 tokens
				},
			},
			policy:       contracts.ContextPolicy{Strategy: StrategyTruncate, MaxTokens: 8},
			wantMsgCount: 2, // remove first to get to 8 tokens
		},
		{
			name: "truncate with no limit keeps all",
			bundle: &contracts.ContextBundle{
				Messages: []string{"msg1", "msg2", "msg3"},
			},
			policy:       contracts.ContextPolicy{Strategy: StrategyTruncate, MaxTokens: 0},
			wantMsgCount: 3,
		},
		{
			name: "unknown strategy treated as none",
			bundle: &contracts.ContextBundle{
				Messages: []string{"msg1", "msg2"},
			},
			policy:       contracts.ContextPolicy{Strategy: "unknown"},
			wantMsgCount: 2,
		},
		{
			name: "exceeds limit after compaction returns error",
			bundle: &contracts.ContextBundle{
				Messages: []string{"12345678901234567890"}, // 20 chars = 5 tokens
			},
			policy:  contracts.ContextPolicy{Strategy: StrategyNone, MaxTokens: 2},
			wantErr: contracts.ErrContextTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := compactor.Compact(tt.bundle, tt.policy)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Compact() expected error, got nil")
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Compact() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Compact() unexpected error = %v", err)
			}

			if len(result.Messages) != tt.wantMsgCount {
				t.Errorf("Compact() message count = %d, want %d", len(result.Messages), tt.wantMsgCount)
			}
		})
	}
}

func TestContextCompactor_PreservesLastMessages(t *testing.T) {
	compactor := NewContextCompactor()

	bundle := &contracts.ContextBundle{
		Messages: []string{"oldest", "middle", "newest"},
	}

	result, err := compactor.Compact(bundle, contracts.ContextPolicy{
		Strategy:  StrategyKeepLastN,
		KeepLastN: 2,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}

	if result.Messages[0] != "middle" {
		t.Errorf("first message = %q, want %q", result.Messages[0], "middle")
	}
	if result.Messages[1] != "newest" {
		t.Errorf("second message = %q, want %q", result.Messages[1], "newest")
	}
}

func TestContextCompactor_DoesNotMutateOriginal(t *testing.T) {
	compactor := NewContextCompactor()

	original := &contracts.ContextBundle{
		Messages: []string{"msg1", "msg2", "msg3", "msg4"},
		Memory:   map[string]string{"key": "value"},
		Tools:    map[string]string{"tool": "def"},
	}

	_, err := compactor.Compact(original, contracts.ContextPolicy{
		Strategy:  StrategyKeepLastN,
		KeepLastN: 2,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original should be unchanged
	if len(original.Messages) != 4 {
		t.Errorf("original.Messages was mutated: len = %d, want 4", len(original.Messages))
	}
}

func TestContextCompactor_CopiesMemoryAndTools(t *testing.T) {
	compactor := NewContextCompactor()

	bundle := &contracts.ContextBundle{
		Messages: []string{"msg1"},
		Memory:   map[string]string{"key1": "value1", "key2": "value2"},
		Tools:    map[string]string{"tool1": "def1"},
	}

	result, err := compactor.Compact(bundle, contracts.ContextPolicy{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Memory should be copied
	if len(result.Memory) != 2 {
		t.Errorf("memory count = %d, want 2", len(result.Memory))
	}
	if result.Memory["key1"] != "value1" {
		t.Errorf("memory[key1] = %q, want %q", result.Memory["key1"], "value1")
	}

	// Tools should be copied
	if len(result.Tools) != 1 {
		t.Errorf("tools count = %d, want 1", len(result.Tools))
	}

	// Modify result shouldn't affect original
	result.Memory["new"] = "value"
	if _, exists := bundle.Memory["new"]; exists {
		t.Error("modifying result affected original")
	}
}

func TestContextCompactor_TruncateRemovesOldest(t *testing.T) {
	compactor := NewContextCompactor()

	bundle := &contracts.ContextBundle{
		Messages: []string{
			"oldest message here",   // ~5 tokens
			"middle message here",   // ~5 tokens
			"newest message here",   // ~5 tokens
		},
	}

	result, err := compactor.Compact(bundle, contracts.ContextPolicy{
		Strategy:  StrategyTruncate,
		MaxTokens: 10, // Should keep 2 messages
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}

	// Should have kept the newer ones
	if result.Messages[0] != "middle message here" {
		t.Errorf("first message = %q, expected middle", result.Messages[0])
	}
	if result.Messages[1] != "newest message here" {
		t.Errorf("second message = %q, expected newest", result.Messages[1])
	}
}

func TestContextCompactor_TruncateWithMemoryAndTools(t *testing.T) {
	compactor := NewContextCompactor()

	bundle := &contracts.ContextBundle{
		Messages: []string{"12345678", "12345678", "12345678"}, // 24 chars = 6 tokens
		Memory:   map[string]string{"key": "12345678"},         // 8 chars = 2 tokens
		Tools:    map[string]string{"tool": "12345678"},        // 8 chars = 2 tokens
	}
	// Total: 10 tokens (messages) + 2 (memory) + 2 (tools) = 14... wait
	// Actually memory/tools counted: 8 + 8 = 16 chars = 4 tokens
	// Messages: 24 chars = 6 tokens
	// Total: ~10 tokens

	result, err := compactor.Compact(bundle, contracts.ContextPolicy{
		Strategy:  StrategyTruncate,
		MaxTokens: 8,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Memory and tools should be preserved
	if len(result.Memory) != 1 {
		t.Errorf("memory was affected, len = %d", len(result.Memory))
	}
	if len(result.Tools) != 1 {
		t.Errorf("tools was affected, len = %d", len(result.Tools))
	}
}

func TestContextCompactor_CustomRatio(t *testing.T) {
	// 2 chars per token
	compactor := NewContextCompactorWithRatio(2)

	bundle := &contracts.ContextBundle{
		Messages: []string{"12345678"}, // 8 chars = 4 tokens with ratio 2
	}

	_, err := compactor.Compact(bundle, contracts.ContextPolicy{
		Strategy:  StrategyNone,
		MaxTokens: 3, // 4 tokens > 3, should fail
	})

	if !errors.Is(err, contracts.ErrContextTooLarge) {
		t.Errorf("expected ErrContextTooLarge, got %v", err)
	}
}

func TestContextCompactor_ZeroRatioDefaults(t *testing.T) {
	compactor := NewContextCompactorWithRatio(0)

	bundle := &contracts.ContextBundle{
		Messages: []string{"test"},
	}

	result, err := compactor.Compact(bundle, contracts.ContextPolicy{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Error("unexpected result")
	}
}

func TestContextCompactor_EmptyNilMaps(t *testing.T) {
	compactor := NewContextCompactor()

	bundle := &contracts.ContextBundle{
		Messages: []string{"msg"},
		Memory:   nil,
		Tools:    nil,
	}

	result, err := compactor.Compact(bundle, contracts.ContextPolicy{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should create empty maps, not nil
	if result.Memory == nil {
		t.Error("result.Memory is nil, expected empty map")
	}
	if result.Tools == nil {
		t.Error("result.Tools is nil, expected empty map")
	}
}
