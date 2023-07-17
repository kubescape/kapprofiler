package collector

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ExecCalls struct {
	Path string
	Args []string
	Envs []string
}

type ContainerProfile struct {
	Name  string
	Execs []ExecCalls
}

type ApplicationProfileSpec struct {
	Containers []ContainerProfile
}

type ApplicationProfile struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the ApplicationProfile.
	Spec ApplicationProfileSpec `json:"spec,omitempty"`
}

var AppProfileGvr schema.GroupVersionResource = schema.GroupVersionResource{
	Group:    "kubescape.io",
	Version:  "v1",
	Resource: "applicationprofiles",
}
