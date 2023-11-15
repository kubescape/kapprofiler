package eventsink

import (
	"fmt"
	"os"

	"github.com/kubescape/kapprofiler/pkg/tracing"

	"log"

	nutsdb "github.com/nutsdb/nutsdb"
)

type EventSink struct {
	homeDir                  string
	filterEvents             bool
	fileDB                   *nutsdb.DB
	execveEventChannel       chan *tracing.ExecveEvent
	openEventChannel         chan *tracing.OpenEvent
	capabilitiesEventChannel chan *tracing.CapabilitiesEvent
	dnsEventChannel          chan *tracing.DnsEvent
	networkEventChannel      chan *tracing.NetworkEvent
	eventFilters             []*EventSinkFilter
}

type EventSinkFilter struct {
	EventType   tracing.EventType
	ContainerID string
}

func NewEventSink(homeDir string, filterEvents bool) (*EventSink, error) {
	return &EventSink{homeDir: homeDir, filterEvents: filterEvents}, nil
}

func (es *EventSink) Start() error {
	// Setup nutsdb database
	if es.homeDir == "" {
		// TODO: Use a better default
		es.homeDir = "/tmp"
	}
	
	db, err := nutsdb.Open(
		nutsdb.DefaultOptions,
		nutsdb.WithDir(es.homeDir+"/execve-events.db"),
		nutsdb.WithRWMode(nutsdb.FileIO),
	)
	if err != nil {
		return err
	}
	es.fileDB = db

	// Create the channel for execve events
	es.execveEventChannel = make(chan *tracing.ExecveEvent, 10000)

	// Create the channel for the open events
	es.openEventChannel = make(chan *tracing.OpenEvent, 10000)

	// Create the channel for the capabilities events
	es.capabilitiesEventChannel = make(chan *tracing.CapabilitiesEvent, 10000)

	// Create the channel for the dns events
	es.dnsEventChannel = make(chan *tracing.DnsEvent, 10000)

	// Create the channel for the network events
	es.networkEventChannel = make(chan *tracing.NetworkEvent, 10000)

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

	// Close the nuts database
	err := es.fileDB.Close()
	if err != nil {
		return err
	}

	// Delete nutsdb file
	os.Remove(es.homeDir + "/execve-events.db")

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
		err := es.fileDB.Update(func(tx *nutsdb.Tx) error {
			sEvent, err := event.GobEncode()
			if err != nil {
				log.Printf("error encoding network event: %s\n", err)
				return err
			}
			err = tx.Put(bucket, sEvent, nil, 0)
			if err != nil {
				log.Printf("error storing network event: %s\n", err)
				return err
			}
			return nil
		})
		if err != nil {
			log.Printf("error storing network event: %s\n", err)
		}
	}

	return nil
}

func (es *EventSink) dnsEventWorker() error {
	for event := range es.dnsEventChannel {
		bucket := fmt.Sprintf("dns-%s-%s-%s", event.Namespace, event.PodName, event.ContainerName)
		err := es.fileDB.Update(func(tx *nutsdb.Tx) error {
			sEvent, err := event.GobEncode()
			if err != nil {
				log.Printf("error encoding dns event: %s\n", err)
				return err
			}
			err = tx.Put(bucket, sEvent, nil, 0)
			if err != nil {
				log.Printf("error storing dns event: %s\n", err)
				return err
			}
			return nil
		})
		if err != nil {
			log.Printf("error storing dns event: %s\n", err)
		}
	}

	return nil
}

func (es *EventSink) capabilitiesEventWorker() error {
	for event := range es.capabilitiesEventChannel {
		bucket := fmt.Sprintf("capabilities-%s-%s-%s", event.Namespace, event.PodName, event.ContainerName)
		err := es.fileDB.Update(func(tx *nutsdb.Tx) error {
			sEvent, err := event.GobEncode()
			if err != nil {
				log.Printf("error encoding capabilities event: %s\n", err)
				return err
			}
			err = tx.Put(bucket, sEvent, nil, 0)
			if err != nil {
				log.Printf("error storing capabilities event: %s\n", err)
				return err
			}
			return nil
		})
		if err != nil {
			log.Printf("error storing capabilities event: %s\n", err)
		}
	}

	return nil
}

func (es *EventSink) openEventWorker() error {
	for event := range es.openEventChannel {
		bucket := fmt.Sprintf("open-%s-%s-%s", event.Namespace, event.PodName, event.ContainerName)
		err := es.fileDB.Update(func(tx *nutsdb.Tx) error {
			sEvent, err := event.GobEncode()
			if err != nil {
				log.Printf("error encoding open event: %s\n", err)
				return err
			}
			err = tx.Put(bucket, sEvent, nil, 0)
			if err != nil {
				log.Printf("error storing open event: %s\n", err)
				return err
			}
			return nil
		})
		if err != nil {
			log.Printf("error storing open event: %s\n", err)
		}
	}

	return nil
}

func (es *EventSink) execveEventWorker() error {
	// TODO: Implement this with batch writes

	// Wait for execve events and store them in the database
	for event := range es.execveEventChannel {
		bucket := fmt.Sprintf("execve-%s-%s-%s", event.Namespace, event.PodName, event.ContainerName)
		err := es.fileDB.Update(func(tx *nutsdb.Tx) error {
			sEvent, err := event.GobEncode()
			if err != nil {
				log.Printf("error encoding execve event: %s\n", err)
				return err
			}
			err = tx.Put(bucket, sEvent, nil, 0)
			if err != nil {
				log.Printf("error storing execve event: %s\n", err)
				return err
			}
			return nil
		})
		if err != nil {
			log.Printf("error storing execve event: %s\n", err)
		}
	}

	return nil
}

func (es *EventSink) CleanupContainer(namespace string, podName string, containerID string) error {
	bucket := fmt.Sprintf("execve-%s-%s-%s", namespace, podName, containerID)
	if err := es.fileDB.Update(
		func(tx *nutsdb.Tx) error {
			return tx.DeleteBucket(nutsdb.DataStructureBTree,bucket)
		}); err != nil {
		log.Printf("error deleting bucket: %s\n", err)
		}


	bucket = fmt.Sprintf("open-%s-%s-%s", namespace, podName, containerID)
	if err := es.fileDB.Update(
		func(tx *nutsdb.Tx) error {
			return tx.DeleteBucket(nutsdb.DataStructureBTree,bucket)
		}); err != nil {
		log.Printf("error deleting bucket: %s\n", err)
		}

	bucket = fmt.Sprintf("capabilities-%s-%s-%s", namespace, podName, containerID)
	if err := es.fileDB.Update(
		func(tx *nutsdb.Tx) error {
			return tx.DeleteBucket(nutsdb.DataStructureBTree,bucket)
		}); err != nil {
		log.Printf("error deleting bucket: %s\n", err)
		}

	bucket = fmt.Sprintf("dns-%s-%s-%s", namespace, podName, containerID)
	if err := es.fileDB.Update(
		func(tx *nutsdb.Tx) error {
			return tx.DeleteBucket(nutsdb.DataStructureBTree,bucket)
		}); err != nil {
		log.Printf("error deleting bucket: %s\n", err)
		}

	bucket = fmt.Sprintf("network-%s-%s-%s", namespace, podName, containerID)
	if err := es.fileDB.Update(
		func(tx *nutsdb.Tx) error {
			return tx.DeleteBucket(nutsdb.DataStructureBTree,bucket)
		}); err != nil {
		log.Printf("error deleting bucket: %s\n", err)
		}
	return nil
	}

func (es *EventSink) GetNetworkEvents(namespace string, podName string, containerID string) ([]*tracing.NetworkEvent, error) {
	bucket := fmt.Sprintf("network-%s-%s-%s", namespace, podName, containerID)
	var events []*tracing.NetworkEvent
	es.fileDB.View(func(tx *nutsdb.Tx) error {
		entries, err := tx.GetAll(bucket)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			event := &tracing.NetworkEvent{}
			err := event.GobDecode(entry.Key)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		return nil
	})

	return events, nil
}

func (es *EventSink) GetDnsEvents(namespace string, podName string, containerID string) ([]*tracing.DnsEvent, error) {
	bucket := fmt.Sprintf("dns-%s-%s-%s", namespace, podName, containerID)
	var events []*tracing.DnsEvent
	es.fileDB.View(func(tx *nutsdb.Tx) error {
		entries, err := tx.GetAll(bucket)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			event := &tracing.DnsEvent{}
			err := event.GobDecode(entry.Key)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		return nil
	})

	return events, nil
}

func (es *EventSink) GetCapabilitiesEvents(namespace string, podName string, containerID string) ([]*tracing.CapabilitiesEvent, error) {
	bucket := fmt.Sprintf("capabilities-%s-%s-%s", namespace, podName, containerID)
	var events []*tracing.CapabilitiesEvent
	es.fileDB.View(func(tx *nutsdb.Tx) error {
		entries, err := tx.GetAll(bucket)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			event := &tracing.CapabilitiesEvent{}
			err := event.GobDecode(entry.Key)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		return nil
	})

	return events, nil
}

func (es *EventSink) GetExecveEvents(namespace string, podName string, containerID string) ([]*tracing.ExecveEvent, error) {
	bucket := fmt.Sprintf("execve-%s-%s-%s", namespace, podName, containerID)
	var events []*tracing.ExecveEvent
	es.fileDB.View(func(tx *nutsdb.Tx) error {
		entries, err := tx.GetAll(bucket)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			event := &tracing.ExecveEvent{}
			err := event.GobDecode(entry.Key)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		return nil
	})

	return events, nil
}

func (es *EventSink) GetOpenEvents(namespace string, podName string, containerID string) ([]*tracing.OpenEvent, error) {
	bucket := fmt.Sprintf("open-%s-%s-%s", namespace, podName, containerID)
	var events []*tracing.OpenEvent
	es.fileDB.View(func(tx *nutsdb.Tx) error {
		entries, err := tx.GetAll(bucket)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			event := &tracing.OpenEvent{}
			err := event.GobDecode(entry.Key)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		return nil
	})

	return events, nil
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
	return es.fileDB.Close()
}
