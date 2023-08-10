package tracing

import (
	"log"

	tracerseccomp "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/advise/seccomp/tracer"
	tracercapabilities "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/capabilities/tracer"
	tracercapabilitiestype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/capabilities/types"
	tracerexec "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/exec/tracer"
	tracerexectype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/exec/types"
	traceropen "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/open/tracer"
	traceropentype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/open/types"
	tracertcp "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/tcp/tracer"
	tracertcptype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/tcp/types"
	eventtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
)

// Global constants
const execTraceName = "trace_exec"
const openTraceName = "trace_open"
const tcpTraceName = "trace_tcp"
const capabilitiesTraceName = "trace_capabilities"

func (t *Tracer) startAppBehaviorTracing() error {

	// Start tracing execve
	err := t.startExecTracing()
	if err != nil {
		log.Printf("error starting exec tracing: %s\n", err)
		return err
	}

	// Start tracing tcp
	err = t.startTcpTracing()
	if err != nil {
		log.Printf("error starting tcp tracing: %s\n", err)
		return err
	}

	// Start tracing seccomp
	err = t.startSystemcallTracing()
	if err != nil {
		log.Printf("error starting seccomp tracing: %s\n", err)
		return err
	}

	// Start tracing open
	err = t.startOpenTracing()
	if err != nil {
		log.Printf("error starting open tracing: %s\n", err)
		return err
	}

	// Start tracing capabilities
	err = t.startCapabilitiesTracing()
	if err != nil {
		log.Printf("error starting capabilities tracing: %s\n", err)
		return err
	}

	return nil
}

func (t *Tracer) startCapabilitiesTracing() error {
	if err := t.tCollection.AddTracer(capabilitiesTraceName, t.containerSelector); err != nil {
		log.Printf("error adding tracer: %v\n", err)
		return err
	}

	// Get mount namespace map to filter by containers
	capabilitiesMountnsmap, err := t.tCollection.TracerMountNsMap(capabilitiesTraceName)
	if err != nil {
		log.Printf("failed to get capabilitiesMountnsmap: %s\n", err)
		return err
	}

	tracerCapabilities, err := tracercapabilities.NewTracer(&tracercapabilities.Config{MountnsMap: capabilitiesMountnsmap}, t.cCollection, t.capabilitiesEventCallback)
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	t.capabilitiesTracer = tracerCapabilities

	return nil
}

func (t *Tracer) startOpenTracing() error {
	if err := t.tCollection.AddTracer(openTraceName, t.containerSelector); err != nil {
		log.Printf("error adding tracer: %v\n", err)
		return err
	}

	// Get mount namespace map to filter by containers
	openMountnsmap, err := t.tCollection.TracerMountNsMap(openTraceName)
	if err != nil {
		log.Printf("failed to get openMountnsmap: %s\n", err)
		return err
	}

	tracerOpen, err := traceropen.NewTracer(&traceropen.Config{MountnsMap: openMountnsmap, FullPath: true}, t.cCollection, t.openEventCallback)
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	t.openTracer = tracerOpen

	return nil
}

func (t *Tracer) capabilitiesEventCallback(event *tracercapabilitiestype.Event) {
	if event.Type == eventtypes.NORMAL {
		capabilitiesEvent := &CapabilitiesEvent{
			ContainerID:    event.K8s.ContainerName,
			PodName:        event.K8s.PodName,
			Namespace:      event.K8s.Namespace,
			Syscall:        event.Syscall,
			CapabilityName: event.CapName,
			Timestamp:      int64(event.Timestamp),
		}
		t.eventSink.SendCapabilitiesEvent(capabilitiesEvent)
	}
}

func (t *Tracer) openEventCallback(event *traceropentype.Event) {
	if event.Type == eventtypes.NORMAL && event.Ret > -1 {
		openEvent := &OpenEvent{
			ContainerID: event.K8s.ContainerName,
			PodName:     event.K8s.PodName,
			Namespace:   event.K8s.Namespace,
			PathName:    event.FullPath,
			TaskName:    event.Comm,
			TaskId:      int(event.Pid),
			Flags:       event.Flags,
			Timestamp:   int64(event.Timestamp),
		}
		t.eventSink.SendOpenEvent(openEvent)
	}
}

func (t *Tracer) execEventCallback(event *tracerexectype.Event) {
	if event.Type == eventtypes.NORMAL && event.Retval > -1 {
		execveEvent := &ExecveEvent{
			ContainerID: event.K8s.ContainerName,
			PodName:     event.K8s.PodName,
			Namespace:   event.K8s.Namespace,
			PathName:    event.Args[0],
			Args:        event.Args[1:],
			Env:         []string{},
			Timestamp:   int64(event.Timestamp),
		}
		t.eventSink.SendExecveEvent(execveEvent)
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
	tracerExec, err := tracerexec.NewTracer(&tracerexec.Config{MountnsMap: execMountnsmap}, t.cCollection, t.execEventCallback)
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	t.execTracer = tracerExec
	return nil
}

func (t *Tracer) tcpEventCallback(event *tracertcptype.Event) {
	if event.Type == eventtypes.NORMAL {
		var src, dest string
		var srcPort, destPort int

		// If the operation is accept, then the source and destination are reversed (interesting why?)
		if event.Operation == "accept" {
			destPort = int(event.SrcEndpoint.Port)
			dest = event.SrcEndpoint.Addr
			// Force it to be 0 for now to prevent feeding data which is not interesting
			srcPort = 0
			//srcPort = int(event.Dport)
			src = event.DstEndpoint.Addr
		} else if event.Operation == "connect" {
			destPort = int(event.DstEndpoint.Port)
			dest = event.DstEndpoint.Addr
			// Force it to be 0 for now to prevent feeding data which is not interesting
			srcPort = 0
			//srcPort = int(event.Sport)
			src = event.SrcEndpoint.Addr
		} else {
			// Don't care about other operations
			return
		}

		tcpEvent := &TcpEvent{
			ContainerID: event.K8s.ContainerName,
			PodName:     event.K8s.PodName,
			Namespace:   event.K8s.Namespace,
			Source:      src,
			SourcePort:  srcPort,
			Destination: dest,
			DestPort:    destPort,
			Operation:   event.Operation,
			Timestamp:   int64(event.Timestamp),
		}

		t.eventSink.SendTcpEvent(tcpEvent)
	} else {
		// TODO: Handle error
	}
}

func (t *Tracer) startTcpTracing() error {
	// Add tcp tracer
	if err := t.tCollection.AddTracer(tcpTraceName, t.containerSelector); err != nil {
		log.Printf("error adding tcp tracer: %s\n", err)
		return err
	}

	// Get mount namespace map to filter by containers
	tcpMountnsmap, err := t.tCollection.TracerMountNsMap(tcpTraceName)
	if err != nil {
		log.Printf("failed to get tcpMountnsmap: %s\n", err)
		return err
	}

	// Create the tcp tracer
	tracerTcp, err := tracertcp.NewTracer(&tracertcp.Config{MountnsMap: tcpMountnsmap}, t.cCollection, t.tcpEventCallback)
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	t.tcpTracer = tracerTcp
	return nil
}

func (t *Tracer) startSystemcallTracing() error {
	// Add seccomp tracer
	syscallTracer, err := tracerseccomp.NewTracer()
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	t.syscallTracer = syscallTracer
	return nil
}

func (t *Tracer) stopAppBehaviorTracing() error {
	var err error
	err = nil
	// Stop exec tracer
	if err = t.stopExecTracing(); err != nil {
		log.Printf("error stopping exec tracing: %s\n", err)
	}
	// Stop tcp tracer
	if err = t.stopTcpTracing(); err != nil {
		log.Printf("error stopping tcp tracing: %s\n", err)
	}
	// Stop seccomp tracer
	if err = t.stopSystemcallTracing(); err != nil {
		log.Printf("error stopping seccomp tracing: %s\n", err)
	}
	return err
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

func (t *Tracer) stopTcpTracing() error {
	// Stop tcp tracer
	if err := t.tCollection.RemoveTracer(tcpTraceName); err != nil {
		log.Printf("error removing tracer: %s\n", err)
		return err
	}
	t.tcpTracer.Stop()
	return nil
}

func (t *Tracer) stopSystemcallTracing() error {
	// Stop seccomp tracer
	t.syscallTracer.Close()
	return nil
}
