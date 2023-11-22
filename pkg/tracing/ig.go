package tracing

import (
	"log"

	"github.com/cilium/ebpf"
	tracerseccomp "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/advise/seccomp/tracer"
	tracercapabilities "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/capabilities/tracer"
	tracercapabilitiestype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/capabilities/types"
	tracerdns "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/tracer"
	tracerdnstype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/types"
	tracerexec "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/exec/tracer"
	tracerexectype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/exec/types"
	tracernetwork "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/network/tracer"
	tracernetworktype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/network/types"
	traceropen "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/open/tracer"
	traceropentype "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/open/types"
	tracercollection "github.com/inspektor-gadget/inspektor-gadget/pkg/tracer-collection"
	eventtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/utils/host"
)

// Global constants
const execTraceName = "trace_exec"
const openTraceName = "trace_open"
const capabilitiesTraceName = "trace_capabilities"
const dnsTraceName = "trace_dns"
const networkTraceName = "trace_network"

func createEbpfMountNsMap(tracerId string) (*ebpf.Map, error) {
	mntnsSpec := &ebpf.MapSpec{
		Name:       tracercollection.MountMapPrefix + tracerId,
		Type:       ebpf.Hash,
		KeySize:    8,
		ValueSize:  4,
		MaxEntries: tracercollection.MaxContainersPerNode,
	}
	return ebpf.NewMap(mntnsSpec)
}

func (t *Tracer) startAppBehaviorTracing() error {

	// Start tracing execve
	err := t.startExecTracing()
	if err != nil {
		log.Printf("error starting exec tracing: %s\n", err)
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

	// Start tracing dns
	err = t.startDnsTracing()
	if err != nil {
		log.Printf("error starting dns tracing: %s\n", err)
		return err
	}

	// Start tracing network
	err = t.startNetworkTracing()
	if err != nil {
		log.Printf("error starting network tracing: %s\n", err)
		return err
	}

	return nil
}

func (t *Tracer) startNetworkTracing() error {
	//host.Init(host.Config{AutoMountFilesystems: true})

	tracerNetwork, err := tracernetwork.NewTracer()
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	tracerNetwork.SetEventHandler(t.networkEventCallback)

	t.tracingStateMutex.Lock()
	t.tracingState[NetworkEventType] = TracingState{
		usageReferenceCount:    make(map[uint64]int),
		eBpfContainerFilterMap: nil,
		gadget:                 nil,
		attachable:             tracerNetwork,
	}
	t.tracingStateMutex.Unlock()

	return nil
}

func (t *Tracer) startCapabilitiesTracing() error {
	// Create nsmount map to filter by containers
	capabilitiesMountnsmap, err := createEbpfMountNsMap(capabilitiesTraceName)
	if err != nil {
		log.Printf("error creating mountnsmap: %s\n", err)
		return err
	}

	tracerCapabilities, err := tracercapabilities.NewTracer(&tracercapabilities.Config{MountnsMap: capabilitiesMountnsmap}, t.cCollection, t.capabilitiesEventCallback)
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}

	t.tracingStateMutex.Lock()
	t.tracingState[CapabilitiesEventType] = TracingState{
		usageReferenceCount:    make(map[uint64]int),
		eBpfContainerFilterMap: capabilitiesMountnsmap,
		gadget:                 tracerCapabilities,
		attachable:             nil,
	}
	t.tracingStateMutex.Unlock()

	return nil
}

func (t *Tracer) startDnsTracing() error {
	host.Init(host.Config{AutoMountFilesystems: true})

	tracerDns, err := tracerdns.NewTracer()
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	tracerDns.SetEventHandler(t.dnsEventCallback)

	t.tracingStateMutex.Lock()
	t.tracingState[DnsEventType] = TracingState{
		usageReferenceCount:    make(map[uint64]int),
		eBpfContainerFilterMap: nil,
		gadget:                 nil,
		attachable:             tracerDns,
	}
	t.tracingStateMutex.Unlock()

	return nil
}

func (t *Tracer) startOpenTracing() error {
	// Create nsmount map to filter by containers
	openMountnsmap, err := createEbpfMountNsMap(openTraceName)
	if err != nil {
		log.Printf("error creating mountnsmap: %s\n", err)
		return err
	}

	tracerOpen, err := traceropen.NewTracer(&traceropen.Config{MountnsMap: openMountnsmap, FullPath: true}, t.cCollection, t.openEventCallback)
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}

	t.tracingStateMutex.Lock()
	t.tracingState[OpenEventType] = TracingState{
		usageReferenceCount:    make(map[uint64]int),
		eBpfContainerFilterMap: openMountnsmap,
		gadget:                 tracerOpen,
		attachable:             nil,
	}
	t.tracingStateMutex.Unlock()

	return nil
}

func (t *Tracer) dnsEventCallback(event *tracerdnstype.Event) {
	if event.Type == eventtypes.NORMAL {
		t.cCollection.EnrichByMntNs(&event.CommonData, event.MountNsID)
		dnsEvent := &DnsEvent{
			GeneralEvent: GeneralEvent{
				ContainerName: event.K8s.ContainerName,
				ContainerID:   event.Runtime.ContainerID,
				PodName:       event.K8s.PodName,
				Namespace:     event.K8s.Namespace,
				Timestamp:     int64(event.Timestamp),
				EventType:     DnsEventType,
			},
			DnsName:   event.DNSName,
			Addresses: event.Addresses,
		}
		for _, eventSink := range t.eventSinks {
			eventSink.SendDnsEvent(dnsEvent)
		}
	}
}

func (t *Tracer) networkEventCallback(event *tracernetworktype.Event) {
	if event.Type == eventtypes.NORMAL {
		t.cCollection.EnrichByMntNs(&event.CommonData, event.MountNsID)
		networkEvent := &NetworkEvent{
			GeneralEvent: GeneralEvent{
				ContainerName: event.K8s.ContainerName,
				ContainerID:   event.Runtime.ContainerID,
				PodName:       event.K8s.PodName,
				Namespace:     event.K8s.Namespace,
				Timestamp:     int64(event.Timestamp),
				EventType:     NetworkEventType,
			},
			PacketType:  event.PktType,
			Protocol:    event.Proto,
			Port:        event.Port,
			DstEndpoint: event.DstEndpoint.String(),
		}
		for _, eventSink := range t.eventSinks {
			eventSink.SendNetworkEvent(networkEvent)
		}
	}
}

func (t *Tracer) capabilitiesEventCallback(event *tracercapabilitiestype.Event) {
	if event.Type == eventtypes.NORMAL {
		capabilitiesEvent := &CapabilitiesEvent{
			GeneralEvent: GeneralEvent{
				ContainerName: event.K8s.ContainerName,
				ContainerID:   event.Runtime.ContainerID,
				PodName:       event.K8s.PodName,
				Namespace:     event.K8s.Namespace,
				Timestamp:     int64(event.Timestamp),
				EventType:     CapabilitiesEventType,
			},
			Syscall:        event.Syscall,
			CapabilityName: event.CapName,
		}
		for _, eventSink := range t.eventSinks {
			eventSink.SendCapabilitiesEvent(capabilitiesEvent)
		}
	}
}

func (t *Tracer) openEventCallback(event *traceropentype.Event) {
	if event.Type == eventtypes.NORMAL && event.Ret > -1 {
		openEvent := &OpenEvent{
			GeneralEvent: GeneralEvent{
				ContainerName: event.K8s.ContainerName,
				ContainerID:   event.Runtime.ContainerID,
				PodName:       event.K8s.PodName,
				Namespace:     event.K8s.Namespace,
				Timestamp:     int64(event.Timestamp),
				EventType:     OpenEventType,
			},
			PathName: event.FullPath,
			TaskName: event.Comm,
			TaskId:   int(event.Pid),
			Flags:    event.Flags,
		}
		for _, eventSink := range t.eventSinks {
			eventSink.SendOpenEvent(openEvent)
		}
	}
}

func (t *Tracer) execEventCallback(event *tracerexectype.Event) {
	if event.Type == eventtypes.NORMAL && event.Retval > -1 {
		execveEvent := &ExecveEvent{
			GeneralEvent: GeneralEvent{
				ContainerName: event.K8s.ContainerName,
				ContainerID:   event.Runtime.ContainerID,
				PodName:       event.K8s.PodName,
				Namespace:     event.K8s.Namespace,
				Timestamp:     int64(event.Timestamp),
				EventType:     ExecveEventType,
			},
			PathName: event.Args[0],
			Args:     event.Args[1:],
			Env:      []string{},
		}
		for _, eventSink := range t.eventSinks {
			eventSink.SendExecveEvent(execveEvent)
		}
	}
}

func (t *Tracer) startExecTracing() error {
	// Create nsmount map to filter by containers
	execMountnsmap, err := createEbpfMountNsMap(execTraceName)

	// Create the exec tracer
	tracerExec, err := tracerexec.NewTracer(&tracerexec.Config{MountnsMap: execMountnsmap}, t.cCollection, t.execEventCallback)
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}

	t.tracingStateMutex.Lock()
	t.tracingState[ExecveEventType] = TracingState{
		usageReferenceCount:    make(map[uint64]int),
		eBpfContainerFilterMap: execMountnsmap,
		gadget:                 tracerExec,
		attachable:             nil,
	}
	t.tracingStateMutex.Unlock()

	return nil
}

func (t *Tracer) startSystemcallTracing() error {
	// Add seccomp tracer
	syscallTracer, err := tracerseccomp.NewTracer()
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}

	t.tracingStateMutex.Lock()
	t.tracingState[SyscallEventType] = TracingState{
		usageReferenceCount:    nil,
		eBpfContainerFilterMap: nil,
		gadget:                 nil,
		attachable:             nil,
		peekable:               syscallTracer,
	}
	t.tracingStateMutex.Unlock()
	return nil
}

func (t *Tracer) stopAppBehaviorTracing() error {
	var err error
	err = nil
	// Stop exec tracer
	if err = t.stopExecTracing(); err != nil {
		log.Printf("error stopping exec tracing: %s\n", err)
	}
	// Stop seccomp tracer
	if err = t.stopSystemcallTracing(); err != nil {
		log.Printf("error stopping seccomp tracing: %s\n", err)
	}
	// Stop open tracer
	if err = t.stopOpenTracing(); err != nil {
		log.Printf("error stopping open tracing: %s\n", err)
	}
	// Stop capabilities tracer
	if err = t.stopCapabilitiesTracing(); err != nil {
		log.Printf("error stopping capabilities tracing: %s\n", err)
	}
	// Stop dns tracer
	if err = t.stopDnsTracing(); err != nil {
		log.Printf("error stopping dns tracing: %s\n", err)
	}
	// Stop network tracer
	if err = t.stopNetworkTracing(); err != nil {
		log.Printf("error stopping network tracing: %s\n", err)
	}

	return err
}

func (t *Tracer) stopExecTracing() error {
	t.tracingStateMutex.Lock()
	defer t.tracingStateMutex.Unlock()
	if t.tracingState[ExecveEventType].gadget != nil {
		t.tracingState[ExecveEventType].gadget.Stop()
	}
	return nil
}

func (t *Tracer) stopDnsTracing() error {
	t.tracingStateMutex.Lock()
	defer t.tracingStateMutex.Unlock()
	if t.tracingState[DnsEventType].attachable != nil {
		t.tracingState[DnsEventType].attachable.Close()
	}
	return nil
}

func (t *Tracer) stopNetworkTracing() error {
	t.tracingStateMutex.Lock()
	defer t.tracingStateMutex.Unlock()
	if t.tracingState[NetworkEventType].attachable != nil {
		t.tracingState[NetworkEventType].attachable.Close()
	}
	return nil
}

func (t *Tracer) stopOpenTracing() error {
	t.tracingStateMutex.Lock()
	defer t.tracingStateMutex.Unlock()
	if t.tracingState[OpenEventType].gadget != nil {
		t.tracingState[OpenEventType].gadget.Stop()
	}
	return nil
}

func (t *Tracer) stopCapabilitiesTracing() error {
	t.tracingStateMutex.Lock()
	defer t.tracingStateMutex.Unlock()
	if t.tracingState[CapabilitiesEventType].gadget != nil {
		t.tracingState[CapabilitiesEventType].gadget.Stop()
	}
	return nil
}

func (t *Tracer) stopSystemcallTracing() error {
	t.tracingStateMutex.Lock()
	defer t.tracingStateMutex.Unlock()
	if t.tracingState[SyscallEventType].peekable != nil {
		t.tracingState[SyscallEventType].peekable.Close()
	}
	return nil
}
