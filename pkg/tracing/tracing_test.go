package tracing_test

import (
	"kapprofiler/pkg/tracing"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
)

func TestStartTrace(t *testing.T) {
	// Setup
	// Depending on the implementation, you may have to setup some state here

	// Get Kubernetes configuration
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		t.Errorf("error getting Kubernetes configuration: %s\n", err)
	}

	// Exercise
	testTracer := tracing.NewTracer("test", config, nil, nil)

	err = testTracer.Start()
	if err != nil {
		t.Errorf("error starting tracer: %s\n", err)
	}

	defer testTracer.Stop()
}
