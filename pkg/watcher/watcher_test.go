package watcher

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestWatcherBasic(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "testgroup", Version: "v1", Resource: "testresources"}

	testResource := &unstructured.Unstructured{}
	testResource.SetName("testname")
	testResource.SetGroupVersionKind(gvr.GroupVersion().WithKind("testkind"))

	fakeclient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		gvr: "TestResourceList",
	})
	watcher := NewWatcher(fakeclient, false)

	_, err := fakeclient.Resource(gvr).Create(context.Background(), testResource, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("Create failed: %v", err)
	}

	numberOfAddFuncCalls := 0
	numberOfUpdateFuncCalls := 0
	numberOfDeleteFuncCalls := 0

	watcher.Start(WatchNotifyFunctions{
		AddFunc: func(obj *unstructured.Unstructured) {
			numberOfAddFuncCalls++
			t.Logf("AddFunc called with %v", obj)
		},
		UpdateFunc: func(obj *unstructured.Unstructured) {
			numberOfUpdateFuncCalls++
			t.Logf("UpdateFunc called with %v", obj)
		},
		DeleteFunc: func(obj *unstructured.Unstructured) {
			numberOfDeleteFuncCalls++
			t.Logf("DeleteFunc called with %v", obj)
		},
	}, gvr, metav1.ListOptions{Watch: true})
	defer watcher.Stop()

	// Sleep for a bit to allow the watcher to start
	time.Sleep(100 * time.Millisecond)

	testResource.SetLabels(map[string]string{"test": "test"})
	_, err = fakeclient.Resource(gvr).Update(context.Background(), testResource, metav1.UpdateOptions{})
	if err != nil {
		t.Errorf("Update failed: %v", err)
	}

	err = fakeclient.Resource(gvr).Delete(context.Background(), testResource.GetName(), metav1.DeleteOptions{})
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Sleep for a bit to allow the watcher to get data
	time.Sleep(100 * time.Millisecond)

	// Check that the AddFunc is called once
	if numberOfAddFuncCalls != 1 {
		t.Errorf("AddFunc called %d times, expected 1", numberOfAddFuncCalls)
	}

	// Check that the UpdateFunc is called once
	if numberOfUpdateFuncCalls != 1 {
		t.Errorf("UpdateFunc called %d times, expected 1", numberOfUpdateFuncCalls)
	}

	// Check that the DeleteFunc is not called
	if numberOfDeleteFuncCalls != 1 {
		t.Errorf("DeleteFunc called %d times, expected 1", numberOfDeleteFuncCalls)
	}
}

func TestIsResourceVersionHigher(t *testing.T) {
	tests := []struct {
		name                   string
		resourceVersion        string
		currentResourceVersion string
		expectedResult         bool
	}{
		{
			name:                   "Empty currentResourceVersion",
			resourceVersion:        "123",
			currentResourceVersion: "",
			expectedResult:         true,
		},
		{
			name:                   "Valid resourceVersion and currentResourceVersion",
			resourceVersion:        "456",
			currentResourceVersion: "123",
			expectedResult:         true,
		},
		{
			name:                   "Invalid resourceVersion",
			resourceVersion:        "abc",
			currentResourceVersion: "123",
			expectedResult:         false,
		},
		{
			name:                   "Invalid currentResourceVersion",
			resourceVersion:        "456",
			currentResourceVersion: "def",
			expectedResult:         false,
		},
		{
			name:                   "Equal resourceVersion and currentResourceVersion",
			resourceVersion:        "456",
			currentResourceVersion: "456",
			expectedResult:         false,
		},
		{
			name:                   "currentResourceVersion higher then resourceVersion",
			resourceVersion:        "123",
			currentResourceVersion: "456",
			expectedResult:         false,
		},
		{
			name:                   "Valid resourceVersion and currentResourceVersion high numbers",
			resourceVersion:        "1234567892",
			currentResourceVersion: "1234567891",
			expectedResult:         true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isResourceVersionHigher(test.resourceVersion, test.currentResourceVersion)
			if result != test.expectedResult {
				t.Errorf("Expected %v, but got %v", test.expectedResult, result)
			}
		})
	}
}
