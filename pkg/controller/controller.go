package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/kubescape/kapprofiler/pkg/collector"

	"golang.org/x/exp/slices"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// AppProfile controller struct
type Controller struct {
	config            *rest.Config
	staticClient      *kubernetes.Clientset
	dynamicClient     *dynamic.DynamicClient
	appProfileGvr     schema.GroupVersionResource
	controllerChannel chan struct{}
}

// Create a new controller based on given config
func NewController(config *rest.Config) *Controller {

	// Initialize clients and channels
	staticClient, _ := kubernetes.NewForConfig(config)
	dynamicClient, _ := dynamic.NewForConfig(config)
	controllerChannel := make(chan struct{})

	return &Controller{
		config:            config,
		staticClient:      staticClient,
		dynamicClient:     dynamicClient,
		appProfileGvr:     collector.AppProfileGvr,
		controllerChannel: controllerChannel,
	}
}

// Responsible for instantiating AppProfile controller
func (c *Controller) StartController() {

	// Initialize factory and informer
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(c.dynamicClient, 0, metav1.NamespaceAll, nil)
	informer := factory.ForResource(c.appProfileGvr).Informer()

	// Add event handlers to informer
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) { // Called when an ApplicationProfile is added
			c.handleApplicationProfile(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) { // Called when an ApplicationProfile is updated
			old, _ := c.getApplicationProfileFromObj(oldObj)
			new, _ := c.getApplicationProfileFromObj(newObj)
			if reflect.DeepEqual(old.Spec, new.Spec) {
				return
			}

			c.handleApplicationProfile(newObj)
		},
		DeleteFunc: func(obj interface{}) { // Called when an ApplicationProfile is deleted
			c.handleApplicationProfile(obj)
		},
	})

	// Run the informer
	go informer.Run(c.controllerChannel)
}

// Stop the AppProfile controller
func (c *Controller) StopController() {
	close(c.controllerChannel)
}

func (c *Controller) handleApplicationProfile(obj interface{}) {
	applicationProfile, err := c.getApplicationProfileFromObj(obj)
	if err != nil {
		return
	}

	// If the application profile is marked as partial, do not propagate it
	if applicationProfile.GetAnnotations()["kapprofiler.kubescape.io/partial"] == "true" {
		return
	}

	// Get Object name from ApplicationProfile. Application profile name has the kind in as the prefix like deployment-nginx
	objectName := strings.Join(strings.Split(applicationProfile.ObjectMeta.Name, "-")[1:], "-")

	// Get pod to which the ApplicationProfile belongs to
	pod, err := c.staticClient.CoreV1().Pods(applicationProfile.ObjectMeta.Namespace).Get(context.TODO(), objectName, metav1.GetOptions{})
	if err != nil { // Ensures that the ApplicationProfile belongs to a pod or a replicaset
		replicaSet, err := c.staticClient.AppsV1().ReplicaSets(applicationProfile.ObjectMeta.Namespace).Get(context.TODO(), objectName, metav1.GetOptions{})
		if err != nil { // ApplicationProfile belongs to neither
			return
		}

		if len(replicaSet.OwnerReferences) > 0 && replicaSet.OwnerReferences[0].Kind == "Deployment" { // If owner of replicaset is a deployment
			profileName := fmt.Sprintf("deployment-%v", replicaSet.OwnerReferences[0].Name)
			existingApplicationProfile, err := c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(replicaSet.Namespace).Get(context.TODO(), profileName, metav1.GetOptions{})
			if err != nil { // ApplicationProfile doesn't exist for deployment
				deploymentApplicationProfile := &collector.ApplicationProfile{
					TypeMeta: metav1.TypeMeta{
						Kind:       collector.ApplicationProfileKind,
						APIVersion: collector.ApplicationProfileApiVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        profileName,
						Annotations: applicationProfile.GetAnnotations(),
					},
					Spec: collector.ApplicationProfileSpec{
						Containers: applicationProfile.Spec.Containers,
					},
				}
				deploymentApplicationProfileRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploymentApplicationProfile)
				if err != nil {
					return
				}
				_, err = c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(replicaSet.Namespace).Create(context.TODO(), &unstructured.Unstructured{Object: deploymentApplicationProfileRaw}, metav1.CreateOptions{})
				if err != nil {
					return
				}
			} else { // ApplicationProfile exists for deployment
				// Check if the higher level application profile is marked as final (imutable)
				if existingApplicationProfile.GetAnnotations()["kapprofiler.kubescape.io/final"] == "true" {
					// Don't update the application profile
					return
				}

				deploymentApplicationProfile := &collector.ApplicationProfile{}
				deploymentApplicationProfile.Annotations = applicationProfile.GetAnnotations()
				deploymentApplicationProfile.Spec.Containers = applicationProfile.Spec.Containers
				deploymentApplicationProfileRaw, _ := json.Marshal(deploymentApplicationProfile)
				_, err = c.dynamicClient.Resource(collector.AppProfileGvr).Namespace(replicaSet.Namespace).Patch(context.TODO(), profileName, apitypes.MergePatchType, deploymentApplicationProfileRaw, metav1.PatchOptions{})
				if err != nil {
					return
				}
			}
		} else {
			log.Printf("ApplicationProfile %v doesn't belong to a deployment", applicationProfile.ObjectMeta.Name)
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
		podApplicationProfileObj, err := c.getApplicationProfileFromObj(typedObj)
		if err != nil {
			log.Printf("ApplicationProfile for pod %v doesn't exist", pod.GetName())
			return
		}

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
		controllerApplicationProfile := &collector.ApplicationProfile{
			TypeMeta: metav1.TypeMeta{
				Kind:       collector.ApplicationProfileKind,
				APIVersion: collector.ApplicationProfileApiVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        applicationProfileNameForController,
				Annotations: applicationProfile.GetAnnotations(),
			},
			Spec: collector.ApplicationProfileSpec{
				Containers: containers,
			},
		}
		controllerApplicationProfileRaw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(controllerApplicationProfile)
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
		if existingApplicationProfile.GetAnnotations()["kapprofiler.kubescape.io/final"] == "true" {
			// Don't update the application profile
			return
		}
		controllerApplicationProfile := &collector.ApplicationProfile{}
		controllerApplicationProfile.Annotations = applicationProfile.GetAnnotations()
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
func (c *Controller) getApplicationProfileFromObj(obj interface{}) (*collector.ApplicationProfile, error) {
	typedObj := obj.(*unstructured.Unstructured)
	bytes, err := typedObj.MarshalJSON()
	if err != nil {
		return &collector.ApplicationProfile{}, err
	}

	var applicationProfileObj *collector.ApplicationProfile
	err = json.Unmarshal(bytes, &applicationProfileObj)
	if err != nil {
		return applicationProfileObj, err
	}
	return applicationProfileObj, nil
}
