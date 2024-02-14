package tracing

import (
	"fmt"
	"log"
	"os"
	"sync"

	containercollection "github.com/inspektor-gadget/inspektor-gadget/pkg/container-collection"
	igtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"

	"github.com/inspektor-gadget/inspektor-gadget/pkg/container-utils/types"

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
	StartTraceContainer(mntns uint64, pid uint32, eventType EventType) error
	StopTraceContainer(mntns uint64, pid uint32, eventType EventType) error
}

type Tracer struct {
	// Configuration
	nodeName      string
	filterByLabel bool

	// Internal state
	running     bool
	cCollection *containercollection.ContainerCollection
	k8sConfig   *rest.Config

	// Tracing state
	tracingState      map[EventType]TracingState
	tracingStateMutex sync.Mutex

	// Trace event sink objects
	eventSinks []EventSink

	// Container activity listener
	containerActivityListener []ContainerActivityEventListener
}

func NewTracer(nodeName string, k8sConfig *rest.Config, eventSinks []EventSink, filterByLabel bool) *Tracer {
	// Create the tracer state
	tracingState := make(map[EventType]TracingState)

	return &Tracer{running: false,
		nodeName:                  nodeName,
		filterByLabel:             filterByLabel,
		k8sConfig:                 k8sConfig,
		eventSinks:                eventSinks,
		tracingState:              tracingState,
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

		// Generate a list of running containers
		t.GenerateContainerEventsOnStart()
	}
	return nil
}

func (t *Tracer) GenerateContainerEventsOnStart() {
	// Generate a list of running containers
	containers, err := t.GetListOfRunningContainers()
	if err != nil {
		log.Printf("error getting list of running containers: %s\n", err)
	} else {
		for _, container := range *containers {
			container.Activity = ContainerActivityEventAttached
			for _, containerActivityListener := range t.containerActivityListener {
				containerActivityListener.OnContainerActivityEvent(&container)
			}
		}
	}
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
	if t.tracingState[SyscallEventType].peekable == nil {
		return nil, fmt.Errorf("tracing initialized but not started")
	}
	return t.tracingState[SyscallEventType].peekable.Peek(nsMountId)
}

func (t *Tracer) setupContainerCollection() error {
	// Use container collection to get notified for new containers
	containerCollection := &containercollection.ContainerCollection{}

	// Start the container collection
	containerEventFuncs := []containercollection.FuncNotify{t.containerEventHandler}

	// Define the different options for the container collection instance
	opts := []containercollection.ContainerCollectionOption{
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

	containerRuntimeName := os.Getenv("CONTAINER_RUNTIME_NAME")
	containerRuntimeSocket := os.Getenv("CONTAINER_RUNTIME_SOCKET")
	if containerRuntimeName != "" && containerRuntimeSocket != "" {
		// Add the container runtime to the container collection
		opts = append(opts, containercollection.WithContainerRuntimeEnrichment(&types.RuntimeConfig{
			Name:       igtypes.RuntimeName(containerRuntimeName),
			SocketPath: containerRuntimeSocket,
		}))
	} else if containerRuntimeName != "" || containerRuntimeSocket != "" {
		return fmt.Errorf("both CONTAINER_RUNTIME_NAME and CONTAINER_RUNTIME_SOCKET environment variables must be set")
	} else {
		// Read the HOST_ROOT environment variable
		hostRootMount := os.Getenv("HOST_ROOT")

		// Detect the container runtime
		runtimeConfig, err := DetectContainerRuntime(hostRootMount)
		if err != nil {
			log.Printf("failed to detect container runtime: %s\n", err)
		} else {
			// Add the container runtime to the container collection
			opts = append(opts, containercollection.WithContainerRuntimeEnrichment(runtimeConfig))
		}
	}

	// Initialize the container collection
	if err := containerCollection.Initialize(opts...); err != nil {
		log.Printf("failed to initialize container collection: %s\n", err)
		return err
	}

	// Store the container collection instance
	t.cCollection = containerCollection

	return nil
}

func (t *Tracer) stopContainerCollection() error {
	if t.cCollection != nil {
		t.cCollection.Close()
	}
	return nil
}

func (t *Tracer) containerEventHandler(notif containercollection.PubSubEvent) {
	if !t.running {
		// Ignore events if tracing is not running
		// These events are replac
		return
	}
	if t.containerActivityListener != nil && len(t.containerActivityListener) > 0 {
		activityEvent := &ContainerActivityEvent{
			PodName:       notif.Container.K8s.PodName,
			Namespace:     notif.Container.K8s.Namespace,
			ContainerName: notif.Container.K8s.ContainerName,
			NsMntId:       notif.Container.Mntns,
			ContainerID:   notif.Container.Runtime.ContainerID,
			Pid:           notif.Container.Pid,
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

func (t *Tracer) StartTraceContainer(mntns uint64, pid uint32, eventType EventType) error {
	if t == nil || !t.running {
		return fmt.Errorf("tracing not running (running %v)", t.running)
	}
	var eventTypesToStart []EventType
	if eventType == AllEventType {
		eventTypesToStart = []EventType{NetworkEventType, DnsEventType, ExecveEventType, CapabilitiesEventType, OpenEventType, RandomXEventType}
	} else {
		eventTypesToStart = append(eventTypesToStart, eventType)
	}
	for _, startEventType := range eventTypesToStart {
		if t.tracingState[startEventType].gadget != nil {
			// Tracing gadget with nsmap control
			t.tracingStateMutex.Lock()
			one := uint32(1)
			mntnsC := uint64(mntns)
			t.tracingState[startEventType].eBpfContainerFilterMap.Put(&mntnsC, &one)
			t.tracingState[startEventType].usageReferenceCount[mntns]++
			t.tracingStateMutex.Unlock()
		} else if t.tracingState[startEventType].attachable != nil {
			// Tracing gadget with peekable interface
			t.tracingStateMutex.Lock()
			t.tracingState[startEventType].attachable.Attach(pid)
			t.tracingStateMutex.Unlock()
		} else {
			return fmt.Errorf("not a tracable event type")
		}
	}
	return nil
}

func (t *Tracer) StopTraceContainer(mntns uint64, pid uint32, eventType EventType) error {
	if t == nil || !t.running {
		return fmt.Errorf("tracing not running")
	}
	t.tracingStateMutex.Lock()
	defer t.tracingStateMutex.Unlock()
	var eventTypesToStop []EventType
	if eventType == AllEventType {
		eventTypesToStop = []EventType{NetworkEventType, DnsEventType, ExecveEventType, CapabilitiesEventType, OpenEventType, RandomXEventType}
	} else {
		eventTypesToStop = append(eventTypesToStop, eventType)
	}
	for _, stopEventType := range eventTypesToStop {
		if t.tracingState[stopEventType].gadget != nil {
			// Tracing gadget with nsmap control
			if t.tracingState[stopEventType].usageReferenceCount[mntns] == 0 {
				continue
			}
			t.tracingState[stopEventType].usageReferenceCount[mntns]--
			if t.tracingState[stopEventType].usageReferenceCount[mntns] == 0 {
				zero := uint32(0)
				mntnsC := uint64(mntns)
				t.tracingState[stopEventType].eBpfContainerFilterMap.Put(&mntnsC, &zero)
			}
		} else if t.tracingState[stopEventType].attachable != nil {
			// Tracing gadget with peekable interface
			t.tracingState[stopEventType].attachable.Detach(pid)
		} else {
			return fmt.Errorf("not a tracable event type")
		}
	}
	return nil
}

func (t *Tracer) GetListOfRunningContainers() (*[]ContainerActivityEvent, error) {
	if t == nil || !t.running {
		return nil, fmt.Errorf("tracing not running")
	}

	containers := t.cCollection.GetContainersBySelector(&containercollection.ContainerSelector{})
	var containerActivityEvents []ContainerActivityEvent
	for _, container := range containers {
		containerActivityEvents = append(containerActivityEvents, ContainerActivityEvent{
			PodName:       container.K8s.PodName,
			Namespace:     container.K8s.Namespace,
			ContainerName: container.K8s.ContainerName,
			NsMntId:       container.Mntns,
			ContainerID:   container.Runtime.ContainerID,
			Pid:           container.Pid,
			Activity:      ContainerActivityEventStart,
		})
	}

	return &containerActivityEvents, nil
}
