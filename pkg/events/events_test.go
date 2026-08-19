package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/scriptertoufiq/gobook/pkg/events"
)

type thingHappened struct{ Value string }

func (thingHappened) Name() string { return "thing.happened" }

type otherThing struct{}

func (otherThing) Name() string { return "other.thing" }

func TestListenersRunInRegistrationOrder(t *testing.T) {
	d := events.New()
	var order []string

	d.Listen("thing.happened", "first", func(context.Context, events.Event) error {
		order = append(order, "first")
		return nil
	})
	d.Listen("thing.happened", "second", func(context.Context, events.Event) error {
		order = append(order, "second")
		return nil
	})

	d.Dispatch(context.Background(), thingHappened{})

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("expected [first second], got %v", order)
	}
}

func TestEventPayloadReachesTheListener(t *testing.T) {
	d := events.New()
	var got string

	d.Listen("thing.happened", "capture", func(_ context.Context, e events.Event) error {
		if happened, ok := e.(thingHappened); ok {
			got = happened.Value
		}
		return nil
	})

	d.Dispatch(context.Background(), thingHappened{Value: "carried"})

	if got != "carried" {
		t.Errorf("listener saw %q, want %q", got, "carried")
	}
}

func TestListenersOnlyReceiveTheirOwnEvent(t *testing.T) {
	d := events.New()
	calls := 0

	d.Listen("thing.happened", "counter", func(context.Context, events.Event) error {
		calls++
		return nil
	})

	d.Dispatch(context.Background(), otherThing{})

	if calls != 0 {
		t.Errorf("listener fired for an unrelated event, %d times", calls)
	}
}

// A failing listener must not stop the ones behind it — otherwise registration
// order silently becomes a dependency.
func TestOneFailingListenerDoesNotStopTheRest(t *testing.T) {
	d := events.New()
	reached := false

	d.Listen("thing.happened", "explodes", func(context.Context, events.Event) error {
		return errors.New("boom")
	})
	d.Listen("thing.happened", "still-runs", func(context.Context, events.Event) error {
		reached = true
		return nil
	})

	d.Dispatch(context.Background(), thingHappened{})

	if !reached {
		t.Error("the second listener never ran")
	}
}

func TestDispatchingWithNoListenersIsHarmless(t *testing.T) {
	events.New().Dispatch(context.Background(), thingHappened{})
}

func TestListenerCountReportsRegistrations(t *testing.T) {
	d := events.New()
	noop := func(context.Context, events.Event) error { return nil }

	d.Listen("thing.happened", "a", noop)
	d.Listen("thing.happened", "b", noop)

	if got := d.ListenerCount("thing.happened"); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
	if got := d.ListenerCount("never.registered"); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}
