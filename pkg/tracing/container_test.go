package tracing

import (
	"testing"

	igtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
)

func TestContainerRuntimeDetection(t *testing.T) {
	runtime, err := DetectContainerRuntime("")
	if err != nil {
		t.Errorf("error detecting container runtime: %s\n", err)
	}
	if runtime.Name != igtypes.RuntimeNameDocker {
		t.Errorf("expected runtime name %s, got %s\n", igtypes.RuntimeNameDocker, runtime.Name)
	}
}
