package collector_test

import (
	"context"
	"kapprofiler/pkg/collector"
	"kapprofiler/pkg/eventsink"
	"kapprofiler/pkg/tracing"
	"os/exec"
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetKubernetesConfig() (*rest.Config, error) {
	// Check if the Kubernetes cluster is reachable
	// Load the Kubernetes configuration from the default location
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	return config, nil
}

func SetupCrdInCluster() error {
	// Run external process "kubectl apply -f etc/app-profile.crd.yaml"
	// to create the CRD in the cluster
	exec.Command("kubectl", "apply", "-f", "etc/app-profile.crd.yaml").Run()
	return nil
}

func TestCollectorBasic(t *testing.T) {
	// Setup
	SetupCrdInCluster()

	// Get Kubernetes config
	k8sConfig, err := GetKubernetesConfig()
	if err != nil {
		t.Errorf("error getting Kubernetes config: %s\n", err)
	}

	// Create an event sink
	eventSink, err := eventsink.NewEventSink("")
	if err != nil {
		t.Errorf("error creating event sink: %s\n", err)
	}

	// Start event sink
	err = eventSink.Start()
	if err != nil {
		t.Fatalf("error starting event sink: %s\n", err)
	}
	defer eventSink.Stop()

	// Start collector manager
	cm, err := collector.StartCollectorManager(&collector.CollectorManagerConfig{
		Interval:  10,
		K8sConfig: k8sConfig,
		EventSink: eventSink,
	})
	if err != nil {
		t.Fatalf("error starting collector manager: %s\n", err)
	}
	defer cm.StopCollectorManager()

	// Exercise
	containedID := &collector.ContainerId{
		Namespace: "default",
		PodName:   "nginx",
		Container: "app",
	}

	// Start container
	cm.ContainerStarted(containedID)

	// Send execve event
	eventSink.SendExecveEvent(&tracing.ExecveEvent{
		ContainerID: containedID.Container,
		PodName:     containedID.PodName,
		Namespace:   containedID.Namespace,
		PathName:    "/bin/bash",
		Args:        []string{"-c", "echo", "HapoelForever"},
		Env:         []string{},
		Timestamp:   0,
	})

	// Let the event sink process the event
	time.Sleep(1 * time.Second)

	// Stop container
	cm.ContainerStopped(containedID)

	// Sleep for a 1 second to allow the object to be stored
	time.Sleep(1 * time.Second)

	// Verify
	dynamicClient, err := dynamic.NewForConfig(k8sConfig)
	if err != nil {
		t.Fatalf("error creating dynamic client: %s\n", err)
	}

	// Get app profile CRD
	appProfileListRaw, err := dynamicClient.Resource(collector.AppProfileGvr).Namespace(containedID.Namespace).List(context.Background(), v1.ListOptions{})
	if err != nil {
		t.Fatalf("error getting app profile list: %s\n", err)
	}

	// Verify that the app profile was stored
	if len(appProfileListRaw.Items) != 1 {
		t.Fatalf("expected 1 app profile, got %d\n", len(appProfileListRaw.Items))
	}

	// Get app profile
	appProfileRaw, err := dynamicClient.Resource(collector.AppProfileGvr).Namespace(containedID.Namespace).Get(context.Background(), appProfileListRaw.Items[0].GetName(), v1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting app profile: %s\n", err)
	}

	// Verify that the app profile was stored
	appProfile := &collector.ApplicationProfile{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(appProfileRaw.Object, appProfile)
	if err != nil {
		t.Errorf("error converting app profile: %s\n", err)
	}
	defer dynamicClient.Resource(collector.AppProfileGvr).Namespace(containedID.Namespace).Delete(context.Background(), appProfileListRaw.Items[0].GetName(), v1.DeleteOptions{})

	// Verify length containers
	if len(appProfile.Spec.Containers) != 1 {
		t.Errorf("expected 1 container, got %d\n", len(appProfile.Spec.Containers))
	}

	// Verify container ID
	if appProfile.Spec.Containers[0].Name != containedID.Container {
		t.Errorf("expected container ID %s, got %s\n", containedID.Container, appProfile.Spec.Containers[0].Name)
	}

	// Verify length execve events
	if len(appProfile.Spec.Containers[0].Execs) != 1 {
		t.Errorf("expected 1 execve event, got %d\n", len(appProfile.Spec.Containers[0].Execs))
	}

	// Verify execve event
	if appProfile.Spec.Containers[0].Execs[0].Path != "/bin/bash" {
		t.Errorf("expected path name test, got %s\n", appProfile.Spec.Containers[0].Execs[0].Path)
	}

}
