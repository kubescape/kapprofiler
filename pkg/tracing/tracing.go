package tracing

import (
	"log"

	containercollection "github.com/inspektor-gadget/inspektor-gadget/pkg/container-collection"
	tracerexec "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/exec/tracer"
	tracertcp "github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/tcp/tracer"
	tracercollection "github.com/inspektor-gadget/inspektor-gadget/pkg/tracer-collection"

	"k8s.io/client-go/rest"
)

type Tracer struct {
	running           bool
	cCollection       *containercollection.ContainerCollection
	tCollection       *tracercollection.TracerCollection
	nodeName          string
	k8sConfig         *rest.Config
	containerSelector containercollection.ContainerSelector

	// IG tracers
	execTracer *tracerexec.Tracer
	tcpTracer  *tracertcp.Tracer

	// Trace event sink object
	eventSink EventSink

	// Container activity listener
	containerActivityListener ContainerActivityEventListener
}

func NewTracer(nodeName string, k8sConfig *rest.Config, eventSink EventSink, contActivityListener ContainerActivityEventListener) *Tracer {
	return &Tracer{running: false, nodeName: nodeName, k8sConfig: k8sConfig, eventSink: eventSink, containerActivityListener: contActivityListener}
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
	t.containerSelector.Labels = map[string]string{"kapprofiler/enabled": "true"}

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
	if t.containerActivityListener != nil {
		if notif.Type == containercollection.EventTypeAddContainer {

			t.containerActivityListener.OnContainerActivityEvent(&ContainerActivityEvent{
				PodName:       notif.Container.Podname,
				Namespace:     notif.Container.Namespace,
				ContainerName: notif.Container.Name,
				Activity:      ContainerActivityEventStart,
			})

		} else if notif.Type == containercollection.EventTypeRemoveContainer {
			t.containerActivityListener.OnContainerActivityEvent(&ContainerActivityEvent{
				PodName:       notif.Container.Podname,
				Namespace:     notif.Container.Namespace,
				ContainerName: notif.Container.Name,
				Activity:      ContainerActivityEventStop,
			})
		}
	}
}
