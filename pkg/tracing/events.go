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
	// Low level container information
	ContainerID string
	NsMntId     uint64
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

type TcpEvent struct {
	ContainerID string
	PodName     string
	Namespace   string
	Source      string
	SourcePort  int
	Destination string
	DestPort    int
	Operation   string
	IpvType     string
	Timestamp   int64
}

type OpenEvent struct {
	ContainerID string
	PodName     string
	Namespace   string
	TaskName    string
	TaskId      int
	PathName    string
	Flags       []string
	Timestamp   int64
}

type CapabilitiesEvent struct {
	ContainerID    string
	PodName        string
	Namespace      string
	Syscall        string
	CapabilityName string
	Timestamp      int64
}

type DnsEvent struct {
	ContainerID string
	PodName     string
	Namespace   string
	DnsName     string
	Addresses   []string
	Type        string
	Timestamp   int64
}

type EventSink interface {
	// SendExecveEvent sends an execve event to the sink
	SendExecveEvent(event *ExecveEvent)
	// SendTcpEvent sends a TCP event to the sink
	SendTcpEvent(event *TcpEvent)
	// SendOpenEvent sends a OPEN event to the sink
	SendOpenEvent(event *OpenEvent)
	// SendCapabilitiesEvent sends a Capabilities event to the sink
	SendCapabilitiesEvent(event *CapabilitiesEvent)
	// SendDnsEvent sends a Dns event to the sink
	SendDnsEvent(event *DnsEvent)
}

// Encode/Decode functions for CapabilitiesEvent
func (event *DnsEvent) GobEncode() ([]byte, error) {
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
	if err := encoder.Encode(event.DnsName); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Addresses); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Type); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Timestamp); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (event *DnsEvent) GobDecode(buf []byte) error {
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
	if err := decoder.Decode(&event.DnsName); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Addresses); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Type); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Timestamp); err != nil {
		return err
	}
	return nil
}

// Encode/Decode functions for CapabilitiesEvent
func (event *CapabilitiesEvent) GobEncode() ([]byte, error) {
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
	if err := encoder.Encode(event.Syscall); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.CapabilityName); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Timestamp); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (event *CapabilitiesEvent) GobDecode(buf []byte) error {
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
	if err := decoder.Decode(&event.Syscall); err != nil {
		return err
	}
	if err := decoder.Decode(&event.CapabilityName); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Timestamp); err != nil {
		return err
	}
	return nil
}

// Encode/Decode functions for OpenEvent
func (event *OpenEvent) GobEncode() ([]byte, error) {
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
	if err := encoder.Encode(event.TaskName); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.TaskId); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Flags); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Timestamp); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (event *OpenEvent) GobDecode(buf []byte) error {
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
	if err := decoder.Decode(&event.TaskName); err != nil {
		return err
	}
	if err := decoder.Decode(&event.TaskId); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Flags); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Timestamp); err != nil {
		return err
	}
	return nil
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

// Encode/Decode functions for ExecveEvent
func (event *TcpEvent) GobEncode() ([]byte, error) {
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
	if err := encoder.Encode(event.Operation); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Source); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.SourcePort); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Destination); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.DestPort); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (event *TcpEvent) GobDecode(buf []byte) error {
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
	if err := decoder.Decode(&event.Operation); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Source); err != nil {
		return err
	}
	if err := decoder.Decode(&event.SourcePort); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Destination); err != nil {
		return err
	}
	if err := decoder.Decode(&event.DestPort); err != nil {
		return err
	}
	return nil
}
