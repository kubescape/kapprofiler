package eventsink

import (
	"fmt"
	"kapprofiler/pkg/tracing"
	"os"

	bolt "go.etcd.io/bbolt"
)

type EventSink struct {
	homeDir            string
	fileDB             *bolt.DB
	execveEventChannel chan *tracing.ExecveEvent
}

func NewEventSink(homeDir string) (*EventSink, error) {
	return &EventSink{homeDir: homeDir}, nil
}

func (es *EventSink) Start() error {
	// Setup bolt database
	if es.homeDir == "" {
		// TODO: Use a better default
		es.homeDir = "/tmp"
	}
	db, err := bolt.Open(es.homeDir+"/execve-events.db", 0600, nil)
	if err != nil {
		return err
	}
	es.fileDB = db

	// Create the channel for execve events
	es.execveEventChannel = make(chan *tracing.ExecveEvent, 10000)

	// Start the execve event worker
	go es.execveEventWorker()

	return nil
}

func (es *EventSink) Stop() error {
	// Close the channel for execve events
	close(es.execveEventChannel)

	// Close the bolt database
	err := es.fileDB.Close()
	if err != nil {
		return err
	}

	// Delete boltdb file
	os.Remove(es.homeDir + "/execve-events.db")

	return nil
}

func (es *EventSink) execveEventWorker() error {
	// TODO: Implement this with batch writes

	// Wait for execve events and store them in the database
	for event := range es.execveEventChannel {
		bucket := fmt.Sprintf("execve-%s-%s-%s", event.Namespace, event.PodName, event.ContainerID)
		es.fileDB.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				// TODO: Report error
				return err
			}
			sEvent, err := event.GobEncode()
			if err != nil {
				// TODO: Report error
				return err
			}
			err = b.Put(sEvent, nil)
			if err != nil {
				// TODO: Report error
				return err
			}
			return nil
		})
	}

	return nil
}

func (es *EventSink) CleanupContainer(namespace string, podName string, containerID string) error {
	bucket := fmt.Sprintf("execve-%s-%s-%s", namespace, podName, containerID)
	err := es.fileDB.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(bucket))
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

func (es *EventSink) GetExecveEvents(namespace string, podName string, containerID string) ([]*tracing.ExecveEvent, error) {
	bucket := fmt.Sprintf("execve-%s-%s-%s", namespace, podName, containerID)
	var events []*tracing.ExecveEvent
	err := es.fileDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			event := &tracing.ExecveEvent{}
			err := event.GobDecode(k)
			if err != nil {
				return err
			}
			events = append(events, event)
			return nil
		})
		return nil
	})
	return events, err
}

func (es *EventSink) SendExecveEvent(event *tracing.ExecveEvent) {
	es.execveEventChannel <- event
}

func (es *EventSink) Close() error {
	return es.fileDB.Close()
}
