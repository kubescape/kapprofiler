package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type WatchNotifyFunctions struct {
	AddFunc    func(obj *unstructured.Unstructured)
	UpdateFunc func(obj *unstructured.Unstructured)
	DeleteFunc func(obj *unstructured.Unstructured)
}

type WatcherInterface interface {
	Start(notifyF WatchNotifyFunctions, gvr schema.GroupVersionResource, listOptions metav1.ListOptions) error
	Stop()
	Destroy()
}

type Watcher struct {
	client  dynamic.Interface
	watcher watch.Interface
	running bool
}

func NewWatcher(k8sClient dynamic.Interface) WatcherInterface {
	return &Watcher{client: k8sClient, watcher: nil}
}

func (w *Watcher) Start(notifyF WatchNotifyFunctions, gvr schema.GroupVersionResource, listOptions metav1.ListOptions) error {
	if w.watcher != nil {
		return fmt.Errorf("watcher already started")
	}

	// List of current objects
	resourceVersion := ""
	listOptions.Watch = false
	list, err := w.client.Resource(gvr).Namespace("").List(context.Background(), listOptions)
	if err != nil {
		return err
	}
	// Get the resourceVersion of the last object
	for _, item := range list.Items {
		if item.GetResourceVersion() > resourceVersion {
			resourceVersion = item.GetResourceVersion()
		}
	}

	listOptions.ResourceVersion = resourceVersion
	watcher, err := w.client.Resource(gvr).Namespace("").Watch(context.Background(), listOptions)
	if err != nil {
		return err
	}
	w.watcher = watcher
	w.running = true
	go func() {

		// Send the current objects
		for _, item := range list.Items {
			notifyF.AddFunc(&item)
		}

		// Watch for events

		for {
			event, ok := <-watcher.ResultChan()
			if !ok {
				if w.running {
					// Need to restart the watcher: wait a bit and restart
					time.Sleep(5 * time.Second)
					listOptions.ResourceVersion = resourceVersion
					w.watcher, err = w.client.Resource(gvr).Namespace("").Watch(context.Background(), listOptions)
					if err != nil {
						log.Printf("watcher restart error: %v", err)
					}
					// Restart the loop
					continue
				} else {
					// Stop the watcher
					return
				}
			}
			switch event.Type {
			case watch.Added:
				// Convert the object to unstructured
				addedObject := event.Object.(*unstructured.Unstructured)
				if addedObject == nil {
					log.Printf("watcher error: addedObject is nil")
					continue
				}
				// Update the resourceVersion
				if addedObject.GetResourceVersion() > resourceVersion {
					resourceVersion = addedObject.GetResourceVersion()
				}
				notifyF.AddFunc(addedObject)
			case watch.Modified:
				// Convert the object to unstructured
				modifiedObject := event.Object.(*unstructured.Unstructured)
				if modifiedObject == nil {
					log.Printf("watcher error: modifiedObject is nil")
					continue
				}
				// Update the resourceVersion
				if modifiedObject.GetResourceVersion() > resourceVersion {
					resourceVersion = modifiedObject.GetResourceVersion()
				}
				notifyF.UpdateFunc(modifiedObject)
			case watch.Deleted:
				// Convert the object to unstructured
				deletedObject := event.Object.(*unstructured.Unstructured)
				if deletedObject == nil {
					log.Printf("watcher error: deletedObject is nil")
					continue
				}
				// Update the resourceVersion
				if deletedObject.GetResourceVersion() > resourceVersion {
					resourceVersion = deletedObject.GetResourceVersion()
				}
				notifyF.DeleteFunc(deletedObject)
			case watch.Error:
				log.Printf("watcher error: %v", event.Object)
			}
		}
	}()

	return nil
}

func (w *Watcher) Stop() {
	if w.watcher != nil {
		w.running = false
		w.watcher.Stop()
		w.watcher = nil
	}
}

func (w *Watcher) Destroy() {
}
