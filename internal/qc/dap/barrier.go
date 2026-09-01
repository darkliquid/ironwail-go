package dap

import (
	"sync"
)

type stepMode int

const (
	modeContinue stepMode = iota
	modeStepIn
	modeStepOver
	modeStepOut
	modePause
)

// Barrier coordinates synchronization between the execution thread and DAP reader goroutine.
type Barrier struct {
	mu        sync.Mutex
	cond      *sync.Cond
	mode      stepMode
	targetDep int
	paused    bool
}

// NewBarrier creates an initialized Barrier.
func NewBarrier() *Barrier {
	b := &Barrier{
		mode: modeContinue,
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Mode returns the current step mode and target depth thread-safely.
func (b *Barrier) Mode() (stepMode, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mode, b.targetDep
}

// Arm prepares the barrier for waiting by marking it paused.
func (b *Barrier) Arm() {
	b.mu.Lock()
	b.paused = true
	b.mu.Unlock()
}

// Resume unblocks the execution thread.
func (b *Barrier) Resume(mode stepMode, targetDep int) {
	b.mu.Lock()
	b.mode = mode
	b.targetDep = targetDep
	b.paused = false
	b.cond.Broadcast()
	b.mu.Unlock()
}

// Wait blocks until Resume is called.
func (b *Barrier) Wait() {
	b.mu.Lock()
	for b.paused {
		b.cond.Wait()
	}
	b.mu.Unlock()
}

// IsPaused returns true if execution is halted at the barrier.
func (b *Barrier) IsPaused() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.paused
}
