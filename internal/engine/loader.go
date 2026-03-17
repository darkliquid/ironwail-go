package engine

import "sync"

// LoadResult holds the outcome of loading a single asset.
// T is the loaded asset type (e.g., *model.Model, *audio.SFX, image.Image).
type LoadResult[T any] struct {
	Key   string // Asset identifier (path, name, etc.)
	Value T      // Loaded asset (zero value on error)
	Err   error  // Non-nil if loading failed
}

// LoadFunc is a function that loads a single asset by key.
type LoadFunc[T any] func(key string) (T, error)

// ParallelLoad loads multiple assets concurrently using up to 'workers'
// goroutines. Results are returned in input order.
func ParallelLoad[T any](keys []string, workers int, load LoadFunc[T]) []LoadResult[T] {
	if workers <= 0 {
		workers = 1
	}
	if len(keys) == 0 {
		return nil
	}

	results := make([]LoadResult[T], len(keys))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for i, key := range keys {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore slot
		go func(idx int, k string) {
			defer wg.Done()
			defer func() { <-sem }() // release slot
			val, err := load(k)
			results[idx] = LoadResult[T]{Key: k, Value: val, Err: err}
		}(i, key)
	}

	wg.Wait()
	return results
}

// LoadPipeline is a channel-based asset loading pipeline. Assets are
// submitted via Send() and results are read from the Results channel.
// The pipeline runs 'workers' goroutines that process requests concurrently.
type LoadPipeline[T any] struct {
	input  chan string
	output chan LoadResult[T]
	done   chan struct{}
	once   sync.Once
}

// NewLoadPipeline creates and starts a channel-based loading pipeline.
func NewLoadPipeline[T any](load LoadFunc[T], workers int) *LoadPipeline[T] {
	if workers <= 0 {
		workers = 1
	}

	p := &LoadPipeline[T]{
		input:  make(chan string, workers*2),
		output: make(chan LoadResult[T], workers*2),
		done:   make(chan struct{}),
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range p.input {
				val, err := load(key)
				p.output <- LoadResult[T]{Key: key, Value: val, Err: err}
			}
		}()
	}

	// Close output channel when all workers finish.
	go func() {
		wg.Wait()
		close(p.output)
		close(p.done)
	}()

	return p
}

// Send submits an asset key for loading. Must not be called after Close().
func (p *LoadPipeline[T]) Send(key string) {
	p.input <- key
}

// Results returns the channel from which loaded assets can be read.
func (p *LoadPipeline[T]) Results() <-chan LoadResult[T] {
	return p.output
}

// Close shuts down the pipeline. No more Send() calls are allowed after this.
// Read remaining results from Results() until the channel closes.
func (p *LoadPipeline[T]) Close() {
	p.once.Do(func() {
		close(p.input)
		<-p.done
	})
}
