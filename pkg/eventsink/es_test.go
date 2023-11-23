package eventsink_test

import (
	"testing"
	"time"

	"github.com/kubescape/kapprofiler/pkg/eventsink"
	"github.com/kubescape/kapprofiler/pkg/tracing"
)

func TestNewEventSink(t *testing.T) {
	// Setup
	// Depending on the implementation, you may have to setup some state here
	es, err := eventsink.NewEventSink("", false)
	if err != nil {
		t.Errorf("error creating event sink: %s\n", err)
	}

	err = es.Start()
	if err != nil {
		t.Errorf("error starting event sink: %s\n", err)
	}
	defer es.Stop()

	// Exercise
	es.SendExecveEvent(&tracing.ExecveEvent{
		GeneralEvent: tracing.GeneralEvent{
			ContainerID:   "test",
			ContainerName: "test",
			PodName:       "test",
			Namespace:     "test",
			Pid:           1,
			Timestamp:     0,
		},
		PathName: "test",
		Args:     []string{"test"},
		Env:      []string{"test"},
	})

	// Verify

	// Sleep for a 1 second to allow the event to be processed
	time.Sleep(1 * time.Second)

	// Get events
	events, err := es.GetExecveEvents("test", "test", "test")
	if err != nil {
		t.Errorf("error getting execve events: %s\n", err)
	}

	// Verify that the event was stored
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d\n", len(events))
		return
	}
	if events[0].ContainerID != "test" {
		t.Errorf("expected container ID test, got %s\n", events[0].ContainerID)
	}
	if events[0].PodName != "test" {
		t.Errorf("expected pod name test, got %s\n", events[0].PodName)
	}
	if events[0].Namespace != "test" {
		t.Errorf("expected namespace test, got %s\n", events[0].Namespace)
	}
	if events[0].PathName != "test" {
		t.Errorf("expected path name test, got %s\n", events[0].PathName)
	}
	if len(events[0].Args) != 1 {
		t.Errorf("expected 1 argument, got %d\n", len(events[0].Args))
	}
	if events[0].Args[0] != "test" {
		t.Errorf("expected argument test, got %s\n", events[0].Args[0])
	}
	if len(events[0].Env) != 1 {
		t.Errorf("expected 1 environment variable, got %d\n", len(events[0].Env))
	}
	if events[0].Env[0] != "test" {
		t.Errorf("expected environment variable test, got %s\n", events[0].Env[0])
	}
	if events[0].Timestamp != 0 {
		t.Errorf("expected timestamp 0, got %d\n", events[0].Timestamp)
	}

	// Teardown

	// Delete bucket
	err = es.CleanupContainer("test", "test", "test")
	if err != nil {
		t.Errorf("error cleaning up container: %s\n", err)
	}
}

func TestEventSinkWithFilter(t *testing.T) {
	// Setup
	// Depending on the implementation, you may have to setup some state here
	es, err := eventsink.NewEventSink("", true)
	if err != nil {
		t.Errorf("error creating event sink: %s\n", err)
	}

	err = es.Start()
	if err != nil {
		t.Errorf("error starting event sink: %s\n", err)
	}
	defer es.Stop()

	// Exercise
	es.SendExecveEvent(&tracing.ExecveEvent{
		GeneralEvent: tracing.GeneralEvent{
			ContainerID:   "test",
			ContainerName: "test",
			PodName:       "test",
			Namespace:     "test",
			Pid:           1,
			Timestamp:     0,
		},
		PathName: "test",
		Args:     []string{"test"},
		Env:      []string{"test"},
	})

	// Verify

	// Sleep for a 1 second to allow the event to be
	// processed
	time.Sleep(1 * time.Second)

	// Get events
	events, err := es.GetExecveEvents("test", "test", "test")
	if err != nil {
		t.Errorf("error getting execve events: %s\n", err)
	}

	// Since we did not apply matching filter, we should not get any events
	if len(events) != 0 {
		t.Errorf("expected 0 event, got %d\n", len(events))
	}

	// Add filter
	es.AddFilter(&eventsink.EventSinkFilter{
		ContainerID: "test",
		EventType:   tracing.ExecveEventType,
	})

	// Send another event
	es.SendExecveEvent(&tracing.ExecveEvent{
		GeneralEvent: tracing.GeneralEvent{
			ContainerID:   "test",
			ContainerName: "test",
			PodName:       "test",
			Namespace:     "test",
			Pid:           1,
			Timestamp:     0,
		},
		PathName: "test",
		Args:     []string{"test"},
		Env:      []string{"test"},
	})

	// Verify

	// Sleep for a 1 second to allow the event to be
	// processed
	time.Sleep(1 * time.Second)

	// Get events
	events, err = es.GetExecveEvents("test", "test", "test")
	if err != nil {
		t.Errorf("error getting execve events: %s\n", err)
	}

	// Since we did not apply matching filter, we should not get any events
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d\n", len(events))
	}
}
