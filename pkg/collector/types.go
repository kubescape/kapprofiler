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

const (
	// ApplicationProfileKind is the kind of ApplicationProfile
	ApplicationProfileKind string = "ApplicationProfile"
	// ApplicationProfileGroup is the group of ApplicationProfile
	ApplicationProfileGroup string = "kubescape.io"
	// ApplicationProfileVersion is the version of ApplicationProfile
	ApplicationProfileVersion string = "v1"
	// ApplicationProfilePlural is the plural of ApplicationProfile
	ApplicationProfilePlural string = "applicationprofiles"
	// ApplicationProfileApiVersion is the api version of ApplicationProfile
	ApplicationProfileApiVersion string = ApplicationProfileGroup + "/" + ApplicationProfileVersion
)

var AppProfileGvr schema.GroupVersionResource = schema.GroupVersionResource{
	Group:    ApplicationProfileGroup,
	Version:  ApplicationProfileVersion,
	Resource: ApplicationProfilePlural,
}
