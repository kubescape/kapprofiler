package tracing

import (
	"fmt"
	"os"

	runtimeclient "github.com/inspektor-gadget/inspektor-gadget/pkg/container-utils/runtime-client"
	containerutilsTypes "github.com/inspektor-gadget/inspektor-gadget/pkg/container-utils/types"
	igtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
)

func DetectContainerRuntime(hostMount string) (*containerutilsTypes.RuntimeConfig, error) {
	runtimes := map[igtypes.RuntimeName]string{
		igtypes.RuntimeNameDocker:     runtimeclient.DockerDefaultSocketPath,
		igtypes.RuntimeNameCrio:       runtimeclient.CrioDefaultSocketPath,
		igtypes.RuntimeNameContainerd: runtimeclient.ContainerdDefaultSocketPath,
		igtypes.RuntimeNamePodman:     runtimeclient.PodmanDefaultSocketPath,
	}
	for runtimeName, socketPath := range runtimes {
		// Check if the socket is available on the host mount
		socketPath = hostMount + socketPath
		if _, err := os.Stat(socketPath); err == nil {
			return &containerutilsTypes.RuntimeConfig{
				Name:       runtimeName,
				SocketPath: socketPath,
			}, nil
		}
	}
	return nil, fmt.Errorf("no container runtime detected at the following paths: %v", runtimes)
}
