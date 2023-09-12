package tracing

import (
	"log"

	"github.com/inspektor-gadget/inspektor-gadget/pkg/container-collection/networktracer"
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
	eventtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/utils/host"
)

// Global constants
const execTraceName = "trace_exec"
const openTraceName = "trace_open"
const capabilitiesTraceName = "trace_capabilities"
const dnsTraceName = "trace_dns"
const networkTraceName = "trace_network"

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

	if err := t.tCollection.AddTracer(networkTraceName, t.containerSelector); err != nil {
		log.Printf("error adding tracer: %v\n", err)
		return err
	}

	tracerNetwork, err := tracernetwork.NewTracer()
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	tracerNetwork.SetEventHandler(t.networkEventCallback)

	t.networkTracer = tracerNetwork

	config := &networktracer.ConnectToContainerCollectionConfig[tracernetworktype.Event]{
		Tracer:   t.networkTracer,
		Resolver: t.cCollection,
		Selector: t.containerSelector,
		Base:     tracernetworktype.Base,
	}

	_, err = networktracer.ConnectToContainerCollection(config)

	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
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

func (t *Tracer) startDnsTracing() error {
	host.Init(host.Config{AutoMountFilesystems: true})

	if err := t.tCollection.AddTracer(dnsTraceName, t.containerSelector); err != nil {
		log.Printf("error adding tracer: %v\n", err)
		return err
	}

	tracerDns, err := tracerdns.NewTracer()
	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}
	tracerDns.SetEventHandler(t.dnsEventCallback)

	t.dnsTracer = tracerDns

	config := &networktracer.ConnectToContainerCollectionConfig[tracerdnstype.Event]{
		Tracer:   t.dnsTracer,
		Resolver: t.cCollection,
		Selector: t.containerSelector,
		Base:     tracerdnstype.Base,
	}

	_, err = networktracer.ConnectToContainerCollection(config)

	if err != nil {
		log.Printf("error creating tracer: %s\n", err)
		return err
	}

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

func (t *Tracer) dnsEventCallback(event *tracerdnstype.Event) {
	if event.Type == eventtypes.NORMAL {
		t.cCollection.EnrichByMntNs(&event.CommonData, event.MountNsID)
		dnsEvent := &DnsEvent{
			ContainerID: event.K8s.ContainerName,
			PodName:     event.K8s.PodName,
			Namespace:   event.K8s.Namespace,
			DnsName:     event.DNSName,
			Addresses:   event.Addresses,
			Timestamp:   int64(event.Timestamp),
		}
		t.eventSink.SendDnsEvent(dnsEvent)
	}
}

func (t *Tracer) networkEventCallback(event *tracernetworktype.Event) {
	if event.Type == eventtypes.NORMAL {
		t.cCollection.EnrichByMntNs(&event.CommonData, event.MountNsID)
		networkEvent := &NetworkEvent{
			ContainerID: event.K8s.ContainerName,
			PodName:     event.K8s.PodName,
			Namespace:   event.K8s.Namespace,
			PacketType:  event.PktType,
			Protocol:    event.Proto,
			Port:        event.Port,
			DstEndpoint: event.DstEndpoint.String(),
			Timestamp:   int64(event.Timestamp),
		}
		t.eventSink.SendNetworkEvent(networkEvent)
	}
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
	// Stop exec tracer
	if err := t.tCollection.RemoveTracer(execTraceName); err != nil {
		log.Printf("error removing tracer: %s\n", err)
		return err
	}
	t.execTracer.Stop()
	return nil
}

func (t *Tracer) stopDnsTracing() error {
	// Stop dns tracer
	if err := t.tCollection.RemoveTracer(dnsTraceName); err != nil {
		log.Printf("error removing tracer: %s\n", err)
		return err
	}

	t.dnsTracer.Close()

	return nil
}

func (t *Tracer) stopNetworkTracing() error {
	// Stop network tracer
	if err := t.tCollection.RemoveTracer(networkTraceName); err != nil {
		log.Printf("error removing tracer: %s\n", err)
		return err
	}

	t.networkTracer.Close()

	return nil
}

func (t *Tracer) stopOpenTracing() error {
	// Stop open tracer
	if err := t.tCollection.RemoveTracer(openTraceName); err != nil {
		log.Printf("error removing tracer: %s\n", err)
		return err
	}
	t.openTracer.Stop()
	return nil
}

func (t *Tracer) stopCapabilitiesTracing() error {
	// Stop capabilities tracer
	if err := t.tCollection.RemoveTracer(capabilitiesTraceName); err != nil {
		log.Printf("error removing tracer: %s\n", err)
		return err
	}
	t.capabilitiesTracer.Stop()
	return nil
}

func (t *Tracer) stopSystemcallTracing() error {
	// Stop seccomp tracer
	t.syscallTracer.Close()
	return nil
}
