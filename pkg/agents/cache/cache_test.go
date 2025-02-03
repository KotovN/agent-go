package cache

import (
	"testing"
)

func TestNewCacheHandler(t *testing.T) {
	handler := NewCacheHandler()
	if handler == nil {
		t.Error("Cache handler should not be nil")
	}
}

func TestCacheOperations(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		input   string
		output  string
		wantHit bool
	}{
		{
			name:    "Basic cache operation",
			tool:    "TestTool",
			input:   "test input",
			output:  "test output",
			wantHit: true,
		},
		{
			name:    "Empty input cache operation",
			tool:    "TestTool",
			input:   "",
			output:  "empty input result",
			wantHit: true,
		},
		{
			name:    "Special characters in input",
			tool:    "TestTool",
			input:   "test!@#$%^&*()",
			output:  "special chars result",
			wantHit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCacheHandler()

			// Test Add operation
			handler.Add(tt.tool, tt.input, tt.output)

			// Test Read operation
			result, exists := handler.Read(tt.tool, tt.input)
			if exists != tt.wantHit {
				t.Errorf("Cache hit status mismatch: got %v, want %v", exists, tt.wantHit)
			}
			if exists && result != tt.output {
				t.Errorf("Cached output mismatch: got %v, want %v", result, tt.output)
			}

			// Test non-existent key
			_, exists = handler.Read(tt.tool+"nonexistent", tt.input)
			if exists {
				t.Error("Should not find non-existent key")
			}
		})
	}
}

func TestCacheClear(t *testing.T) {
	handler := NewCacheHandler()

	// Add some items to cache
	testCases := []struct {
		tool   string
		input  string
		output string
	}{
		{"Tool1", "input1", "output1"},
		{"Tool2", "input2", "output2"},
		{"Tool3", "input3", "output3"},
	}

	for _, tc := range testCases {
		handler.Add(tc.tool, tc.input, tc.output)
	}

	// Verify items are in cache
	for _, tc := range testCases {
		result, exists := handler.Read(tc.tool, tc.input)
		if !exists {
			t.Errorf("Item should exist in cache: %s", tc.tool)
		}
		if result != tc.output {
			t.Errorf("Cached output should match: got %v, want %v", result, tc.output)
		}
	}

	// Clear cache
	handler.Clear()

	// Verify items are removed
	for _, tc := range testCases {
		_, exists := handler.Read(tc.tool, tc.input)
		if exists {
			t.Errorf("Item should not exist after clear: %s", tc.tool)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	handler := NewCacheHandler()
	done := make(chan bool)
	iterations := 1000

	// Concurrent writers
	go func() {
		for i := 0; i < iterations; i++ {
			handler.Add("Tool1", "input1", "output1")
		}
		done <- true
	}()

	// Concurrent readers
	go func() {
		for i := 0; i < iterations; i++ {
			handler.Read("Tool1", "input1")
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify final state
	result, exists := handler.Read("Tool1", "input1")
	if !exists {
		t.Error("Final cache entry should exist")
	}
	if result != "output1" {
		t.Errorf("Final cache entry should have correct value: got %v, want %v", result, "output1")
	}
}

func TestCacheKeyGeneration(t *testing.T) {
	handler := NewCacheHandler()

	// Test that different tool names with same input are treated as different entries
	handler.Add("Tool1", "same_input", "output1")
	handler.Add("Tool2", "same_input", "output2")

	result1, exists1 := handler.Read("Tool1", "same_input")
	result2, exists2 := handler.Read("Tool2", "same_input")

	if !exists1 {
		t.Error("First cache entry should exist")
	}
	if !exists2 {
		t.Error("Second cache entry should exist")
	}
	if result1 == result2 {
		t.Error("Different tools with same input should have different outputs")
	}
}
