package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cilium/ebpf/rlimit"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubescape/kapprofiler/pkg/collector"
	"github.com/kubescape/kapprofiler/pkg/controller"
	"github.com/kubescape/kapprofiler/pkg/eventsink"
	"github.com/kubescape/kapprofiler/pkg/tracing"
)

// Global variables
var NodeName string
var k8sConfig *rest.Config

func checkKubernetesConnection() (*rest.Config, error) {
	// Check if the Kubernetes cluster is reachable
	// Load the Kubernetes configuration from the default location
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	// Create a Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Failed to create Kubernetes client: %v\n", err)
		return nil, err
	}

	// Send a request to the API server to check if it's reachable
	_, err = clientset.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Failed to communicate with Kubernetes API server: %v\n", err)
		return nil, err
	}

	return config, nil
}

func serviceInitNChecks() error {
	// Raise the rlimit for memlock to the maximum allowed (eBPF needs it)
	if err := rlimit.RemoveMemlock(); err != nil {
		return err
	}

	// Check Kubernetes cluster connection
	config, err := checkKubernetesConnection()
	if err != nil {
		return err
	}
	k8sConfig = config

	// Get Node name from environment variable
	if nodeName := os.Getenv("NODE_NAME"); nodeName == "" {
		return fmt.Errorf("NODE_NAME environment variable not set")
	} else {
		NodeName = nodeName
	}

	return nil
}

func main() {
	// Initialize the service
	if err := serviceInitNChecks(); err != nil {
		log.Fatalf("Failed to initialize service: %v\n", err)
	}

	// Create the event sink
	eventSink, err := eventsink.NewEventSink("", true)
	if err != nil {
		log.Fatalf("Failed to create event sink: %v\n", err)
	}

	// Start the event sink
	if err := eventSink.Start(); err != nil {
		log.Fatalf("Failed to start event sink: %v\n", err)
	}
	defer eventSink.Stop()

	// Create the tracer
	tracer := tracing.NewTracer(NodeName, k8sConfig, []tracing.EventSink{eventSink}, false)

	// Start the collector manager
	collectorManagerConfig := &collector.CollectorManagerConfig{
		EventSink:      eventSink,
		Tracer:         tracer,
		Interval:       60, // 60 seconds for now, TODO: make it configurable
		FinalizeTime:   0,  // 0 seconds to disable finalization
		K8sConfig:      k8sConfig,
		RecordStrategy: collector.RecordStrategyOnlyIfNotExists,
		NodeName:       NodeName,
	}
	cm, err := collector.StartCollectorManager(collectorManagerConfig)
	if err != nil {
		log.Fatalf("Failed to start collector manager: %v\n", err)
	}
	defer cm.StopCollectorManager()

	// Start the service
	if err := tracer.Start(); err != nil {
		log.Fatalf("Failed to start service: %v\n", err)
	}
	defer tracer.Stop()

	// Start AppProfile controller
	appProfileController := controller.NewController(k8sConfig)
	appProfileController.StartController()
	defer appProfileController.StopController()

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	<-shutdown
	log.Println("Shutting down...")

	// Exit with success
	os.Exit(0)
}
