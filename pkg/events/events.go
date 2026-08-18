// Package events is a small in-process event bus.
//
// It exists to keep side effects out of the code that causes them. A service
// that saves a post should not also have to remember to touch the cache, warm
// a search index and send a notification — it announces what happened, and
// whatever cares subscribes. Adding a new reaction then means adding a
// listener, not editing the service again.
//
// Delivery is synchronous and in-process. That is deliberate: the first
// listener here keeps a cache consistent with the database, and a read issued
// immediately after a write must not race an invalidation that has not run
// yet. A listener that genuinely wants to be asynchronous can start its own
// goroutine — that choice belongs to the listener, not the bus.
package events

import (
	"context"
	"log"
	"sync"
)

// Event is anything that can be dispatched. The name is what listeners
// subscribe to, so it must be stable.
type Event interface {
	Name() string
}

// Listener reacts to an event. Returning an error does not fail the operation
// that emitted the event — see Dispatch.
type Listener func(ctx context.Context, event Event) error

// Dispatcher routes events to the listeners registered for them.
//
// Safe for concurrent use: listeners are registered at boot and read on every
// dispatch, which is exactly the read-mostly pattern RWMutex is for.
type Dispatcher struct {
	mu        sync.RWMutex
	listeners map[string][]namedListener
}

type namedListener struct {
	// label identifies the listener in logs. Without it a failure reports only
	// that "a listener failed", which is not something you can act on.
	label string
	fn    Listener
}

func New() *Dispatcher {
	return &Dispatcher{listeners: map[string][]namedListener{}}
}

// Listen registers fn for an event name. Listeners run in registration order.
func (d *Dispatcher) Listen(eventName, label string, fn Listener) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.listeners[eventName] = append(d.listeners[eventName], namedListener{label: label, fn: fn})
}

// Dispatch delivers an event to every listener registered for it.
//
// Failures are logged, not returned, and one failing listener does not stop
// the rest. The reasoning is the same one that governs cache invalidation
// generally: the thing that emitted this event has already happened and is
// durable, so reporting an error here would invite a caller to retry work that
// succeeded. What a failure costs is a stale side effect, which is why it is
// logged loudly enough to notice.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event) {
	d.mu.RLock()
	listeners := d.listeners[event.Name()]
	d.mu.RUnlock()

	for _, l := range listeners {
		if err := l.fn(ctx, event); err != nil {
			log.Printf("events: listener %q failed handling %s: %v", l.label, event.Name(), err)
		}
	}
}

// ListenerCount reports how many listeners are registered for an event.
// Exposed for tests and for a boot-time summary.
func (d *Dispatcher) ListenerCount(eventName string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return len(d.listeners[eventName])
}
