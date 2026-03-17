package engine

import (
	"sync"
	"testing"
)

func TestEventBus_PublishSubscribe(t *testing.T) {
	bus := NewEventBus[int]()
	var received []int

	bus.Subscribe(func(v int) {
		received = append(received, v)
	})
	bus.Subscribe(func(v int) {
		received = append(received, v*10)
	})

	bus.Publish(5)
	if len(received) != 2 || received[0] != 5 || received[1] != 50 {
		t.Fatalf("received = %v; want [5 50]", received)
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus[string]()
	var received []string

	unsub := bus.Subscribe(func(s string) {
		received = append(received, s)
	})

	bus.Publish("hello")
	unsub()
	bus.Publish("world")

	if len(received) != 1 || received[0] != "hello" {
		t.Fatalf("received = %v; want [hello]", received)
	}
	if bus.Len() != 0 {
		t.Fatalf("Len after unsubscribe = %d; want 0", bus.Len())
	}
}

func TestEventBus_ConcurrentSubscribe(t *testing.T) {
	bus := NewEventBus[int]()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Subscribe(func(_ int) {})
		}()
	}
	wg.Wait()

	if bus.Len() != 10 {
		t.Fatalf("Len = %d; want 10", bus.Len())
	}
}

func TestEventBus_NoSubscribers(t *testing.T) {
	bus := NewEventBus[int]()
	// Should not panic
	bus.Publish(42)
}

type testEvent struct {
	EntityID int
	Damage   float32
}

func TestEventBus_StructEvents(t *testing.T) {
	bus := NewEventBus[testEvent]()
	var got testEvent

	bus.Subscribe(func(e testEvent) {
		got = e
	})

	bus.Publish(testEvent{EntityID: 7, Damage: 25.5})
	if got.EntityID != 7 || got.Damage != 25.5 {
		t.Fatalf("got = %+v; want {EntityID:7 Damage:25.5}", got)
	}
}
