package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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
	running bool
}

type CollectorManager struct {
	// Map of container ID to container state
	containers map[ContainerId]*ContainerState

	// Kubernetes connection clien
	k8sClient     *kubernetes.Clientset
	dynamicClient *dynamic.DynamicClient

	// Event sink
	eventSink *eventsink.EventSink

	// Tracer
	tracer tracing.ITracer

	// config
	config CollectorManagerConfig
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
}

func StartCollectorManager(config *CollectorManagerConfig) (*CollectorManager, error) {
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
		containers:    make(map[ContainerId]*ContainerState),
		k8sClient:     client,
		dynamicClient: dynamicClient,
		config:        *config,
		eventSink:     config.EventSink,
		tracer:        config.Tracer,
	}

	// Setup container events listener
	cm.tracer.AddContainerActivityListener(cm)

	return cm, nil
}

func (cm *CollectorManager) StopCollectorManager() error {
	// Stop container events listener
	cm.tracer.RemoveContainerActivityListener(cm)

	return nil
}

func (cm *CollectorManager) ContainerStarted(id *ContainerId) {
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
	cm.containers[*id] = &ContainerState{
		running: true,
	}

	// Start event sink filter for container
	cm.eventSink.AddFilter(&eventsink.EventSinkFilter{
		ContainerID: id.ContainerID,
		EventType:   tracing.AllEventType,
	})

	// Get all events for this container
	err = cm.tracer.StartTraceContainer(id.NsMntId, id.Pid, tracing.AllEventType)
	if err != nil {
		log.Printf("error starting tracing container: %s - %v\n", err, id)
	}

	// Add a timer for collection of data from container events
	startContainerTimer(id, cm.config.Interval, cm.CollectContainerEvents)

	if cm.config.FinalizeTime > 0 && cm.config.FinalizeTime > cm.config.Interval {
		// Add a timer for finalizing the application profile
		startContainerTimer(id, cm.config.FinalizeTime, cm.FinalizeApplicationProfile)
	}
}

func (cm *CollectorManager) ContainerStopped(id *ContainerId) {
	// Check if container is still running (is it in the map?)
	if _, ok := cm.containers[*id]; ok {
		// Turn running state to false
		cm.containers[*id].running = false

		// Stop tracing container
		cm.tracer.StopTraceContainer(id.NsMntId, id.Pid, tracing.AllEventType)

		// Remove the this container from the filters of the event sink so that it does not collect events for it anymore
		cm.eventSink.RemoveFilter(&eventsink.EventSinkFilter{EventType: tracing.AllEventType, ContainerID: id.ContainerID})
	}

	// Collect data from container events
	go cm.CollectContainerEvents(id)
}

func (cm *CollectorManager) CollectContainerEvents(id *ContainerId) {
	// Check if container is still running (is it in the map?)
	if _, ok := cm.containers[*id]; ok {
		// Collect data from container events
		execveEvents, err := cm.eventSink.GetExecveEvents(id.Namespace, id.PodName, id.Container)
		if err != nil {
			log.Printf("error getting execve events: %s\n", err)
			return
		}

		openEvents, err := cm.eventSink.GetOpenEvents(id.Namespace, id.PodName, id.Container)
		if err != nil {
			log.Printf("error getting open events: %s\n", err)
			return
		}

		syscallList, err := cm.tracer.PeekSyscallInContainer(id.NsMntId)
		if err != nil {
			log.Printf("error getting syscall list: %s\n", err)
			return
		}

		capabilitiesEvents, err := cm.eventSink.GetCapabilitiesEvents(id.Namespace, id.PodName, id.Container)
		if err != nil {
			log.Printf("error getting capabilities events: %s\n", err)
			return
		}

		dnsEvents, err := cm.eventSink.GetDnsEvents(id.Namespace, id.PodName, id.Container)
		if err != nil {
			log.Printf("error getting dns events: %s\n", err)
			return
		}

		networkEvents, err := cm.eventSink.GetNetworkEvents(id.Namespace, id.PodName, id.Container)
		if err != nil {
			log.Printf("error getting network events: %s\n", err)
			return
		}

		// If there are no events, return
		if len(networkEvents) == 0 && len(dnsEvents) == 0 && len(execveEvents) == 0 && len(openEvents) == 0 && len(syscallList) == 0 && len(capabilitiesEvents) == 0 {
			return
		}

		containerProfile := ContainerProfile{Name: id.Container}

		// Add syscalls to container profile
		containerProfile.SysCalls = append(containerProfile.SysCalls, syscallList...)

		// Add execve events to container profile
		for _, event := range execveEvents {
			// TODO: check if event is already in containerProfile.Execs
			containerProfile.Execs = append(containerProfile.Execs, ExecCalls{
				Path: event.PathName,
				Args: event.Args,
				Envs: event.Env,
			})
		}

		// Add dns events to container profile
		for _, event := range dnsEvents {
			if !dnsEventExists(event, containerProfile.Dns) {
				containerProfile.Dns = append(containerProfile.Dns, DnsCalls{
					DnsName:   event.DnsName,
					Addresses: event.Addresses,
				})
			}
		}

		//interstingCapabilities := []string{"setpcap", "sysmodule", "net_raw", "net_admin", "sys_admin", "sys_rawio", "sys_ptrace", "sys_boot", "mac_override", "mac_admin", "perfmon", "all", "bpf"}
		// Add capabilities events to container profile
		for _, event := range capabilitiesEvents {
			// TODO: check if event is already in containerProfile.Capabilities
			//if slices.Contains(interstingCapabilities, event.CapabilityName) {
			if len(containerProfile.Capabilities) == 0 {
				containerProfile.Capabilities = append(containerProfile.Capabilities, CapabilitiesCalls{
					Capabilities: []string{event.CapabilityName},
					Syscall:      event.Syscall,
				})
			} else {
				for _, capability := range containerProfile.Capabilities {
					if capability.Syscall == event.Syscall {
						if !slices.Contains(capability.Capabilities, event.CapabilityName) {
							capability.Capabilities = append(capability.Capabilities, event.CapabilityName)
						}
					} else {
						var syscalls []string
						for _, cap := range containerProfile.Capabilities {
							syscalls = append(syscalls, cap.Syscall)
						}
						if !slices.Contains(syscalls, event.Syscall) {
							containerProfile.Capabilities = append(containerProfile.Capabilities, CapabilitiesCalls{
								Capabilities: []string{event.CapabilityName},
								Syscall:      event.Syscall,
							})
						}
					}
				}
			}
		}

		// Add open events to container profile
		for _, event := range openEvents {
			hasSameFile, hasSameFlags := openEventExists(event, containerProfile.Opens)
			// TODO: check if event is already in containerProfile.Opens & remove the 10000 limit.
			if len(containerProfile.Opens) < 10000 && !(hasSameFile && hasSameFlags) {
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
		for _, networkEvent := range networkEvents {
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
				// Remove the this container from the filters of the event sink so that it does not collect events for it anymore
				// This is an optimization to avoid collecting events for containers that are already finalized
				cm.eventSink.RemoveFilter(&eventsink.EventSinkFilter{EventType: tracing.AllEventType, ContainerID: id.ContainerID})
				return
			}

			appProfile := &ApplicationProfile{}

			// Add container profile to the list of containers
			appProfile.Spec.Containers = append(appProfile.Spec.Containers, containerProfile)

			// Convert the typed struct to json string.
			appProfileRawNew, err := json.Marshal(appProfile)
			if err != nil {
				log.Printf("error converting application profile: %s\n", err)
			} else {
				// Print the json string.
				_, err = cm.dynamicClient.Resource(AppProfileGvr).Namespace(id.Namespace).Patch(context.Background(),
					appProfileName, apitypes.MergePatchType, appProfileRawNew, v1.PatchOptions{})
				if err != nil {
					log.Printf("error patching application profile: %s\n", err)
				}
			}
		}

		// Restart timer
		startContainerTimer(id, cm.config.Interval, cm.CollectContainerEvents)
	}
}

func (cm *CollectorManager) FinalizeApplicationProfile(id *ContainerId) {
	// Check if container is still running (is it in the map?)
	if _, ok := cm.containers[*id]; ok {
		// Patch the application profile to make it immutable with the final annotation
		appProfileName := fmt.Sprintf("pod-%s", id.PodName)
		_, err := cm.dynamicClient.Resource(AppProfileGvr).Namespace(id.Namespace).Patch(context.Background(),
			appProfileName, apitypes.MergePatchType, []byte("{\"metadata\":{\"annotations\":{\"kapprofiler.kubescape.io/final\":\"true\"}}}"), v1.PatchOptions{})
		if err != nil {
			log.Printf("error patching application profile: %s\n", err)
		}
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
		})
	} else if event.Activity == tracing.ContainerActivityEventStop {
		cm.ContainerStopped(&ContainerId{
			Namespace:   event.Namespace,
			PodName:     event.PodName,
			Container:   event.ContainerName,
			NsMntId:     event.NsMntId,
			ContainerID: event.ContainerID,
			Pid:         event.Pid,
		})
	}
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
					log.Print("Event exists, appending missing address")
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
