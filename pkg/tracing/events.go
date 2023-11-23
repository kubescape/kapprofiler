package tracing

import (
	"bytes"
	"encoding/gob"
)

const (
	ContainerActivityEventStart    = "start"
	ContainerActivityEventAttached = "attached"
	ContainerActivityEventStop     = "stop"
)

type EventType int

const (
	ExecveEventType EventType = iota
	OpenEventType
	CapabilitiesEventType
	DnsEventType
	NetworkEventType
	SyscallEventType
	AllEventType
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
	Pid         uint32
}

type GeneralEvent struct {
	ContainerName string
	ContainerID   string
	PodName       string
	Namespace     string
	Timestamp     int64
	EventType     EventType
}

type ExecveEvent struct {
	GeneralEvent

	PathName string
	Args     []string
	Env      []string
}

type OpenEvent struct {
	GeneralEvent

	TaskName string
	TaskId   int
	PathName string
	Flags    []string
}

type CapabilitiesEvent struct {
	GeneralEvent

	Syscall        string
	CapabilityName string
}

type DnsEvent struct {
	GeneralEvent

	DnsName   string
	Addresses []string
}

type NetworkEvent struct {
	GeneralEvent

	PacketType  string
	Protocol    string
	Port        uint16
	DstEndpoint string
}

type SyscallEvent struct {
	GeneralEvent

	Syscalls []string
}

type EventSink interface {
	// SendExecveEvent sends an execve event to the sink
	SendExecveEvent(event *ExecveEvent)
	// SendOpenEvent sends a OPEN event to the sink
	SendOpenEvent(event *OpenEvent)
	// SendCapabilitiesEvent sends a Capabilities event to the sink
	SendCapabilitiesEvent(event *CapabilitiesEvent)
	// SendDnsEvent sends a Dns event to the sink
	SendDnsEvent(event *DnsEvent)
	// SendNetworkEvent sends a Network event to the sink
	SendNetworkEvent(event *NetworkEvent)
}

// Encode/Decode functions for NetowrkEvent
func (event *NetworkEvent) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	if err := encoder.Encode(event.ContainerName); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.ContainerID); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.PodName); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Namespace); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.PacketType); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Protocol); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Port); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.DstEndpoint); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Timestamp); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (event *NetworkEvent) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	if err := decoder.Decode(&event.ContainerName); err != nil {
		return err
	}
	if err := decoder.Decode(&event.ContainerID); err != nil {
		return err
	}
	if err := decoder.Decode(&event.PodName); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Namespace); err != nil {
		return err
	}
	if err := decoder.Decode(&event.PacketType); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Protocol); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Port); err != nil {
		return err
	}
	if err := decoder.Decode(&event.DstEndpoint); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Timestamp); err != nil {
		return err
	}
	return nil
}

// Encode/Decode functions for DnsEvent
func (event *DnsEvent) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	if err := encoder.Encode(event.ContainerName); err != nil {
		return nil, err
	}
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
	if err := encoder.Encode(event.Timestamp); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (event *DnsEvent) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	if err := decoder.Decode(&event.ContainerName); err != nil {
		return err
	}
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
	if err := decoder.Decode(&event.Timestamp); err != nil {
		return err
	}
	return nil
}

// Encode/Decode functions for CapabilitiesEvent
func (event *CapabilitiesEvent) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	if err := encoder.Encode(event.ContainerName); err != nil {
		return nil, err
	}
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
	if err := decoder.Decode(&event.ContainerName); err != nil {
		return err
	}
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
	if err := encoder.Encode(event.ContainerName); err != nil {
		return nil, err
	}
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
	if err := decoder.Decode(&event.ContainerName); err != nil {
		return err
	}
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
	if err := encoder.Encode(event.ContainerName); err != nil {
		return nil, err
	}
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
	if err := decoder.Decode(&event.ContainerName); err != nil {
		return err
	}
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

// Encode/Decode functions for SyscallEvent
func (event *SyscallEvent) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	if err := encoder.Encode(event.ContainerName); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.ContainerID); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.PodName); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Namespace); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Syscalls); err != nil {
		return nil, err
	}
	if err := encoder.Encode(event.Timestamp); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (event *SyscallEvent) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	if err := decoder.Decode(&event.ContainerName); err != nil {
		return err
	}
	if err := decoder.Decode(&event.ContainerID); err != nil {
		return err
	}
	if err := decoder.Decode(&event.PodName); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Namespace); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Syscalls); err != nil {
		return err
	}
	if err := decoder.Decode(&event.Timestamp); err != nil {
		return err
	}
	return nil
}
