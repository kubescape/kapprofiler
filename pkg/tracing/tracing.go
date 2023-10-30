package tracing

import (
	"fmt"
	"log"

	containercollection "github.com/inspektor-gadget/inspektor-gadget/pkg/container-collection"
	tracerseccomp "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/advise/seccomp/tracer"
	tracercapabilities "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/capabilities/tracer"
	tracerdns "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/tracer"
	tracerexec "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/exec/tracer"
	tracernetwork "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/network/tracer"
	traceropen "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/open/tracer"
	tracercollection "github.com/inspektor-gadget/inspektor-gadget/pkg/tracer-collection"

	"k8s.io/client-go/rest"
)

type ITracer interface {
	Start() error
	Stop() error
	AddContainerActivityListener(listener ContainerActivityEventListener)
	RemoveContainerActivityListener(listener ContainerActivityEventListener)
	PeekSyscallInContainer(nsMountId uint64) ([]string, error)
	AddEventSink(sink EventSink)
	RemoveEventSink(sink EventSink)
}

type Tracer struct {
	// Configuration
	nodeName      string
	filterByLabel bool

	// Internal state
	running           bool
	cCollection       *containercollection.ContainerCollection
	tCollection       *tracercollection.TracerCollection
	k8sConfig         *rest.Config
	containerSelector containercollection.ContainerSelector

	// IG tracers
	execTracer         *tracerexec.Tracer
	syscallTracer      *tracerseccomp.Tracer
	openTracer         *traceropen.Tracer
	dnsTracer          *tracerdns.Tracer
	networkTracer      *tracernetwork.Tracer
	capabilitiesTracer *tracercapabilities.Tracer

	// Trace event sink objects
	eventSinks []EventSink

	// Container activity listener
	containerActivityListener []ContainerActivityEventListener
}

func NewTracer(nodeName string, k8sConfig *rest.Config, eventSinks []EventSink, filterByLabel bool) *Tracer {
	return &Tracer{running: false,
		nodeName:                  nodeName,
		filterByLabel:             filterByLabel,
		k8sConfig:                 k8sConfig,
		eventSinks:                eventSinks,
		containerActivityListener: []ContainerActivityEventListener{}}
}

func (t *Tracer) Start() error {
	if !t.running {
		err := t.setupContainerCollection()
		if err != nil {
			log.Printf("error setting up container collection: %s\n", err)
			return err
		}

		err = t.startAppBehaviorTracing()
		if err != nil {
			t.stopContainerCollection()
			log.Printf("error starting app behavior tracing: %s\n", err)
			return err
		}

		t.running = true
	}
	return nil
}

func (t *Tracer) Stop() error {
	if t.running {
		t.stopContainerCollection()
		t.stopAppBehaviorTracing()
		t.running = false
	}
	return nil
}

func (t *Tracer) AddEventSink(sink EventSink) {
	if t != nil && t.eventSinks != nil {
		t.eventSinks = append(t.eventSinks, sink)
	}
}

func (t *Tracer) RemoveEventSink(sink EventSink) {
	if t != nil && t.eventSinks != nil {
		for i, s := range t.eventSinks {
			if s == sink {
				t.eventSinks = append(t.eventSinks[:i], t.eventSinks[i+1:]...)
				break
			}
		}
	}
}

func (t *Tracer) AddContainerActivityListener(listener ContainerActivityEventListener) {
	if t != nil && t.containerActivityListener != nil {
		t.containerActivityListener = append(t.containerActivityListener, listener)
	}
}

func (t *Tracer) RemoveContainerActivityListener(listener ContainerActivityEventListener) {
	if t != nil && t.containerActivityListener != nil {
		for i, l := range t.containerActivityListener {
			if l == listener {
				t.containerActivityListener = append(t.containerActivityListener[:i], t.containerActivityListener[i+1:]...)
				break
			}
		}
	}
}

func (t *Tracer) PeekSyscallInContainer(nsMountId uint64) ([]string, error) {
	if t == nil || !t.running {
		return nil, fmt.Errorf("tracing not running")
	}
	return t.syscallTracer.Peek(nsMountId)
}

func (t *Tracer) setupContainerCollection() error {
	// Use container collection to get notified for new containers
	containerCollection := &containercollection.ContainerCollection{}

	// Create a tracer collection instance
	tracerCollection, err := tracercollection.NewTracerCollection(containerCollection)
	if err != nil {
		log.Printf("failed to create trace-collection: %s\n", err)
		return err
	}
	t.tCollection = tracerCollection

	// Start the container collection
	containerEventFuncs := []containercollection.FuncNotify{t.containerEventHandler}

	// Define the different options for the container collection instance
	opts := []containercollection.ContainerCollectionOption{
		containercollection.WithTracerCollection(tracerCollection),

		// Get containers created with runc
		containercollection.WithRuncFanotify(),

		// Get containers created with docker
		containercollection.WithCgroupEnrichment(),

		// Enrich events with Linux namespaces information, it is needed for per container filtering
		containercollection.WithLinuxNamespaceEnrichment(),

		// Enrich those containers with data from the Kubernetes API
		containercollection.WithKubernetesEnrichment(t.nodeName, t.k8sConfig),

		// Get Notifications from the container collection
		containercollection.WithPubSub(containerEventFuncs...),
	}

	// Initialize the container collection
	if err := containerCollection.Initialize(opts...); err != nil {
		log.Printf("failed to initialize container collection: %s\n", err)
		return err
	}

	// Define the container selector for later use
	if t.filterByLabel {
		t.containerSelector.K8s.PodLabels = map[string]string{"kapprofiler/enabled": "true"}
	}

	// Store the container collection instance
	t.cCollection = containerCollection

	return nil
}

func (t *Tracer) stopContainerCollection() error {
	if t.cCollection != nil {
		t.tCollection.Close()
		t.cCollection.Close()
	}
	return nil
}

func (t *Tracer) containerEventHandler(notif containercollection.PubSubEvent) {
	if t.containerActivityListener != nil && len(t.containerActivityListener) > 0 {
		activityEvent := &ContainerActivityEvent{
			PodName:       notif.Container.K8s.PodName,
			Namespace:     notif.Container.K8s.Namespace,
			ContainerName: notif.Container.K8s.ContainerName,
			NsMntId:       notif.Container.Mntns,
			ContainerID:   notif.Container.Runtime.ContainerID,
		}
		if notif.Type == containercollection.EventTypeAddContainer {
			activityEvent.Activity = ContainerActivityEventStart
		} else if notif.Type == containercollection.EventTypeRemoveContainer {
			activityEvent.Activity = ContainerActivityEventStop
		}
		// Notify listeners
		for _, listener := range t.containerActivityListener {
			listener.OnContainerActivityEvent(activityEvent)
		}

	}
}
