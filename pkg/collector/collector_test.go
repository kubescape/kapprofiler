package collector_test

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/kubescape/kapprofiler/pkg/collector"
	"github.com/kubescape/kapprofiler/pkg/eventsink"
	"github.com/kubescape/kapprofiler/pkg/tracing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	openapi3gen "github.com/getkin/kin-openapi/openapi3gen"
	yaml "gopkg.in/yaml.v2"
)

type TestTracer struct {
}

func (t *TestTracer) Start() error {
	return nil
}

func (t *TestTracer) Stop() error {
	return nil
}

func (t *TestTracer) AddContainerActivityListener(listener tracing.ContainerActivityEventListener) {
}

func (t *TestTracer) RemoveContainerActivityListener(listener tracing.ContainerActivityEventListener) {
}

func (t *TestTracer) PeekSyscallInContainer(nsMountId uint64) ([]string, error) {
	return []string{"open", "close"}, nil
}

func (t *TestTracer) AddEventSink(sink tracing.EventSink) {
}

func (t *TestTracer) RemoveEventSink(sink tracing.EventSink) {
}

func (t *TestTracer) StartTraceContainer(mntns uint64, pid uint32, eventType tracing.EventType) error {
	return nil
}

func (t *TestTracer) StopTraceContainer(mntns uint64, pid uint32, eventType tracing.EventType) error {
	return nil
}

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
	eventSink, err := eventsink.NewEventSink("", false)
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
		Tracer:    &TestTracer{},
	})
	if err != nil {
		t.Fatalf("error starting collector manager: %s\n", err)
	}
	defer cm.StopCollectorManager()

	// Exercise
	containedID := &collector.ContainerId{
		Namespace:   "default",
		PodName:     "nginx",
		Container:   "app",
		ContainerID: "1234567890",
	}

	// Start container
	cm.ContainerStarted(containedID, false)

	// Send execve event
	eventSink.SendExecveEvent(&tracing.ExecveEvent{
		GeneralEvent: tracing.GeneralEvent{
			ProcessDetails: tracing.ProcessDetails{
				Pid: 0,
			},
			ContainerID:   containedID.ContainerID,
			PodName:       containedID.PodName,
			Namespace:     containedID.Namespace,
			ContainerName: containedID.Container,
			MountNsID:     containedID.NsMntId,
			Timestamp:     0,
		},
		PathName: "/bin/bash",
		Args:     []string{"-c", "echo", "HapoelForever"},
		Env:      []string{},
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
	defer deleteContainerProfile(k8sConfig, containedID.Namespace, containedID.PodName)

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
		return
	}

	// Verify container ID
	if appProfile.Spec.Containers[0].Name != containedID.Container {
		t.Errorf("expected container ID %s, got %s\n", containedID.Container, appProfile.Spec.Containers[0].Name)
		return
	}

	// Verify length execve events
	if len(appProfile.Spec.Containers[0].Execs) != 1 {
		t.Errorf("expected 1 execve event, got %d\n", len(appProfile.Spec.Containers[0].Execs))
		return
	}

	// Verify execve event
	if appProfile.Spec.Containers[0].Execs[0].Path != "/bin/bash" {
		t.Errorf("expected path name test, got %s\n", appProfile.Spec.Containers[0].Execs[0].Path)
	}
}

func TestCollectorWithContainerProfileUpdates(t *testing.T) {
	// Setup
	SetupCrdInCluster()

	// Get Kubernetes config
	k8sConfig, err := GetKubernetesConfig()
	if err != nil {
		t.Errorf("error getting Kubernetes config: %s\n", err)
	}

	// Create an event sink
	eventSink, err := eventsink.NewEventSink("", false)
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
		Interval:  2,
		K8sConfig: k8sConfig,
		EventSink: eventSink,
		Tracer:    &TestTracer{},
		NodeName:  "minikube",
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
	cm.ContainerStarted(containedID, false)

	// Send execve event
	eventSink.SendExecveEvent(&tracing.ExecveEvent{
		GeneralEvent: tracing.GeneralEvent{
			ProcessDetails: tracing.ProcessDetails{
				Pid: 0,
			},
			ContainerID:   containedID.Container,
			ContainerName: containedID.Container,
			PodName:       containedID.PodName,
			Namespace:     containedID.Namespace,
			MountNsID:     containedID.NsMntId,
			Timestamp:     0,
		},
		PathName: "/bin/bash",
		Args:     []string{"-c", "echo", "HapoelForever"},
		Env:      []string{},
	})

	// Sleep for the interval time + 1 second
	time.Sleep(3 * time.Second)

	// Check if the container profile was created
	appProfile, err := getContainerProfile(k8sConfig, containedID.Namespace, containedID.PodName)
	if err != nil {
		t.Fatalf("error getting container profile: %s\n", err)
	}
	defer deleteContainerProfile(k8sConfig, containedID.Namespace, containedID.PodName)

	// Verify length containers
	if len(appProfile.Spec.Containers) != 1 {
		t.Errorf("expected 1 container, got %d\n", len(appProfile.Spec.Containers))
	}

	// Verify execve event
	if appProfile.Spec.Containers[0].Execs[0].Path != "/bin/bash" {
		t.Errorf("expected path name test, got %s\n", appProfile.Spec.Containers[0].Execs[0].Path)
	}

	// Let the collector update the container profile
	time.Sleep(3 * time.Second)

	// Check if the container profile was updated
	appProfile, err = getContainerProfile(k8sConfig, containedID.Namespace, containedID.PodName)
	if err != nil {
		t.Fatalf("error getting container profile: %s\n", err)
	}

	// Verify length containers
	if len(appProfile.Spec.Containers) != 1 {
		t.Errorf("expected 1 container, got %d\n", len(appProfile.Spec.Containers))
	}

	// Stop container
	cm.ContainerStopped(containedID)
}

func deleteContainerProfile(k8sConfig *rest.Config, namespace string, pod string) error {
	dynamicClient, err := dynamic.NewForConfig(k8sConfig)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("pod-%s", pod)
	err = dynamicClient.Resource(collector.AppProfileGvr).Namespace(namespace).Delete(context.Background(), name, v1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func getContainerProfile(k8sConfig *rest.Config, namespace string, pod string) (*collector.ApplicationProfile, error) {
	dynamicClient, err := dynamic.NewForConfig(k8sConfig)
	if err != nil {
		return nil, err
	}

	// Get app profile CRD
	appProfileListRaw, err := dynamicClient.Resource(collector.AppProfileGvr).Namespace(namespace).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("pod-%s", pod)

	// Verify that the app profile was stored
	if len(appProfileListRaw.Items) < 1 {
		return nil, fmt.Errorf("Got a list of empty app profiles")
	}

	for _, appProfileRaw := range appProfileListRaw.Items {
		if appProfileRaw.GetName() == name {
			appProfile := &collector.ApplicationProfile{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(appProfileRaw.Object, appProfile)
			if err != nil {
				return nil, err
			}
			return appProfile, nil
		}
	}

	return nil, fmt.Errorf("App profile %s not found", name)
}

// Test that a openapispec can be generated from the structs
func TestOpenApiSpec(t *testing.T) {
	schemaRef, err := openapi3gen.NewSchemaRefForValue(&collector.ApplicationProfileSpec{}, nil)
	if err != nil {
		t.Fatalf("error generating openapi spec: %s\n", err)
	}
	_, err = yaml.Marshal(schemaRef)
	if err != nil {
		t.Fatalf("error marshaling openapi spec: %s\n", err)
	}
	//log.Printf("%s\n", yamlData)
}
