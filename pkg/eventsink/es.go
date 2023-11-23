package eventsink

import (
	"fmt"

	"github.com/kubescape/kapprofiler/pkg/inmemorymapdb"
	"github.com/kubescape/kapprofiler/pkg/tracing"
)

type EventSink struct {
	filterEvents bool

	execveEventChannel chan *tracing.ExecveEvent
	execvEventDB       *inmemorymapdb.InMemoryMapDB[*tracing.ExecveEvent]

	openEventChannel chan *tracing.OpenEvent
	openEventDB      *inmemorymapdb.InMemoryMapDB[*tracing.OpenEvent]

	capabilitiesEventChannel chan *tracing.CapabilitiesEvent
	capabilitiesEventDB      *inmemorymapdb.InMemoryMapDB[*tracing.CapabilitiesEvent]

	dnsEventChannel chan *tracing.DnsEvent
	dnsEventDB      *inmemorymapdb.InMemoryMapDB[*tracing.DnsEvent]

	networkEventChannel chan *tracing.NetworkEvent
	networkEventDB      *inmemorymapdb.InMemoryMapDB[*tracing.NetworkEvent]

	eventFilters []*EventSinkFilter
}

type EventSinkFilter struct {
	EventType   tracing.EventType
	ContainerID string
}

func NewEventSink(homeDir string, filterEvents bool) (*EventSink, error) {
	return &EventSink{filterEvents: filterEvents}, nil
}

func (es *EventSink) Start() error {

	// Create the channel for execve events
	es.execveEventChannel = make(chan *tracing.ExecveEvent, 10000)
	es.execvEventDB = inmemorymapdb.NewInMemoryMapDB[*tracing.ExecveEvent](100)
	// Create the channel for the open events
	es.openEventChannel = make(chan *tracing.OpenEvent, 10000)
	es.openEventDB = inmemorymapdb.NewInMemoryMapDB[*tracing.OpenEvent](100)
	// Create the channel for the capabilities events
	es.capabilitiesEventChannel = make(chan *tracing.CapabilitiesEvent, 10000)
	es.capabilitiesEventDB = inmemorymapdb.NewInMemoryMapDB[*tracing.CapabilitiesEvent](100)
	// Create the channel for the dns events
	es.dnsEventChannel = make(chan *tracing.DnsEvent, 10000)
	es.dnsEventDB = inmemorymapdb.NewInMemoryMapDB[*tracing.DnsEvent](100)
	// Create the channel for the network events
	es.networkEventChannel = make(chan *tracing.NetworkEvent, 10000)
	es.networkEventDB = inmemorymapdb.NewInMemoryMapDB[*tracing.NetworkEvent](100)
	// Start the execve event worker
	go es.execveEventWorker()

	// Start the open event worker
	go es.openEventWorker()

	// Start the capabilities event worker
	go es.capabilitiesEventWorker()

	// Start the dns event worker
	go es.dnsEventWorker()

	// Start the network event worker
	go es.networkEventWorker()

	return nil
}

func (es *EventSink) Stop() error {
	// Close the channel for execve events
	close(es.execveEventChannel)

	// Close the channel for open events
	close(es.openEventChannel)

	// Close the channel for capabilities events
	close(es.capabilitiesEventChannel)

	// Close the channel for dns events
	close(es.dnsEventChannel)

	// Close the channel for network events
	close(es.networkEventChannel)

	return nil
}

func (es *EventSink) AddFilter(filter *EventSinkFilter) {
	// Check that it doesn't already exist
	for _, f := range es.eventFilters {
		if f.EventType == filter.EventType && f.ContainerID == filter.ContainerID {
			return
		}
	}
	es.eventFilters = append(es.eventFilters, filter)
}

func (es *EventSink) RemoveFilter(filter *EventSinkFilter) {
	// Check that it exists
	for i, f := range es.eventFilters {
		if f.ContainerID == filter.ContainerID && (f.EventType == filter.EventType || filter.EventType == tracing.AllEventType) {
			es.eventFilters = append(es.eventFilters[:i], es.eventFilters[i+1:]...)
			return
		}
	}
}

func (es *EventSink) networkEventWorker() error {
	for event := range es.networkEventChannel {
		bucket := fmt.Sprintf("network-%s-%s-%s", event.Namespace, event.PodName, event.ContainerName)
		es.networkEventDB.Put(bucket, event)
	}

	return nil
}

func (es *EventSink) dnsEventWorker() error {
	for event := range es.dnsEventChannel {
		bucket := fmt.Sprintf("dns-%s-%s-%s", event.Namespace, event.PodName, event.ContainerName)
		es.dnsEventDB.Put(bucket, event)
	}

	return nil
}

func (es *EventSink) capabilitiesEventWorker() error {
	for event := range es.capabilitiesEventChannel {
		bucket := fmt.Sprintf("capabilities-%s-%s-%s", event.Namespace, event.PodName, event.ContainerName)
		es.capabilitiesEventDB.Put(bucket, event)
	}

	return nil
}

func (es *EventSink) openEventWorker() error {
	for event := range es.openEventChannel {
		bucket := fmt.Sprintf("open-%s-%s-%s", event.Namespace, event.PodName, event.ContainerName)
		es.openEventDB.Put(bucket, event)
	}

	return nil
}

func (es *EventSink) execveEventWorker() error {
	for event := range es.execveEventChannel {
		bucket := fmt.Sprintf("execve-%s-%s-%s", event.Namespace, event.PodName, event.ContainerName)
		es.execvEventDB.Put(bucket, event)
	}

	return nil
}

func (es *EventSink) CleanupContainer(namespace string, podName string, containerID string) error {
	bucket := fmt.Sprintf("execve-%s-%s-%s", namespace, podName, containerID)
	es.execvEventDB.Delete(bucket)

	bucket = fmt.Sprintf("open-%s-%s-%s", namespace, podName, containerID)
	es.openEventDB.Delete(bucket)

	bucket = fmt.Sprintf("capabilities-%s-%s-%s", namespace, podName, containerID)
	es.capabilitiesEventDB.Delete(bucket)

	bucket = fmt.Sprintf("dns-%s-%s-%s", namespace, podName, containerID)
	es.dnsEventDB.Delete(bucket)

	bucket = fmt.Sprintf("network-%s-%s-%s", namespace, podName, containerID)
	es.networkEventDB.Delete(bucket)

	return nil
}

func (es *EventSink) GetNetworkEvents(namespace string, podName string, containerID string) ([]*tracing.NetworkEvent, error) {
	bucket := fmt.Sprintf("network-%s-%s-%s", namespace, podName, containerID)
	return es.networkEventDB.GetNClean(bucket), nil
}

func (es *EventSink) GetDnsEvents(namespace string, podName string, containerID string) ([]*tracing.DnsEvent, error) {
	bucket := fmt.Sprintf("dns-%s-%s-%s", namespace, podName, containerID)
	return es.dnsEventDB.GetNClean(bucket), nil
}

func (es *EventSink) GetCapabilitiesEvents(namespace string, podName string, containerID string) ([]*tracing.CapabilitiesEvent, error) {
	bucket := fmt.Sprintf("capabilities-%s-%s-%s", namespace, podName, containerID)
	return es.capabilitiesEventDB.GetNClean(bucket), nil
}

func (es *EventSink) GetExecveEvents(namespace string, podName string, containerID string) ([]*tracing.ExecveEvent, error) {
	bucket := fmt.Sprintf("execve-%s-%s-%s", namespace, podName, containerID)
	return es.execvEventDB.GetNClean(bucket), nil
}

func (es *EventSink) GetOpenEvents(namespace string, podName string, containerID string) ([]*tracing.OpenEvent, error) {
	bucket := fmt.Sprintf("open-%s-%s-%s", namespace, podName, containerID)
	return es.openEventDB.GetNClean(bucket), nil
}

func (es *EventSink) SendExecveEvent(event *tracing.ExecveEvent) {
	if !es.filterEvents {
		es.execveEventChannel <- event
		return
	} else {
		// Check that there is a matching filter
		for _, filter := range es.eventFilters {
			if filter.ContainerID == event.ContainerID &&
				(filter.EventType == tracing.AllEventType || filter.EventType == tracing.ExecveEventType) {
				es.execveEventChannel <- event
				return
			}
		}
	}
}

func (es *EventSink) SendOpenEvent(event *tracing.OpenEvent) {
	if !es.filterEvents {
		es.openEventChannel <- event
		return
	} else {
		// Check that there is a matching filter
		for _, filter := range es.eventFilters {
			if filter.ContainerID == event.ContainerID &&
				(filter.EventType == tracing.AllEventType || filter.EventType == tracing.OpenEventType) {
				es.openEventChannel <- event
				return
			}
		}
	}
}

func (es *EventSink) SendCapabilitiesEvent(event *tracing.CapabilitiesEvent) {
	if !es.filterEvents {
		es.capabilitiesEventChannel <- event
		return
	} else {
		// Check that there is a matching filter
		for _, filter := range es.eventFilters {
			if filter.ContainerID == event.ContainerID &&
				(filter.EventType == tracing.AllEventType || filter.EventType == tracing.CapabilitiesEventType) {
				es.capabilitiesEventChannel <- event
				return
			}
		}
	}
}

func (es *EventSink) SendDnsEvent(event *tracing.DnsEvent) {
	if !es.filterEvents {
		es.dnsEventChannel <- event
		return
	} else {
		// Check that there is a matching filter
		for _, filter := range es.eventFilters {
			if filter.ContainerID == event.ContainerID &&
				(filter.EventType == tracing.AllEventType || filter.EventType == tracing.DnsEventType) {
				es.dnsEventChannel <- event
				return
			}
		}
	}
}

func (es *EventSink) SendNetworkEvent(event *tracing.NetworkEvent) {
	if !es.filterEvents {
		es.networkEventChannel <- event
		return
	} else {
		// Check that there is a matching filter
		for _, filter := range es.eventFilters {
			if filter.ContainerID == event.ContainerID &&
				(filter.EventType == tracing.AllEventType || filter.EventType == tracing.NetworkEventType) {
				es.networkEventChannel <- event
				return
			}
		}
	}
}

func (es *EventSink) Close() error {
	es.execvEventDB.Close()
	es.openEventDB.Close()
	es.capabilitiesEventDB.Close()
	es.dnsEventDB.Close()
	es.networkEventDB.Close()

	return nil
}
