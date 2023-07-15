package tracing

type ExecveEvent struct {
	ContainerID string
	PodName     string
	Namespace   string
	PathName    string
	Args        []string
	Env         []string
}

type EventSink interface {
	// SendExecveEvent sends an execve event to the sink
	SendExecveEvent(event *ExecveEvent)
}
