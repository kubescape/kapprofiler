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
	RandomXEventType
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
	Pid  uint32
	Ppid uint32
	Comm string
	Cwd  string
	Uid  uint32
	Gid  uint32
}

type GeneralEvent struct {
	ProcessDetails
	ContainerName string
	ContainerID   string
	PodName       string
	Namespace     string
	MountNsID     uint64
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
	TaskId   uint32
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

type RandomXEvent struct {
	GeneralEvent
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
	// SendRandomXEvent sends a RandomX event to the sink
	SendRandomXEvent(event *RandomXEvent)
	// ReportError reports an error to the sink
	ReportError(eventType EventType, err error)
}
