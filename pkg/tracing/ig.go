package tracing

import (
	"log"

	tracerexec "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/exec/tracer"
	tracerexectype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/exec/types"
	eventtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
)

// Global constants
const execTraceName = "trace_exec"

//const openTraceName = "trace_open"
//const tcpTraceName = "trace_tcp"

// Global variables
var traceEventSink EventSink

func (t *Tracer) startAppBehaviorTracing() error {
	// Record sink object
	traceEventSink = t.eventSink

	// Start tracing execve
	err := t.startExecTracing()
	if err != nil {
		log.Printf("error starting exec tracing: %s\n", err)
		return err
	}
	return nil
}

func execEventCallback(event *tracerexectype.Event) {
	if event.Type == eventtypes.NORMAL && event.Retval > -1 {
		if traceEventSink != nil {
			execveEvent := &ExecveEvent{
				ContainerID: event.Container,
				PodName:     event.Pod,
				Namespace:   event.Namespace,
				PathName:    event.Args[0],
				Args:        event.Args[1:],
				Env:         []string{},
			}
			traceEventSink.SendExecveEvent(execveEvent)
		}
	}
}

func (t *Tracer) startExecTracing() error {
	// Add exec tracer
	if err := t.tCollection.AddTracer(execTraceName, t.containerSelector); err != nil {
		log.Printf("error adding tracer: %s\n", err)
		return err
	}

	// Get mount namespace map to filter by containers
	execMountnsmap, err := t.tCollection.TracerMountNsMap(execTraceName)
	if err != nil {
		log.Printf("failed to get execMountnsmap: %s\n", err)
		return err
	}

	// Create the exec tracer
	tracerExec, err := tracerexec.NewTracer(&tracerexec.Config{MountnsMap: execMountnsmap}, t.cCollection, execEventCallback)
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	t.execTracer = tracerExec
	return nil
}

func (t *Tracer) stopAppBehaviorTracing() error {
	// Stop exec tracer
	if err := t.stopExecTracing(); err != nil {
		log.Printf("error stopping exec tracing: %s\n", err)
		return err
	}
	return nil
}

func (t *Tracer) stopExecTracing() error {
	// Stop exec tracer
	if err := t.tCollection.RemoveTracer(execTraceName); err != nil {
		log.Printf("error removing tracer: %s\n", err)
		return err
	}
	t.execTracer.Stop()
	return nil
}
