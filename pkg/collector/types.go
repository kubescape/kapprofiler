package collector

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ExecCalls struct {
	Path string   `json:"path" yaml:"path"`
	Args []string `json:"args" yaml:"args"`
	Envs []string `json:"envs" yaml:"envs"`
}

type RawTcpConnection struct {
	SourceIp   string `json:"sourceip" yaml:"sourceip"`
	SourcePort int    `json:"sourceport" yaml:"sourceport"`
	DestIp     string `json:"destip" yaml:"destip"`
	DestPort   int    `json:"destport" yaml:"destport"`
}

type EnrichedTcpConnection struct {
	RawConnection RawTcpConnection   `json:"rawconnection" yaml:"rawconnection"`
	PodSelectors  []v1.LabelSelector `json:"podselectors" yaml:"podselectors"`
}

type ConnectionContainer struct {
	// Enriched connections
	TcpConnections []EnrichedTcpConnection `json:"tcpconnections" yaml:"tcpconnections"`
}

type NetworkActivity struct {
	Incoming ConnectionContainer `json:"incoming" yaml:"incoming"`
	Outgoing ConnectionContainer `json:"outgoing" yaml:"outgoing"`
}

type OpenCalls struct {
	Path  string   `json:"path" yaml:"path"`
	Flags []string `json:"flags" yaml:"flags"`
}

type CapabilitiesCalls struct {
	Capabilities []string `json:"caps" yaml:"caps"`
	Syscall      string   `json:"syscall" yaml:"syscall"`
}

type ContainerProfile struct {
	Name            string              `json:"name" yaml:"name"`
	Execs           []ExecCalls         `json:"execs" yaml:"execs"`
	Opens           []OpenCalls         `json:"opens" yaml:"opens"`
	NetworkActivity NetworkActivity     `json:"networkActivity" yaml:"networkActivity"`
	Capabilities    []CapabilitiesCalls `json:"capabilities" yaml:"capabilities"`
	SysCalls        []string            `json:"syscalls" yaml:"syscalls"`
}

type ApplicationProfileSpec struct {
	Containers []ContainerProfile `json:"containers" yaml:"containers"`
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
