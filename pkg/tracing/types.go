package tracing

import "github.com/cilium/ebpf"

type GadgetTracerCommon interface {
	Stop()
}

type AtachableTracer interface {
	Attach(pid uint32) error
	Detach(pid uint32) error
	Close()
}

type PeekableTracer interface {
	Peek(nsMountId uint64) ([]string, error)
	Close()
}

type TracingState struct {
	usageReferenceCount    map[uint64]int
	eBpfContainerFilterMap *ebpf.Map
	gadget                 GadgetTracerCommon
	attachable             AtachableTracer
	peekable               PeekableTracer
}
