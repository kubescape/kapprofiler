package tracing

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

type ProcessDetails struct {
	Pid  uint32 `json:"pid"`
	Ppid uint32 `json:"ppid"`
	Comm string `json:"comm"`
	Cwd  string `json:"cwd"`
	Uid  uint32 `json:"uid"`
	Gid  uint32 `json:"gid"`
}

type GeneralEvent struct {
	ProcessDetails
	ContainerName string    `json:"container_name"`
	ContainerID   string    `json:"container_id"`
	PodName       string    `json:"pod_name"`
	Namespace     string    `json:"namespace"`
	MountNsID     uint64    `json:"mount_ns_id"`
	Timestamp     int64     `json:"timestamp"`
	EventType     EventType `json:"event_type"`
}

type ExecveEvent struct {
	GeneralEvent

	PathName string   `json:"path_name"`
	Args     []string `json:"args"`
	Env      []string `json:"env"`
}

type OpenEvent struct {
	GeneralEvent

	TaskName string   `json:"task_name"`
	TaskId   uint32   `json:"task_id"`
	PathName string   `json:"path_name"`
	Flags    []string `json:"flags"`
}

type CapabilitiesEvent struct {
	GeneralEvent

	Syscall        string `json:"syscall"`
	CapabilityName string `json:"capability_name"`
}

type DnsEvent struct {
	GeneralEvent

	DnsName   string   `json:"dns_name"`
	Addresses []string `json:"addresses"`
}

type NetworkEvent struct {
	GeneralEvent

	PacketType  string `json:"packet_type"`
	Protocol    string `json:"protocol"`
	Port        uint16 `json:"port"`
	DstEndpoint string `json:"dst_endpoint"`
}

type SyscallEvent struct {
	GeneralEvent

	Syscalls []string `json:"syscalls"`
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
