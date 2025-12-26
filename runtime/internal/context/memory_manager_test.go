package context

import (
	"sync"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestMemoryManagerGet(t *testing.T) {
	tests := []struct {
		name          string
		run           *contracts.Run
		key           string
		setupMemory   map[string]string
		expectedValue string
		expectedOk    bool
	}{
		{
			name:          "Get existing key",
			run:           &contracts.Run{Memory: map[string]string{"key1": "value1"}},
			key:           "key1",
			expectedValue: "value1",
			expectedOk:    true,
		},
		{
			name:          "Get non-existing key",
			run:           &contracts.Run{Memory: map[string]string{"key1": "value1"}},
			key:           "nonexistent",
			expectedValue: "",
			expectedOk:    false,
		},
		{
			name:          "Get from nil run",
			run:           nil,
			key:           "key1",
			expectedValue: "",
			expectedOk:    false,
		},
		{
			name:          "Get from run with nil Memory",
			run:           &contracts.Run{Memory: nil},
			key:           "key1",
			expectedValue: "",
			expectedOk:    false,
		},
		{
			name:          "Get empty key",
			run:           &contracts.Run{Memory: map[string]string{"": "empty_key_value"}},
			key:           "",
			expectedValue: "empty_key_value",
			expectedOk:    true,
		},
		{
			name:          "Get from empty Memory map",
			run:           &contracts.Run{Memory: map[string]string{}},
			key:           "any_key",
			expectedValue: "",
			expectedOk:    false,
		},
	}

	mm := NewMemoryManager()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := mm.Get(tt.run, tt.key)

			if ok != tt.expectedOk {
				t.Errorf("expected ok=%v, got ok=%v", tt.expectedOk, ok)
			}

			if value != tt.expectedValue {
				t.Errorf("expected value=%q, got value=%q", tt.expectedValue, value)
			}
		})
	}
}

func TestMemoryManagerPut(t *testing.T) {
	tests := []struct {
		name           string
		initialRun     *contracts.Run
		key            string
		value          string
		expectNilCheck bool
		shouldStore    bool
	}{
		{
			name:           "Put into existing Memory",
			initialRun:     &contracts.Run{Memory: map[string]string{"key1": "value1"}},
			key:            "key2",
			value:          "value2",
			expectNilCheck: false,
			shouldStore:    true,
		},
		{
			name:           "Put overwrite existing key",
			initialRun:     &contracts.Run{Memory: map[string]string{"key1": "value1"}},
			key:            "key1",
			value:          "newvalue",
			expectNilCheck: false,
			shouldStore:    true,
		},
		{
			name:           "Put into nil Memory",
			initialRun:     &contracts.Run{Memory: nil},
			key:            "key1",
			value:          "value1",
			expectNilCheck: false,
			shouldStore:    true,
		},
		{
			name:           "Put into nil run",
			initialRun:     nil,
			key:            "key1",
			value:          "value1",
			expectNilCheck: true,
			shouldStore:    false,
		},
		{
			name:           "Put empty key",
			initialRun:     &contracts.Run{Memory: map[string]string{}},
			key:            "",
			value:          "empty_key_value",
			expectNilCheck: false,
			shouldStore:    true,
		},
		{
			name:           "Put empty value",
			initialRun:     &contracts.Run{Memory: map[string]string{}},
			key:            "empty_value_key",
			value:          "",
			expectNilCheck: false,
			shouldStore:    true,
		},
	}

	mm := NewMemoryManager()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm.Put(tt.initialRun, tt.key, tt.value)

			if tt.expectNilCheck {
				// Should not panic and run should still be nil
				if tt.initialRun != nil {
					t.Error("expected run to remain nil")
				}
				return
			}

			if !tt.shouldStore {
				return
			}

			// Verify the value was stored
			if tt.initialRun.Memory == nil {
				t.Error("expected Memory to be initialized")
				return
			}

			storedValue, ok := tt.initialRun.Memory[tt.key]
			if !ok {
				t.Error("expected key to be stored in Memory")
			}

			if storedValue != tt.value {
				t.Errorf("expected stored value=%q, got=%q", tt.value, storedValue)
			}
		})
	}
}

func TestMemoryManagerPutInitializesMemory(t *testing.T) {
	mm := NewMemoryManager()
	run := &contracts.Run{Memory: nil}

	mm.Put(run, "testkey", "testvalue")

	if run.Memory == nil {
		t.Fatal("expected Memory to be initialized")
	}

	value, ok := run.Memory["testkey"]
	if !ok {
		t.Fatal("expected key to exist in Memory")
	}

	if value != "testvalue" {
		t.Errorf("expected value=%q, got=%q", "testvalue", value)
	}
}

func TestMemoryManagerIntegration(t *testing.T) {
	mm := NewMemoryManager()
	run := &contracts.Run{}

	// Put without initialization
	mm.Put(run, "key1", "value1")
	mm.Put(run, "key2", "value2")
	mm.Put(run, "key3", "value3")

	// Get all values
	tests := []struct {
		key      string
		expected string
	}{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	for _, tt := range tests {
		value, ok := mm.Get(run, tt.key)
		if !ok {
			t.Errorf("expected key %q to exist", tt.key)
		}
		if value != tt.expected {
			t.Errorf("key %q: expected %q, got %q", tt.key, tt.expected, value)
		}
	}

	// Overwrite a value
	mm.Put(run, "key1", "newvalue1")
	value, ok := mm.Get(run, "key1")
	if !ok {
		t.Error("expected key1 to exist after overwrite")
	}
	if value != "newvalue1" {
		t.Errorf("expected newvalue1, got %q", value)
	}
}

func TestMemoryManagerConcurrentAccess(t *testing.T) {
	mm := NewMemoryManager()
	run := &contracts.Run{Memory: map[string]string{}}

	const (
		numGoroutines = 100
		operations    = 50
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := "key_" + string(rune(id)) + "_" + string(rune(j))
				value := "value_" + string(rune(id)) + "_" + string(rune(j))
				mm.Put(run, key, value)
			}
		}(i)
	}

	wg.Wait()

	// Verify all values
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := "key_" + string(rune(id)) + "_" + string(rune(j))
				expectedValue := "value_" + string(rune(id)) + "_" + string(rune(j))
				value, ok := mm.Get(run, key)
				if !ok {
					t.Errorf("expected key %q to exist", key)
				}
				if value != expectedValue {
					t.Errorf("key %q: expected %q, got %q", key, expectedValue, value)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestMemoryManagerNilRunNoPanic(t *testing.T) {
	mm := NewMemoryManager()

	// These should not panic
	mm.Put(nil, "key", "value")
	value, ok := mm.Get(nil, "key")

	if ok {
		t.Error("expected ok=false for nil run")
	}

	if value != "" {
		t.Errorf("expected empty string, got %q", value)
	}
}

func BenchmarkMemoryManagerGet(b *testing.B) {
	mm := NewMemoryManager()
	run := &contracts.Run{Memory: map[string]string{"key": "value"}}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mm.Get(run, "key")
	}
}

func BenchmarkMemoryManagerPut(b *testing.B) {
	mm := NewMemoryManager()
	run := &contracts.Run{Memory: map[string]string{}}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mm.Put(run, "key", "value")
	}
}

func BenchmarkMemoryManagerGetParallel(b *testing.B) {
	mm := NewMemoryManager()
	run := &contracts.Run{Memory: map[string]string{"key": "value"}}

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mm.Get(run, "key")
		}
	})
}

func BenchmarkMemoryManagerPutParallel(b *testing.B) {
	mm := NewMemoryManager()
	run := &contracts.Run{Memory: map[string]string{}}

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mm.Put(run, "key", "value")
		}
	})
}
