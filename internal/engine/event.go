package engine

import "sync"

// EventBus is a generic typed publish/subscribe system for decoupled
// communication between engine subsystems. Subscribers register callback
// functions that are invoked synchronously when an event is published.
//
// Why synchronous? In a single-threaded game loop (which Quake is), events
// need to be processed in the same frame they are emitted. Asynchronous
// delivery would introduce frame-boundary timing issues. The synchronous
// model also makes debugging trivial — the call stack shows exactly who
// published and who handled the event.
//
// EventBus is thread-safe for Subscribe (which may happen during init from
// multiple goroutines) but Publish should only be called from the game loop
// thread for deterministic ordering.
//
// Example usage:
//
//	type DamageEvent struct { Target int; Amount float32 }
//	bus := engine.NewEventBus[DamageEvent]()
//	bus.Subscribe(func(e DamageEvent) {
//	    log.Printf("entity %d took %.1f damage", e.Target, e.Amount)
//	})
//	bus.Publish(DamageEvent{Target: 1, Amount: 25.0})
type EventBus[T any] struct {
	mu          sync.RWMutex
	subscribers []func(T)
}

// NewEventBus creates an EventBus with no subscribers.
func NewEventBus[T any]() *EventBus[T] {
	return &EventBus[T]{}
}

// Subscribe registers a callback that will be invoked on every Publish call.
// Subscribers are called in registration order. Returns an unsubscribe
// function that removes this specific callback.
func (b *EventBus[T]) Subscribe(fn func(T)) func() {
	b.mu.Lock()
	idx := len(b.subscribers)
	b.subscribers = append(b.subscribers, fn)
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		// Mark slot as nil; we don't compact to preserve indices of other
		// subscribers. Nil entries are skipped during Publish.
		if idx < len(b.subscribers) {
			b.subscribers[idx] = nil
		}
	}
}

// Publish sends an event to all subscribers synchronously. Subscribers are
// called in registration order. If a subscriber is nil (unsubscribed), it is
// skipped.
func (b *EventBus[T]) Publish(event T) {
	b.mu.RLock()
	subs := b.subscribers
	b.mu.RUnlock()

	for _, fn := range subs {
		if fn != nil {
			fn(event)
		}
	}
}

// Len returns the number of active (non-nil) subscribers.
func (b *EventBus[T]) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := 0
	for _, fn := range b.subscribers {
		if fn != nil {
			n++
		}
	}
	return n
}
