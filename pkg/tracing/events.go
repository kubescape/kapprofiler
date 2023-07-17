package tracing

import (
	"bytes"
	"encoding/gob"
)

const (
	ContainerActivityEventStart = "start"
	ContainerActivityEventStop  = "stop"
)

type ContainerActivityEventListener interface {
	// OnContainerActivityEvent is called when a container activity event is received
	OnContainerActivityEvent(event *ContainerActivityEvent)
}

type ContainerActivityEvent struct {
	ContainerName string
	PodName       string
	Namespace     string
	Activity      string
}

type ExecveEvent struct {
	ContainerID string
	PodName     string
	Namespace   string
	PathName    string
	Args        []string
	Env         []string
	Timestamp   int64
}

type EventSink interface {
	// SendExecveEvent sends an execve event to the sink
	SendExecveEvent(event *ExecveEvent)
}

// Encode/Decode functions for ExecveEvent
func (event *ExecveEvent) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	if err := encoder.Encode(event.ContainerID); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.PodName); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Namespace); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.PathName); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Args); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Env); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Timestamp); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (event *ExecveEvent) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	if err := decoder.Decode(&event.ContainerID); err != nil {
		return err
	}
	if err := decoder.Decode(&event.PodName); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Namespace); err != nil {
		return err
	}
	if err := decoder.Decode(&event.PathName); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Args); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Env); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Timestamp); err != nil {
		return err
	}
	return nil
}
