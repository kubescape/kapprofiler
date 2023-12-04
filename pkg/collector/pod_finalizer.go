package collector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type PodProfileFinalizerState struct {
	// Pod name
	PodName string
	// Pod namespace
	Namespace string
	// Timer
	FinalizationTimer *time.Timer
	// Recording state
	Recording bool
}

func (cm *CollectorManager) StartFinalizerWatcher() {
	// Initialize mutex
	cm.podFinalizerStateMutex = &sync.Mutex{}
	// Initialize map
	cm.podFinalizerState = make(map[string]*PodProfileFinalizerState)

	// Initialize factory and informer
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(cm.dynamicClient, 0, metav1.NamespaceAll, func(lo *metav1.ListOptions) {
		lo.FieldSelector = "spec.nodeName=" + cm.config.NodeName
	})
	//factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(cm.dynamicClient, 0, metav1.NamespaceAll, nil)
	// Informer for Pods
	informer := factory.ForResource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}).Informer()

	// Add event handlers to informer
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cm.handlePodAddEvent(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cm.handlePodUpdateEvent(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			cm.handlePodDeleteEvent(obj)
		},
	})

	// Run the informer
	go informer.Run(cm.podFinalizerControl)
}

func generateTableKey(obj metav1.Object) string {
	return fmt.Sprintf("%s-%s", obj.GetName(), obj.GetNamespace())
}

func (cm *CollectorManager) handlePodAddEvent(obj interface{}) {
	// Add pod to finalizer map

	// Convert object to Pod
	pod, err := ConvertInterfaceToPod(obj)
	if err != nil {
		log.Printf("the interface is not a Pod %v", err)
		return
	}

	podReady := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			podReady = true
		}
	}

	// Get mutex
	cm.podFinalizerStateMutex.Lock()
	// Get finalizer state
	_, ok := cm.podFinalizerState[generateTableKey(&pod.ObjectMeta)]
	if !ok {
		// Add pod to map
		cm.podFinalizerState[generateTableKey(&pod.ObjectMeta)] = &PodProfileFinalizerState{
			PodName:   pod.GetName(),
			Namespace: pod.GetNamespace(),
		}
		cm.podFinalizerStateMutex.Unlock()
	} else {
		cm.podFinalizerStateMutex.Unlock()
		// Check if pod is ready
		if podReady {
			// Start finalization timer
			cm.startFinalizationTimer(cm.config.FinalizeTime, pod)
		}
	}

}

func (cm *CollectorManager) handlePodUpdateEvent(oldObj, newObj interface{}) {
	// Need to access the status of the old and new pod to check if the pod became ready

	// Convert interface to Pod object
	oldPod, err := ConvertInterfaceToPod(oldObj)
	if err != nil {
		log.Printf("the interface is not a Pod %v", err)
		return
	}
	newPod, err := ConvertInterfaceToPod(newObj)
	if err != nil {
		log.Printf("the interface is not a Pod %v", err)
		return
	}

	// Check if recoding
	cm.podFinalizerStateMutex.Lock()
	finalizerState, ok := cm.podFinalizerState[generateTableKey(oldPod)]
	if !ok || !finalizerState.Recording {
		// Discard
		cm.podFinalizerStateMutex.Unlock()
		return
	}
	cm.podFinalizerStateMutex.Unlock()

	// Check old pod status
	oldPodReady := false
	for _, condition := range oldPod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			oldPodReady = true
		}
	}

	newPodReady := false
	for _, condition := range newPod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			newPodReady = true
		}
	}

	if newPodReady && !oldPodReady {
		// Pod became ready, add finalizer
		// Get mutex
		cm.podFinalizerStateMutex.Lock()
		defer cm.podFinalizerStateMutex.Unlock()

		// Check if pod is in map
		podState, ok := cm.podFinalizerState[generateTableKey(newPod)]
		if !ok {
			// Pod not in map (WTF?)
			log.Printf("Pod %s in namespace %s not in finalizer map", newPod.GetName(), newPod.GetNamespace())
			return
		}

		// Check if timer is running
		if podState.FinalizationTimer != nil {
			// Timer is running, no need to add finalizer
			return
		}

		// Timer is not running, add finalizer
		podState.FinalizationTimer = cm.startFinalizationTimer(cm.config.FinalizeTime, newPod)
	} else if !newPodReady && oldPodReady {
		cm.stopTimer(&newPod.ObjectMeta)
	}
}

// Timer function
func (cm *CollectorManager) startFinalizationTimer(seconds uint64, pod *v1.Pod) *time.Timer {
	finalizationTimer := time.NewTimer(time.Duration(seconds) * time.Second)

	// This goroutine waits for the timer to finish.
	go func() {
		<-finalizationTimer.C
		cm.finalizePodProfile(pod)
	}()

	return finalizationTimer
}

func (cm *CollectorManager) finalizePodProfile(pod *v1.Pod) {
	// Generate pod application profile name
	appProfileName := fmt.Sprintf("pod-%s", pod.GetName())
	// Put annotation on pod application profile to mark it as finalized
	_, err := cm.dynamicClient.Resource(AppProfileGvr).Namespace(pod.GetNamespace()).Patch(context.Background(),
		appProfileName, apitypes.MergePatchType, []byte("{\"metadata\":{\"annotations\":{\"kapprofiler.kubescape.io/final\":\"true\"}}}"), metav1.PatchOptions{})
	if err != nil {
		log.Printf("error patching application profile: %s\n", err)
	}
}

func (cm *CollectorManager) handlePodDeleteEvent(obj interface{}) {
	// Conver object to metav1.ObjectMeta
	pod, err := ConvertInterfaceToPod(obj)
	if err != nil {
		log.Printf("Error getting Pod object %v", err)
		return
	}

	// Delete timer if there is
	cm.stopTimer(&pod.ObjectMeta)

	// Generate pod application profile name
	appProfileName := fmt.Sprintf("pod-%s", pod.Name)
	// Delete pod application profile CRD
	err = cm.dynamicClient.Resource(AppProfileGvr).Namespace(pod.GetNamespace()).Delete(context.TODO(), appProfileName, metav1.DeleteOptions{})
	if err != nil {
		log.Printf("Error deleting pod application profile: %v", err)
		return
	}
}

func (cm *CollectorManager) stopTimer(pod *metav1.ObjectMeta) {
	// Get mutex
	cm.podFinalizerStateMutex.Lock()
	defer cm.podFinalizerStateMutex.Unlock()

	// Check if pod is in map
	podState, ok := cm.podFinalizerState[generateTableKey(pod)]
	if !ok {
		// Pod not in map (WTF?)
		log.Printf("Pod %s in namespace %s not in finalizer map", pod.GetName(), pod.GetNamespace())
		return
	}

	// Check if timer is running
	if podState.FinalizationTimer == nil {
		// Timer is not running, no need to remove finalizer
		return
	}

	// Timer is running, stop it
	podState.FinalizationTimer.Stop()
	podState.FinalizationTimer = nil
}

func (cm *CollectorManager) MarkPodRecording(pod, namespace string, attach bool) {
	if attach {
		// Check if pod is ready
		pod, err := cm.k8sClient.CoreV1().Pods(namespace).Get(context.Background(), pod, metav1.GetOptions{})
		if err != nil {
			log.Printf("Error getting pod %s in namespace %s: %v", pod, namespace, err)
			return
		}

		podReady := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				podReady = true
			}
		}

		if podReady {
			// Start finalization timer
			cm.startFinalizationTimer(cm.config.FinalizeTime, pod)
		}
	}

	// Get mutex
	cm.podFinalizerStateMutex.Lock()
	defer cm.podFinalizerStateMutex.Unlock()

	// Check if pod is in map
	podState, ok := cm.podFinalizerState[generateTableKey(&metav1.ObjectMeta{
		Name:      pod,
		Namespace: namespace,
	})]
	if !ok {
		// Add pod to map
		cm.podFinalizerState[generateTableKey(&metav1.ObjectMeta{
			Name:      pod,
			Namespace: namespace,
		})] = &PodProfileFinalizerState{
			PodName:   pod,
			Namespace: namespace,
			Recording: true,
		}
	} else {
		podState.Recording = true
	}
}

func (cm *CollectorManager) MarkPodNotRecording(pod, namespace string) {
	// Get mutex
	cm.podFinalizerStateMutex.Lock()
	defer cm.podFinalizerStateMutex.Unlock()

	// Check if pod is in map
	podState, ok := cm.podFinalizerState[generateTableKey(&metav1.ObjectMeta{
		Name:      pod,
		Namespace: namespace,
	})]
	if ok {
		podState.Recording = false
	}
}

func (cm *CollectorManager) StopFinalizerWatcher() {
	close(cm.podFinalizerControl)
}

func ConvertInterfaceToPod(obj interface{}) (*v1.Pod, error) {
	// Convert interface to unstructured object
	unstructuredPod, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("the interface is not an unstructured Pod")
	}

	var pod v1.Pod
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPod.UnstructuredContent(), &pod)
	if err != nil {
		return nil, err
	}

	return &pod, nil
}
