package collector

import (
	"context"
	"fmt"
	"kapprofiler/pkg/eventsink"
	"kapprofiler/pkg/tracing"
	"log"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ContainerId struct {
	Namespace string
	PodName   string
	Container string
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

	// config
	config CollectorManagerConfig
}

type CollectorManagerConfig struct {
	// Event sink object
	EventSink *eventsink.EventSink
	// Interval in seconds for collecting data from containers
	Interval uint64
	// Kubernetes configuration
	K8sConfig *rest.Config
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
	}

	return cm, nil
}

func (cm *CollectorManager) StopCollectorManager() error {
	return nil
}

func (cm *CollectorManager) ContainerStarted(id *ContainerId) {
	// Add container to map with running state set to true
	cm.containers[*id] = &ContainerState{
		running: true,
	}
	// Add a timer for collection of data from container events
	startContainerTimer(id, cm.config.Interval, cm.CollectContainerEvents)
}

func (cm *CollectorManager) ContainerStopped(id *ContainerId) {
	// Check if container is still running (is it in the map?)
	if _, ok := cm.containers[*id]; ok {
		// Turn running state to false
		cm.containers[*id].running = false
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

		tcpEvents, err := cm.eventSink.GetTcpEvents(id.Namespace, id.PodName, id.Container)
		if err != nil {
			log.Printf("error getting tcp events: %s\n", err)
			return
		}

		// If there are no events, return
		if len(execveEvents) == 0 && len(tcpEvents) == 0 {
			return
		}

		containerProfile := ContainerProfile{Name: id.Container}
		for _, event := range execveEvents {
			// TODO: check if event is already in containerProfile.Execs
			containerProfile.Execs = append(containerProfile.Execs, ExecCalls{
				Path: event.PathName,
				Args: event.Args,
				Envs: event.Env,
			})
		}

		var outgoingTcpConnections []EnrichedTcpConnection
		var incomingTcpConnections []EnrichedTcpConnection
		// Add network activity to container profile
		for _, tcpEvent := range tcpEvents {
			if tcpEvent.Operation == "connect" {
				outgoingTcpConnections = append(outgoingTcpConnections, EnrichedTcpConnection{
					RawConnection: RawTcpConnection{
						SourceIp:   tcpEvent.Source,
						SourcePort: tcpEvent.SourcePort,
						DestIp:     tcpEvent.Destination,
						DestPort:   tcpEvent.DestPort,
					},
				})
			} else if tcpEvent.Operation == "accept" {
				incomingTcpConnections = append(incomingTcpConnections, EnrichedTcpConnection{
					RawConnection: RawTcpConnection{
						SourceIp:   tcpEvent.Source,
						SourcePort: tcpEvent.SourcePort,
						DestIp:     tcpEvent.Destination,
						DestPort:   tcpEvent.DestPort,
					},
				})
			}
		}

		containerProfile.NetworkActivity = NetworkActivity{
			Incoming: ConnectionContainer{
				TcpConnections: incomingTcpConnections,
			},
			Outgoing: ConnectionContainer{
				TcpConnections: outgoingTcpConnections,
			},
		}

		// The name of the ApplicationProfile you're looking for.
		appProfileName := fmt.Sprintf("pod-%s", id.PodName)

		// Get the ApplicationProfile object with the name specified above.
		appProfileRaw, err := cm.dynamicClient.Resource(AppProfileGvr).Namespace(id.Namespace).Get(context.Background(), appProfileName, v1.GetOptions{})
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
			// Convert the raw object to a typed struct.
			appProfile := &ApplicationProfile{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(appProfileRaw.Object, appProfile)
			if err != nil {
				log.Printf("error converting application profile: %s\n", err)
			}
			// Delete the same container if it exists
			for i, container := range appProfile.Spec.Containers {
				if container.Name == id.Container {
					appProfile.Spec.Containers = append(appProfile.Spec.Containers[:i], appProfile.Spec.Containers[i+1:]...)
					break
				}
			}

			// it exists, update it
			appProfile.Spec.Containers = append(appProfile.Spec.Containers, containerProfile)

			// Convert the typed struct to a raw object.
			appProfileRawNew, err := runtime.DefaultUnstructuredConverter.ToUnstructured(appProfile)
			_, err = cm.dynamicClient.Resource(AppProfileGvr).Namespace(id.Namespace).Update(
				context.Background(),
				&unstructured.Unstructured{
					Object: appProfileRawNew,
				},
				v1.UpdateOptions{})
			if err != nil {
				log.Printf("error updating application profile: %s\n", err)
			}
		}
	}
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
			Namespace: event.Namespace,
			PodName:   event.PodName,
			Container: event.ContainerName,
		})
	} else if event.Activity == tracing.ContainerActivityEventStop {
		cm.ContainerStopped(&ContainerId{
			Namespace: event.Namespace,
			PodName:   event.PodName,
			Container: event.ContainerName,
		})
	}
}
