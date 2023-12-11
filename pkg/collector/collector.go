package collector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kubescape/kapprofiler/pkg/eventsink"
	"github.com/kubescape/kapprofiler/pkg/tracing"

	"golang.org/x/exp/slices"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	RecordStrategyAlways          = "always"
	RecordStrategyOnlyIfNotExists = "only-if-not-exists"
	MaxOpenEvents                 = 10000 // Per container profile.
	MaxNetworkEvents              = 10000 // Per container profile.
)

type ContainerId struct {
	Namespace string
	PodName   string
	Container string
	// Low level identifiers
	ContainerID string
	NsMntId     uint64
	Pid         uint32
}

type ContainerState struct {
	running  bool
	attached bool
}

type CollectorManager struct {
	// Map of container ID to container state
	containers map[ContainerId]*ContainerState

	// Map mutex
	containersMutex *sync.Mutex

	// Kubernetes connection clien
	k8sClient     *kubernetes.Clientset
	dynamicClient *dynamic.DynamicClient

	// Event sink
	eventSink *eventsink.EventSink

	// Tracer
	tracer tracing.ITracer

	// config
	config CollectorManagerConfig

	// Pod finalizer watcher
	podFinalizerControl chan struct{}

	// Pod finalizer state table
	podFinalizerState map[string]*PodProfileFinalizerState

	// Mutex for pod finalizer state table
	podFinalizerStateMutex *sync.Mutex
}

type CollectorManagerConfig struct {
	// Event sink object
	EventSink *eventsink.EventSink
	// Interval in seconds for collecting data from containers
	Interval uint64
	// Finalize application profiles time
	FinalizeTime uint64
	// Kubernetes configuration
	K8sConfig *rest.Config
	// Tracer object
	Tracer tracing.ITracer
	// Record strategy
	RecordStrategy string
	// Node name
	NodeName string
}

type TotalEvents struct {
	ExecEvents         []*tracing.ExecveEvent
	OpenEvents         []*tracing.OpenEvent
	SyscallEvents      []string
	CapabilitiesEvents []*tracing.CapabilitiesEvent
	DnsEvents          []*tracing.DnsEvent
	NetworkEvents      []*tracing.NetworkEvent
}

func StartCollectorManager(config *CollectorManagerConfig) (*CollectorManager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.NodeName == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}
	// Get Kubernetes client
	client, err := kubernetes.NewForConfig(config.K8sConfig)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config.K8sConfig)
	if err != nil {
		return nil, err
	}
	cm := &CollectorManager{
		containers:      make(map[ContainerId]*ContainerState),
		containersMutex: &sync.Mutex{},
		k8sClient:       client,
		dynamicClient:   dynamicClient,
		config:          *config,
		eventSink:       config.EventSink,
		tracer:          config.Tracer,
	}

	// Setup container events listener
	cm.tracer.AddContainerActivityListener(cm)

	// Start finalizer watcher
	cm.StartFinalizerWatcher()

	return cm, nil
}

func (cm *CollectorManager) StopCollectorManager() error {
	// Stop container events listener
	cm.tracer.RemoveContainerActivityListener(cm)

	// Stop finalizer watcher
	cm.StopFinalizerWatcher()

	return nil
}

func (cm *CollectorManager) ContainerStarted(id *ContainerId, attach bool) {
	// Check if applicaton profile already exists
	appProfileExists, err := cm.doesApplicationProfileExists(id.Namespace, id.PodName, true, true)
	if err != nil {
		//log.Printf("error checking if application profile exists: %s\n", err)
	} else if appProfileExists {
		// If application profile exists, check if record strategy is RecordStrategyOnlyIfNotExists
		if cm.config.RecordStrategy == RecordStrategyOnlyIfNotExists {
			// Do not start recording events for this container
			return
		}
	}

	// Add container to map with running state set to true
	cm.containersMutex.Lock()
	cm.containers[*id] = &ContainerState{
		running:  true,
		attached: attach,
	}

	// Start event sink filter for container
	cm.eventSink.AddFilter(&eventsink.EventSinkFilter{
		ContainerID: id.ContainerID,
		EventType:   tracing.AllEventType,
	})
	cm.containersMutex.Unlock()

	// Get all events for this container
	err = cm.tracer.StartTraceContainer(id.NsMntId, id.Pid, tracing.AllEventType)
	if err != nil {
		log.Printf("error starting tracing container: %s - %v\n", err, id)
	}

	// Add a timer for collection of data from container events
	startContainerTimer(id, cm.config.Interval, cm.CollectContainerEvents)

	if cm.config.FinalizeTime > 0 && cm.config.FinalizeTime > cm.config.Interval {
		cm.MarkPodRecording(id.PodName, id.Namespace, attach)
	}
}

func (cm *CollectorManager) ContainerStopped(id *ContainerId) {
	// Check if container is still running (is it in the map?)
	cm.containersMutex.Lock()
	defer cm.containersMutex.Unlock()
	if _, ok := cm.containers[*id]; ok {
		// Turn running state to false
		cm.containers[*id].running = false

		// Mark stop recording
		cm.MarkPodNotRecording(id.PodName, id.Namespace)

		// Stop tracing container
		cm.tracer.StopTraceContainer(id.NsMntId, id.Pid, tracing.AllEventType)

		// Remove this container from the filters of the event sink so that it does not collect events for it anymore
		cm.eventSink.RemoveFilter(&eventsink.EventSinkFilter{EventType: tracing.AllEventType, ContainerID: id.ContainerID})
		// Remove container from map
		delete(cm.containers, *id)
	}

	// Collect data from container events
	go cm.CollectContainerEvents(id)
}

func (cm *CollectorManager) loadTotalEvents(containerId *ContainerId) (*TotalEvents, error) {
	allEvents := TotalEvents{}
	// Get all events for this container
	execEvents, err := cm.eventSink.GetExecveEvents(containerId.Namespace, containerId.PodName, containerId.Container)
	if err == nil {
		allEvents.ExecEvents = execEvents
	} else {
		log.Printf("error getting execve events: %s\n", err)
	}

	openEvents, err := cm.eventSink.GetOpenEvents(containerId.Namespace, containerId.PodName, containerId.Container)
	if err == nil {
		allEvents.OpenEvents = openEvents
	} else {
		log.Printf("error getting open events: %s\n", err)
	}

	syscallEvents, err := cm.tracer.PeekSyscallInContainer(containerId.NsMntId)
	if err != nil {
		if strings.Contains(err.Error(), "no syscall found") {
			allEvents.SyscallEvents = []string{}
		} else {
			log.Printf("error getting syscall events: %s\n", err)
		}
	} else {
		allEvents.SyscallEvents = syscallEvents
	}

	capabilitiesEvents, err := cm.eventSink.GetCapabilitiesEvents(containerId.Namespace, containerId.PodName, containerId.Container)
	if err == nil {
		allEvents.CapabilitiesEvents = capabilitiesEvents
	} else {
		log.Printf("error getting capabilities events: %s\n", err)
	}

	dnsEvents, err := cm.eventSink.GetDnsEvents(containerId.Namespace, containerId.PodName, containerId.Container)
	if err == nil {
		allEvents.DnsEvents = dnsEvents
	} else {
		log.Printf("error getting dns events: %s\n", err)
	}

	networkEvents, err := cm.eventSink.GetNetworkEvents(containerId.Namespace, containerId.PodName, containerId.Container)
	if err == nil {
		allEvents.NetworkEvents = networkEvents
	} else {
		log.Printf("error getting network events: %s\n", err)
	}

	return &allEvents, nil
}

func shouldProcessEvents(totalEvents *TotalEvents) bool {
	return len(totalEvents.ExecEvents) > 0 || len(totalEvents.OpenEvents) > 0 || len(totalEvents.SyscallEvents) > 0 || len(totalEvents.CapabilitiesEvents) > 0 || len(totalEvents.DnsEvents) > 0 || len(totalEvents.NetworkEvents) > 0
}

func (cm *CollectorManager) CollectContainerEvents(id *ContainerId) {
	// Check if container is still running (is it in the map?)
	cm.containersMutex.Lock()
	if containerState, ok := cm.containers[*id]; ok {
		cm.containersMutex.Unlock()
		// Collect data from container events
		totalEvents, err := cm.loadTotalEvents(id)
		if err != nil {
			log.Printf("error loading total events: %s\n", err)
			return
		}

		// If there are no events, return
		if !shouldProcessEvents(totalEvents) {
			return
		}

		containerProfile := ContainerProfile{Name: id.Container}

		// Add syscalls to container profile
		containerProfile.SysCalls = append(containerProfile.SysCalls, totalEvents.SyscallEvents...)

		// Add execve events to container profile
		for _, event := range totalEvents.ExecEvents {
			// Check if execve event is already in container profile or if it has no path name (Some execve events do not have a path name).
			if !execEventExists(event, containerProfile.Execs) || event.PathName == "" {
				containerProfile.Execs = append(containerProfile.Execs, ExecCalls{
					Path: event.PathName,
					Args: event.Args,
					Envs: event.Env,
				})
			}
		}

		// Add dns events to container profile
		for _, event := range totalEvents.DnsEvents {
			if !dnsEventExists(event, containerProfile.Dns) {
				containerProfile.Dns = append(containerProfile.Dns, DnsCalls{
					DnsName:   event.DnsName,
					Addresses: event.Addresses,
				})
			}
		}

		// Add capabilities events to container profile
		for _, event := range totalEvents.CapabilitiesEvents {
			var syscallExists bool
			for i, capability := range containerProfile.Capabilities {
				if capability.Syscall == event.Syscall {
					syscallExists = true
					if !slices.Contains(capability.Capabilities, event.CapabilityName) {
						containerProfile.Capabilities[i].Capabilities = append(capability.Capabilities, event.CapabilityName)
					}
					break
				}
			}

			if !syscallExists {
				containerProfile.Capabilities = append(containerProfile.Capabilities, CapabilitiesCalls{
					Capabilities: []string{event.CapabilityName},
					Syscall:      event.Syscall,
				})
			}
		}

		// Add open events to container profile
		for _, event := range totalEvents.OpenEvents {
			hasSameFile, hasSameFlags := openEventExists(event, containerProfile.Opens)
			// TODO: check if event is already in containerProfile.Opens & remove the 10000 limit.
			if len(containerProfile.Opens) < MaxOpenEvents && !(hasSameFile && hasSameFlags) {
				openEvent := OpenCalls{
					Path:  event.PathName,
					Flags: event.Flags,
				}
				containerProfile.Opens = append(containerProfile.Opens, openEvent)
			}
		}

		// Add network activity to container profile
		var outgoingConnections []NetworkCalls
		var incomingConnections []NetworkCalls
		for _, networkEvent := range totalEvents.NetworkEvents {
			if networkEvent.PacketType == "OUTGOING" {
				if !networkEventExists(networkEvent, outgoingConnections) {
					outgoingConnections = append(outgoingConnections, NetworkCalls{
						Protocol:    networkEvent.Protocol,
						Port:        networkEvent.Port,
						DstEndpoint: networkEvent.DstEndpoint,
					})
				}
			} else if networkEvent.PacketType == "HOST" {
				if !networkEventExists(networkEvent, incomingConnections) {
					incomingConnections = append(incomingConnections, NetworkCalls{
						Protocol:    networkEvent.Protocol,
						Port:        networkEvent.Port,
						DstEndpoint: networkEvent.DstEndpoint,
					})
				}
			}
		}

		containerProfile.NetworkActivity = NetworkActivity{
			Incoming: incomingConnections,
			Outgoing: outgoingConnections,
		}

		// The name of the ApplicationProfile you're looking for.
		appProfileName := fmt.Sprintf("pod-%s", id.PodName)

		// Get the ApplicationProfile object with the name specified above.
		existingApplicationProfile, err := cm.dynamicClient.Resource(AppProfileGvr).Namespace(id.Namespace).Get(context.Background(), appProfileName, v1.GetOptions{})
		if err != nil {
			// it does not exist, create it
			appProfile := &ApplicationProfile{
				TypeMeta: v1.TypeMeta{
					Kind:       ApplicationProfileKind,
					APIVersion: ApplicationProfileApiVersion,
				},
				ObjectMeta: v1.ObjectMeta{
					Name: appProfileName,
				},
				Spec: ApplicationProfileSpec{
					Containers: []ContainerProfile{containerProfile},
				},
			}
			if containerState.attached {
				appProfile.ObjectMeta.Annotations = map[string]string{"kapprofiler.kubescape.io/partial": "true"}
			}
			appProfileRawNew, err := runtime.DefaultUnstructuredConverter.ToUnstructured(appProfile)
			if err != nil {
				log.Printf("error converting application profile: %s\n", err)
			}
			_, err = cm.dynamicClient.Resource(AppProfileGvr).Namespace(id.Namespace).Create(
				context.Background(),
				&unstructured.Unstructured{
					Object: appProfileRawNew,
				},
				v1.CreateOptions{})
			if err != nil {
				log.Printf("error creating application profile: %s\n", err)
			}
		} else {
			// if the application profile is final (immutable), we cannot patch it
			if existingApplicationProfile.GetAnnotations()["kapprofiler.kubescape.io/final"] == "true" {
				// Remove this container from the filters of the event sink so that it does not collect events for it anymore
				cm.eventSink.RemoveFilter(&eventsink.EventSinkFilter{EventType: tracing.AllEventType, ContainerID: id.ContainerID})
				// Stop tracing container
				cm.tracer.StopTraceContainer(id.NsMntId, id.Pid, tracing.AllEventType)

				// Mark stop recording
				cm.MarkPodNotRecording(id.PodName, id.Namespace)

				// Remove the container from the map
				cm.containersMutex.Lock()
				delete(cm.containers, *id)
				cm.containersMutex.Unlock()

				return
			}

			// Add the container profile into the application profile. If the container profile already exists, it will be merged.
			existingApplicationProfileObject := &ApplicationProfile{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(existingApplicationProfile.Object, existingApplicationProfileObject)
			if err != nil {
				log.Printf("error unmarshalling application profile: %s\n", err)
			}

			// If not attached (seen the container from the start) and partial annotation is set, remove it
			if !containerState.attached && existingApplicationProfile.GetAnnotations()["kapprofiler.kubescape.io/partial"] == "true" {
				log.Printf("Removing partial annotation from application profile %s\n", appProfileName)
				existingApplicationProfileObject.ObjectMeta.Annotations = map[string]string{"kapprofiler.kubescape.io/partial": "false"}
			}

			mergedAppProfile := cm.mergeApplicationProfiles(existingApplicationProfileObject, &containerProfile)
			unstructuredAppProfile, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mergedAppProfile)
			if err != nil {
				log.Printf("error converting application profile: %s\n", err)
			}
			_, err = cm.dynamicClient.Resource(AppProfileGvr).Namespace(id.Namespace).Update(
				context.Background(),
				&unstructured.Unstructured{
					Object: unstructuredAppProfile,
				},
				v1.UpdateOptions{})
			if err != nil {
				log.Printf("error updating application profile: %s\n", err)
			}
		}

		// Restart timer
		startContainerTimer(id, cm.config.Interval, cm.CollectContainerEvents)
	} else {
		cm.containersMutex.Unlock()
	}
}

func (cm *CollectorManager) mergeApplicationProfiles(existingApplicationProfile *ApplicationProfile, containerProfile *ContainerProfile) *ApplicationProfile {
	// Add container profile to the list of containers or merge it with the existing one.
	for i, existingContainerProfile := range existingApplicationProfile.Spec.Containers {
		if existingContainerProfile.Name == containerProfile.Name {
			// Merge container profile
			existingContainer := existingApplicationProfile.Spec.Containers[i]

			// Merge syscalls
			filteredSyscalls := []string{}
			for _, syscall := range containerProfile.SysCalls {
				if !slices.Contains(existingContainer.SysCalls, syscall) {
					filteredSyscalls = append(filteredSyscalls, syscall)
				}
			}
			existingContainer.SysCalls = append(existingContainer.SysCalls, filteredSyscalls...)

			// Merge execve events
			filteredExecs := []ExecCalls{}
			for _, exec := range containerProfile.Execs {
				if !execEventExists(&tracing.ExecveEvent{PathName: exec.Path, Args: exec.Args, Env: exec.Envs}, existingContainer.Execs) {
					filteredExecs = append(filteredExecs, exec)
				}
			}
			existingContainer.Execs = append(existingContainer.Execs, filteredExecs...)

			// Merge dns events
			filteredDns := []DnsCalls{}
			for _, dns := range containerProfile.Dns {
				if !dnsEventExists(&tracing.DnsEvent{DnsName: dns.DnsName, Addresses: dns.Addresses}, existingContainer.Dns) {
					filteredDns = append(filteredDns, dns)
				}
			}
			existingContainer.Dns = append(existingContainer.Dns, filteredDns...)

			// Merge capabilities events
			filteredCapabilities := []CapabilitiesCalls{}
			for _, capability := range containerProfile.Capabilities {
				syscallExists := false
				for i, existingCapability := range existingContainer.Capabilities {
					if existingCapability.Syscall == capability.Syscall {
						syscallExists = true
						for _, cap := range capability.Capabilities {
							if !slices.Contains(existingCapability.Capabilities, cap) {
								existingContainer.Capabilities[i].Capabilities = append(existingCapability.Capabilities, cap)
							}
						}
						break
					}
				}
				if !syscallExists {
					filteredCapabilities = append(filteredCapabilities, capability)
				}
			}
			existingContainer.Capabilities = append(existingContainer.Capabilities, filteredCapabilities...)

			// Merge open events
			filteredOpens := []OpenCalls{}
			for _, open := range containerProfile.Opens {
				hasSameFile, hasSameFlags := openEventExists(&tracing.OpenEvent{PathName: open.Path, Flags: open.Flags}, existingContainer.Opens)
				if len(existingContainer.Opens) < MaxOpenEvents && !(hasSameFile && hasSameFlags) {
					filteredOpens = append(filteredOpens, open)
				}
			}
			existingContainer.Opens = append(existingContainer.Opens, filteredOpens...)

			// Merge network activity
			for _, networkEvent := range containerProfile.NetworkActivity.Incoming {
				if len(existingContainer.NetworkActivity.Incoming) < MaxNetworkEvents && !networkEventExists(&tracing.NetworkEvent{DstEndpoint: networkEvent.DstEndpoint, Port: networkEvent.Port, Protocol: networkEvent.Protocol}, existingContainer.NetworkActivity.Incoming) {
					existingContainer.NetworkActivity.Incoming = append(existingContainer.NetworkActivity.Incoming, networkEvent)
				}
			}
			for _, networkEvent := range containerProfile.NetworkActivity.Outgoing {
				if len(existingContainer.NetworkActivity.Outgoing) < MaxNetworkEvents && !networkEventExists(&tracing.NetworkEvent{DstEndpoint: networkEvent.DstEndpoint, Port: networkEvent.Port, Protocol: networkEvent.Protocol}, existingContainer.NetworkActivity.Outgoing) {
					existingContainer.NetworkActivity.Outgoing = append(existingContainer.NetworkActivity.Outgoing, networkEvent)
				}
			}

			// Replace container profile
			existingApplicationProfile.Spec.Containers[i] = existingContainer
			return existingApplicationProfile
		}
	}

	// Add container profile to the list of containers
	existingApplicationProfile.Spec.Containers = append(existingApplicationProfile.Spec.Containers, *containerProfile)

	return existingApplicationProfile
}

func (cm *CollectorManager) FinalizeApplicationProfile(id *ContainerId) {
	// Check if container is still running (is it in the map?)
	cm.containersMutex.Lock()
	if _, ok := cm.containers[*id]; ok {
		cm.containersMutex.Unlock()
		// Patch the application profile to make it immutable with the final annotation
		appProfileName := fmt.Sprintf("pod-%s", id.PodName)
		_, err := cm.dynamicClient.Resource(AppProfileGvr).Namespace(id.Namespace).Patch(context.Background(),
			appProfileName, apitypes.MergePatchType, []byte("{\"metadata\":{\"annotations\":{\"kapprofiler.kubescape.io/final\":\"true\"}}}"), v1.PatchOptions{})
		if err != nil {
			log.Printf("error patching application profile: %s\n", err)
		}
	} else {
		cm.containersMutex.Unlock()
	}
}

func (cm *CollectorManager) doesApplicationProfileExists(namespace string, podName string, checkFinal bool, checkOwner bool) (bool, error) {
	workloadKind := "Pod"
	workloadName := podName
	if checkOwner {
		// Get the highest level owner of the pod
		pod, err := cm.k8sClient.CoreV1().Pods(namespace).Get(context.Background(), podName, v1.GetOptions{})
		if err != nil {
			return false, err
		}
		ownerReferences := pod.GetOwnerReferences()
		if len(ownerReferences) > 0 {
			for _, owner := range ownerReferences {
				if owner.Controller != nil && *owner.Controller {
					workloadKind = owner.Kind
					workloadName = owner.Name
					break
				}
			}
			// If ReplicaSet is the owner, get the Deployment
			if workloadKind == "ReplicaSet" {
				replicaSet, err := cm.k8sClient.AppsV1().ReplicaSets(namespace).Get(context.Background(), workloadName, v1.GetOptions{})
				if err != nil {
					return false, err
				}
				ownerReferences := replicaSet.GetOwnerReferences()
				if len(ownerReferences) > 0 {
					for _, owner := range ownerReferences {
						if owner.Controller != nil && *owner.Controller {
							workloadKind = owner.Kind
							workloadName = owner.Name
							break
						}
					}
				}
			}
		}
	}

	// The name of the ApplicationProfile you're looking for.
	appProfileName := fmt.Sprintf("%s-%s", strings.ToLower(workloadKind), strings.ToLower(workloadName))

	// Get the ApplicationProfile object with the name specified above.
	existingApplicationProfile, err := cm.dynamicClient.Resource(AppProfileGvr).Namespace(namespace).Get(context.Background(), appProfileName, v1.GetOptions{})
	if err != nil {
		return false, err
	}

	// if the application profile is final (immutable), we cannot patch it
	if checkFinal && existingApplicationProfile.GetAnnotations()["kapprofiler.kubescape.io/final"] != "true" {
		return false, nil
	}

	return true, nil
}

// Timer function
func startContainerTimer(id *ContainerId, seconds uint64, callback func(id *ContainerId)) *time.Timer {
	timer := time.NewTimer(time.Duration(seconds) * time.Second)

	// This goroutine waits for the timer to finish.
	go func() {
		<-timer.C
		callback(id)
	}()

	return timer
}

func (cm *CollectorManager) OnContainerActivityEvent(event *tracing.ContainerActivityEvent) {
	if event.Activity == tracing.ContainerActivityEventStart {
		cm.ContainerStarted(&ContainerId{
			Namespace:   event.Namespace,
			PodName:     event.PodName,
			Container:   event.ContainerName,
			NsMntId:     event.NsMntId,
			ContainerID: event.ContainerID,
			Pid:         event.Pid,
		}, false)
	} else if event.Activity == tracing.ContainerActivityEventStop {
		cm.ContainerStopped(&ContainerId{
			Namespace:   event.Namespace,
			PodName:     event.PodName,
			Container:   event.ContainerName,
			NsMntId:     event.NsMntId,
			ContainerID: event.ContainerID,
			Pid:         event.Pid,
		})
	} else if event.Activity == tracing.ContainerActivityEventAttached {
		cm.ContainerStarted(&ContainerId{
			Namespace:   event.Namespace,
			PodName:     event.PodName,
			Container:   event.ContainerName,
			NsMntId:     event.NsMntId,
			ContainerID: event.ContainerID,
			Pid:         event.Pid,
		}, true)
	}
}

func execEventExists(execEvent *tracing.ExecveEvent, execCalls []ExecCalls) bool {
	for _, call := range execCalls {
		if execEvent.PathName == call.Path && slices.Equal(execEvent.Args, call.Args) && slices.Equal(execEvent.Env, call.Envs) {
			return true
		}
	}

	return false
}

func networkEventExists(networkEvent *tracing.NetworkEvent, networkCalls []NetworkCalls) bool {
	for _, call := range networkCalls {
		if networkEvent.DstEndpoint == call.DstEndpoint && networkEvent.Port == call.Port && networkEvent.Protocol == call.Protocol {
			return true
		}
	}

	return false
}

func dnsEventExists(dnsEvent *tracing.DnsEvent, dnsCalls []DnsCalls) bool {
	for _, call := range dnsCalls {
		if dnsEvent.DnsName == call.DnsName {
			for _, address := range dnsEvent.Addresses {
				if !slices.Contains(call.Addresses, address) {
					call.Addresses = append(call.Addresses, address)
				}
			}
			return true
		}
	}
	return false
}

func openEventExists(openEvent *tracing.OpenEvent, openEvents []OpenCalls) (bool, bool) {
	hasSamePath := false
	hasSameFlags := false
	for _, element := range openEvents {
		if element.Path == openEvent.PathName {
			hasSamePath = true
			hasAllFlags := true
			for _, flag := range openEvent.Flags {
				// Check if flag is in the flags of the openEvent
				hasFlag := false
				for _, flag2 := range element.Flags {
					if flag == flag2 {
						hasFlag = true
						break
					}
				}
				if !hasFlag {
					hasAllFlags = false
					break
				}
			}
			if hasAllFlags {
				hasSameFlags = true
				break
			}
		}
		if hasSamePath && hasSameFlags {
			break
		}
	}

	return hasSamePath, hasSameFlags
}
