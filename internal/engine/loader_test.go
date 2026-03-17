package engine

import (
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelLoad_Basic(t *testing.T) {
	keys := []string{"a", "b", "c", "d"}
	results := ParallelLoad(keys, 2, func(key string) (string, error) {
		return "loaded:" + key, nil
	})

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result %d: unexpected error: %v", i, r.Err)
		}

		expected := "loaded:" + keys[i]
		if r.Value != expected {
			t.Errorf("result %d: expected %q, got %q", i, expected, r.Value)
		}
	}
}

func TestParallelLoad_WithErrors(t *testing.T) {
	keys := []string{"ok", "fail", "ok2"}
	results := ParallelLoad(keys, 2, func(key string) (string, error) {
		if key == "fail" {
			return "", fmt.Errorf("load error: %s", key)
		}
		return key, nil
	})

	if results[1].Err == nil {
		t.Error("expected error for 'fail' key")
	}
}

func TestParallelLoad_Empty(t *testing.T) {
	results := ParallelLoad([]string{}, 2, func(string) (int, error) {
		return 0, nil
	})
	if results != nil {
		t.Errorf("expected nil for empty input, got %v", results)
	}
}

func TestParallelLoad_Concurrency(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	keys := make([]string, 20)
	for i := range keys {
		keys[i] = fmt.Sprintf("key%d", i)
	}

	ParallelLoad(keys, 4, func(key string) (string, error) {
		c := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if c <= old || maxConcurrent.CompareAndSwap(old, c) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		concurrent.Add(-1)
		return key, nil
	})

	if maxConcurrent.Load() < 2 {
		t.Errorf("expected concurrent execution, max concurrent was %d", maxConcurrent.Load())
	}
}

func TestLoadPipeline_Basic(t *testing.T) {
	pipe := NewLoadPipeline(func(key string) (string, error) {
		return "loaded:" + key, nil
	}, 2)

	pipe.Send("x")
	pipe.Send("y")
	pipe.Close()

	var results []string
	for r := range pipe.Results() {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		results = append(results, r.Value)
	}

	sort.Strings(results)
	if len(results) != 2 || results[0] != "loaded:x" || results[1] != "loaded:y" {
		t.Errorf("unexpected results: %v", results)
	}
}
