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
	watcher := NewWatcher(fakeclient)

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
