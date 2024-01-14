package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/kubescape/kapprofiler/pkg/collector"
	"github.com/kubescape/kapprofiler/pkg/watcher"

	"golang.org/x/exp/slices"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// AppProfile controller struct
type Controller struct {
	config        *rest.Config
	staticClient  *kubernetes.Clientset
	dynamicClient *dynamic.DynamicClient
	appProfileGvr schema.GroupVersionResource
	watcher       watcher.WatcherInterface
}

// Create a new controller based on given config
func NewController(config *rest.Config) *Controller {

	// Initialize clients and channels
	staticClient, _ := kubernetes.NewForConfig(config)
	dynamicClient, _ := dynamic.NewForConfig(config)

	return &Controller{
		config:        config,
		staticClient:  staticClient,
		dynamicClient: dynamicClient,
		appProfileGvr: collector.AppProfileGvr,
	}
}

// Responsible for instantiating AppProfile controller
func (c *Controller) StartController() {

	// Initialize the watcher
	// TODO: due to memory constraints, we are not pre-listing the objects. This will be fixed in the future.
	// this means that will not consolidate the application profiles of the pods that are already running and not changing
	appProfileWatcher := watcher.NewWatcher(c.dynamicClient, false)

	// Start the watcher
	err := appProfileWatcher.Start(watcher.WatchNotifyFunctions{
		AddFunc: func(obj *unstructured.Unstructured) {
			c.handleApplicationProfile(obj)
		},
		UpdateFunc: func(obj *unstructured.Unstructured) {
			c.handleApplicationProfile(obj)
		},
		DeleteFunc: func(obj *unstructured.Unstructured) {
			c.handleApplicationProfile(obj)
		},
	}, collector.AppProfileGvr, metav1.ListOptions{})

	if err != nil {
		log.Printf("Error starting watcher %v", err)
		return
	}

	// Set the watcher
	c.watcher = appProfileWatcher
}

// Stop the AppProfile controller
func (c *Controller) StopController() {
	if c.watcher != nil {
		c.watcher.Stop()
	}
}

func (c *Controller) handleApplicationProfile(applicationProfileUnstructured *unstructured.Unstructured) {
	// If the application profile is marked as partial, do not propagate it
	if applicationProfileUnstructured.GetLabels()["kapprofiler.kubescape.io/partial"] == "true" {
		return
	}

	// Get Object name from ApplicationProfile. Application profile name has the kind in as the the prefix like deployment-nginx
	objectName := strings.Join(strings.Split(applicationProfileUnstructured.GetName(), "-")[1:], "-")

	// Get pod to which the ApplicationProfile belongs to
	pod, err := c.staticClient.CoreV1().Pods(applicationProfileUnstructured.GetNamespace()).Get(context.TODO(), objectName, metav1.GetOptions{})
	if err != nil { // Ensures that the ApplicationProfile belongs to a pod or a replicaset
		replicaSet, err := c.staticClient.AppsV1().ReplicaSets(applicationProfileUnstructured.GetNamespace()).Get(context.TODO(), objectName, metav1.GetOptions{})
		if err != nil { // ApplicationProfile belongs to neither
			return
		}

		if len(replicaSet.OwnerReferences) > 0 && replicaSet.OwnerReferences[0].Kind == "Deployment" { // If owner of replicaset is a deployment
			profileName := fmt.Sprintf("deployment-%v", replicaSet.OwnerReferences[0].Name)
			existingApplicationProfile, err := c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(replicaSet.Namespace).Get(context.TODO(), profileName, metav1.GetOptions{})
			if err != nil { // ApplicationProfile doesn't exist for deployment
				applicationProfile, err := getApplicationProfileFromUnstructured(applicationProfileUnstructured)
				if err != nil {
					return
				}
				deploymentApplicationProfile := collector.ApplicationProfile{
					TypeMeta: metav1.TypeMeta{
						Kind:       collector.ApplicationProfileKind,
						APIVersion: collector.ApplicationProfileApiVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:   profileName,
						Labels: applicationProfileUnstructured.GetLabels(),
					},
					Spec: collector.ApplicationProfileSpec{
						Containers: applicationProfile.Spec.Containers,
					},
				}
				deploymentApplicationProfileRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&deploymentApplicationProfile)
				if err != nil {
					return
				}
				_, err = c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(replicaSet.Namespace).Create(context.TODO(), &unstructured.Unstructured{Object: deploymentApplicationProfileRaw}, metav1.CreateOptions{})
				if err != nil {
					return
				}
			} else { // ApplicationProfile exists for deployment
				// Check if the higher level application profile is marked as final (imutable)
				if existingApplicationProfile.GetLabels()["kapprofiler.kubescape.io/final"] == "true" {
					// Don't update the application profile
					return
				}

				applicationProfile, err := getApplicationProfileFromUnstructured(applicationProfileUnstructured)
				if err != nil {
					return
				}

				deploymentApplicationProfile := collector.ApplicationProfile{}
				deploymentApplicationProfile.Labels = applicationProfile.GetLabels()
				deploymentApplicationProfile.Spec.Containers = applicationProfile.Spec.Containers
				deploymentApplicationProfileRaw, _ := json.Marshal(deploymentApplicationProfile)
				_, err = c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(replicaSet.Namespace).Patch(context.TODO(), profileName, apitypes.MergePatchType, deploymentApplicationProfileRaw, metav1.PatchOptions{})
				if err != nil {
					return
				}
			}
		} else {
			log.Printf("ApplicationProfile %v doesn't belong to a deployment", applicationProfileUnstructured.GetName())
			return
		}
	}

	var podControllerName string
	var podControllerKind string
	var pods *v1.PodList

	// Skip if the pod has no owner
	if len(pod.OwnerReferences) == 0 {
		return
	}

	// Figure out pod controller of given pod and get all pods under that controller
	switch pod.OwnerReferences[0].Kind {
	case "ReplicaSet":
		replicaSet, err := c.staticClient.AppsV1().ReplicaSets(pod.Namespace).Get(context.TODO(), pod.OwnerReferences[0].Name, metav1.GetOptions{})
		if err != nil {
			return
		}
		pods, err = c.staticClient.CoreV1().Pods(pod.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: string(labels.Set(replicaSet.Spec.Selector.MatchLabels).AsSelector().String())})
		if err != nil {
			return
		}
		podControllerName = replicaSet.GetName()
		podControllerKind = "replicaset"
	case "DaemonSet":
		daemonSet, err := c.staticClient.AppsV1().DaemonSets(pod.Namespace).Get(context.TODO(), pod.OwnerReferences[0].Name, metav1.GetOptions{})
		if err != nil {
			return
		}
		pods, err = c.staticClient.CoreV1().Pods(pod.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: string(labels.Set(daemonSet.Spec.Selector.MatchLabels).AsSelector().String())})
		if err != nil {
			return
		}
		podControllerName = daemonSet.GetName()
		podControllerKind = "daemonset"
	case "StatefulSet":
		statefulSet, err := c.staticClient.AppsV1().StatefulSets(pod.Namespace).Get(context.TODO(), pod.OwnerReferences[0].Name, metav1.GetOptions{})
		if err != nil {
			return
		}
		pods, err = c.staticClient.CoreV1().Pods(pod.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: string(labels.Set(statefulSet.Spec.Selector.MatchLabels).AsSelector().String())})
		if err != nil {
			return
		}
		podControllerName = statefulSet.GetName()
		podControllerKind = "statefulset"
	case "CronJob":
		cronJob, err := c.staticClient.BatchV1beta1().CronJobs(pod.Namespace).Get(context.TODO(), pod.OwnerReferences[0].Name, metav1.GetOptions{})
		if err != nil {
			return
		}
		pods, err = c.staticClient.CoreV1().Pods(pod.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: string(labels.Set(cronJob.Spec.JobTemplate.Spec.Template.Labels).AsSelector().String())})
		if err != nil {
			return
		}
		podControllerName = cronJob.GetName()
		podControllerKind = "cronjob"
	case "Job":
		job, err := c.staticClient.BatchV1().Jobs(pod.Namespace).Get(context.TODO(), pod.OwnerReferences[0].Name, metav1.GetOptions{})
		if err != nil {
			return
		}
		pods, err = c.staticClient.CoreV1().Pods(pod.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: string(labels.Set(job.Spec.Template.Labels).AsSelector().String())})
		if err != nil {
			return
		}
		podControllerName = job.GetName()
		podControllerKind = "job"
	default:
		// If the pod controller is not a replicaset, daemonset or statefulset, then skip
		return
	}

	containersMap := make(map[string]collector.ContainerProfile)

	// Merge all the container information of all the pods
	for _, pod := range pods.Items {
		appProfileNameForPod := fmt.Sprintf("pod-%s", pod.GetName())
		typedObj, err := c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(pod.GetNamespace()).Get(context.TODO(), appProfileNameForPod, metav1.GetOptions{})
		if err != nil {
			log.Printf("ApplicationProfile for pod %v doesn't exist", pod.GetName())
			return
		}
		podApplicationProfileObj, err := getApplicationProfileFromUnstructured(typedObj)
		if err != nil {
			log.Printf("ApplicationProfile for pod %v doesn't exist", pod.GetName())
			return
		}

		// TODO: Make this code more efficient and less repetitive.
		for _, container := range podApplicationProfileObj.Spec.Containers {
			// Merge containers
			if mapContainer, exists := containersMap[container.Name]; exists {
				// Merge SysCalls
				for _, sysCall := range container.SysCalls {
					if !slices.Contains(mapContainer.SysCalls, sysCall) {
						mapContainer.SysCalls = append(mapContainer.SysCalls, sysCall)
					}
				}

				// Merge Execs
				for _, exec := range container.Execs {
					contains := false
					for _, mapExec := range mapContainer.Execs {
						if mapExec.Equals(exec) {
							contains = true
							break
						}
					}
					if !contains {
						mapContainer.Execs = append(mapContainer.Execs, exec)
					}
				}

				// Merge Capabilities
				for _, capability := range container.Capabilities {
					contains := false
					for _, mapCapability := range mapContainer.Capabilities {
						if mapCapability.Equals(capability) {
							contains = true
							break
						}
					}
					if !contains {
						mapContainer.Capabilities = append(mapContainer.Capabilities, capability)
					}
				}

				// Merge Opens
				for _, open := range container.Opens {
					contains := false
					for _, mapOpen := range mapContainer.Opens {
						if mapOpen.Equals(open) {
							contains = true
							break
						}
					}
					if !contains {
						mapContainer.Opens = append(mapContainer.Opens, open)
					}
				}

				// Merge Dns
				for _, dns := range container.Dns {
					contains := false
					for _, mapDns := range mapContainer.Dns {
						if mapDns.Equals(dns) {
							contains = true
							break
						}
					}
					if !contains {
						mapContainer.Dns = append(mapContainer.Dns, dns)
					}
				}

				// Merge Network
				for _, network := range container.NetworkActivity.Incoming {
					contains := false
					for _, mapNetwork := range mapContainer.NetworkActivity.Incoming {
						if mapNetwork.Equals(network) {
							contains = true
							break
						}
					}
					if !contains {
						mapContainer.NetworkActivity.Incoming = append(mapContainer.NetworkActivity.Incoming, network)
					}
				}

				for _, network := range container.NetworkActivity.Outgoing {
					contains := false
					for _, mapNetwork := range mapContainer.NetworkActivity.Outgoing {
						if mapNetwork.Equals(network) {
							contains = true
							break
						}
					}
					if !contains {
						mapContainer.NetworkActivity.Outgoing = append(mapContainer.NetworkActivity.Outgoing, network)
					}
				}

				containersMap[container.Name] = mapContainer
			} else {
				containersMap[container.Name] = container
			}
		}
	}

	var containers []collector.ContainerProfile
	for _, container := range containersMap {
		containers = append(containers, container)
	}

	applicationProfileNameForController := fmt.Sprintf("%s-%s", podControllerKind, podControllerName)
	// Fetch ApplicationProfile of the controller
	existingApplicationProfile, err := c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(pod.Namespace).Get(context.TODO(), applicationProfileNameForController, metav1.GetOptions{})
	if err != nil { // ApplicationProfile of controller doesn't exist so create a new one
		controllerApplicationProfile := collector.ApplicationProfile{
			TypeMeta: metav1.TypeMeta{
				Kind:       collector.ApplicationProfileKind,
				APIVersion: collector.ApplicationProfileApiVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   applicationProfileNameForController,
				Labels: applicationProfileUnstructured.GetLabels(),
			},
			Spec: collector.ApplicationProfileSpec{
				Containers: containers,
			},
		}
		controllerApplicationProfileRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&controllerApplicationProfile)
		if err != nil {
			log.Printf("Error converting ApplicationProfile of controller %v", err)
			return
		}
		_, err = c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(pod.Namespace).Create(context.TODO(), &unstructured.Unstructured{Object: controllerApplicationProfileRaw}, metav1.CreateOptions{})
		if err != nil {
			log.Printf("Error creating ApplicationProfile of controller %v", err)
			return
		}
	} else { // ApplicationProfile of controller exists so update it
		// Check if the higher level application profile is marked as final (imutable)
		if existingApplicationProfile.GetLabels()["kapprofiler.kubescape.io/final"] == "true" {
			// Don't update the application profile
			return
		}
		controllerApplicationProfile := collector.ApplicationProfile{}
		controllerApplicationProfile.Labels = applicationProfileUnstructured.GetLabels()
		controllerApplicationProfile.Spec.Containers = containers
		controllerApplicationProfileRaw, _ := json.Marshal(controllerApplicationProfile)
		_, err = c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(pod.Namespace).Patch(context.TODO(), applicationProfileNameForController, apitypes.MergePatchType, controllerApplicationProfileRaw, metav1.PatchOptions{})
		if err != nil {
			log.Printf("Error updating ApplicationProfile of controller %v", err)
			return
		}
	}
}

// Helper function to convert interface to ApplicationProfile
func getApplicationProfileFromUnstructured(typedObj *unstructured.Unstructured) (*collector.ApplicationProfile, error) {
	var applicationProfileObj collector.ApplicationProfile
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(typedObj.Object, &applicationProfileObj)
	if err != nil {
		return &collector.ApplicationProfile{}, err
	}
	return &applicationProfileObj, nil
}
