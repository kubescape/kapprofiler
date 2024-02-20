package watcher

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type resourceVersionGetter interface {
	GetResourceVersion() string
}

type WatchNotifyFunctions struct {
	AddFunc    func(obj *unstructured.Unstructured)
	UpdateFunc func(obj *unstructured.Unstructured)
	DeleteFunc func(obj *unstructured.Unstructured)
	OnError    func(err error)
}

type WatcherInterface interface {
	Start(notifyF WatchNotifyFunctions, gvr schema.GroupVersionResource, listOptions metav1.ListOptions) error
	Stop()
	Destroy()
}

type Watcher struct {
	preList             bool
	client              dynamic.Interface
	watcher             watch.Interface
	lastResourceVersion string
	running             bool
}

func NewWatcher(k8sClient dynamic.Interface, preList bool) WatcherInterface {
	return &Watcher{client: k8sClient, watcher: nil, running: false, preList: preList, lastResourceVersion: ""}
}

func (w *Watcher) Start(notifyF WatchNotifyFunctions, gvr schema.GroupVersionResource, listOptions metav1.ListOptions) error {
	if w.watcher != nil {
		return fmt.Errorf("watcher already started")
	}

	// Get a list of current namespaces from the API server
	nameSpaceGvr := schema.GroupVersionResource{
		Group:    "", // The group is empty for core API groups
		Version:  "v1",
		Resource: "namespaces",
	}

	// List the namespaces
	namespaces, err := w.client.Resource(nameSpaceGvr).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// List of current objects
	w.lastResourceVersion = ""

	if w.preList {
		listOptions.Watch = false
		for _, ns := range namespaces.Items {
			list, err := w.client.Resource(gvr).Namespace(ns.GetName()).List(context.Background(), listOptions)
			if err != nil {
				return err
			}
			for i, item := range list.Items {
				if isResourceVersionHigher(item.GetResourceVersion(), w.lastResourceVersion) {
					// Update the resourceVersion to the latest
					w.lastResourceVersion = item.GetResourceVersion()
					if w.preList {
						notifyF.AddFunc(&item)
					}
					// Make sure the item is scraped by the GC
					list.Items[i] = unstructured.Unstructured{}
				}
			}
			list.Items = nil
			list = nil
		}
	}

	// Start the watcher
	listOptions.ResourceVersion = w.lastResourceVersion
	listOptions.Watch = true
	listOptions.AllowWatchBookmarks = true
	w.watcher, err = w.client.Resource(gvr).Namespace("").Watch(context.TODO(), listOptions)
	if err != nil {
		return err
	}
	w.running = true
	go func() {
		// Watch for events
		for {
			if w.watcher == nil {
				w.watcher, err = w.client.Resource(gvr).Namespace("").Watch(context.TODO(), listOptions)
				if err != nil {
					if notifyF.OnError != nil {
						notifyF.OnError(err)
					} else {
						log.Printf("watcher error: %v", err)
					}
					return
				}
			}
			event, ok := <-w.watcher.ResultChan()
			if !ok {
				if w.running {
					log.Printf("Restarting watcher on object %+v", gvr)
					w.watcher.Stop()
					w.watcher = nil
					listOptions.ResourceVersion = w.lastResourceVersion
					// Retry 5 times with exponential backoff.
					for i := 0; i < 5; i++ {
						w.watcher, err = w.client.Resource(gvr).Namespace("").Watch(context.TODO(), listOptions)
						if err == nil {
							break
						}
						log.Printf("watcher restart error: %v, on object: %+v, retrying...", err, gvr)
						time.Sleep(time.Second * time.Duration(i*2)) // Exponential backoff
					}

					if err != nil {
						// If the watcher restart fails after 5 attempts, log the error and call the OnError function.
						if notifyF.OnError != nil {
							notifyF.OnError(err)
						} else {
							log.Printf("Final watcher restart error: %v, on object: %+v", err, gvr)
							log.Println("Closing watcher")
						}
						return
					}
					continue
				} else {
					// Stop the watcher
					return
				}
			}

			if metaObject, ok := event.Object.(resourceVersionGetter); ok {
				// Update the resourceVersion to the latest
				if metaObject.GetResourceVersion() != "" && isResourceVersionHigher(metaObject.GetResourceVersion(), w.lastResourceVersion) {
					w.lastResourceVersion = metaObject.GetResourceVersion()
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
				notifyF.AddFunc(addedObject)
				addedObject = nil // Make sure the item is scraped by the GC
			case watch.Modified:
				// Convert the object to unstructured
				modifiedObject := event.Object.(*unstructured.Unstructured)
				if modifiedObject == nil {
					log.Printf("watcher error: modifiedObject is nil")
					continue
				}
				notifyF.UpdateFunc(modifiedObject)
				modifiedObject = nil // Make sure the item is scraped by the GC
			case watch.Deleted:
				// Convert the object to unstructured
				deletedObject := event.Object.(*unstructured.Unstructured)
				if deletedObject == nil {
					log.Printf("watcher error: deletedObject is nil")
					continue
				}
				notifyF.DeleteFunc(deletedObject)
				deletedObject = nil // Make sure the item is scraped by the GC
			case watch.Error:
				// Convert the object to metav1.Status
				watchError := event.Object.(*metav1.Status)
				if watchError != nil {
					if watchError.Code == 410 {
						// If the resourceVersion is too old, reset the resourceVersion to the latest.
						// https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions
						w.lastResourceVersion = ""
					}
				}
				continue
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

func isResourceVersionHigher(resourceVersion string, currentResourceVersion string) bool {
	// If the currentResourceVersion is empty, return true
	if currentResourceVersion == "" {
		return true
	}

	// Convert the resourceVersion to int64
	resourceVersionInt, err := strconv.ParseInt(resourceVersion, 10, 64)
	if err != nil {
		return false
	}

	// Convert the currentResourceVersion to int64
	currentResourceVersionInt, err := strconv.ParseInt(currentResourceVersion, 10, 64)
	if err != nil {
		return false
	}

	return resourceVersionInt > currentResourceVersionInt
}
