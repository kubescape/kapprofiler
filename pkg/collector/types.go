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

type NetworkCalls struct {
	Protocol    string `json:"protocol" yaml:"protocol"`
	Port        uint16 `json:"port" yaml:"port"`
	DstEndpoint string `json:"dstEndpoint" yaml:"dstEndpoint"`
}

type NetworkActivity struct {
	Incoming []NetworkCalls `json:"incoming" yaml:"incoming"`
	Outgoing []NetworkCalls `json:"outgoing" yaml:"outgoing"`
}

type OpenCalls struct {
	Path  string   `json:"path" yaml:"path"`
	Flags []string `json:"flags" yaml:"flags"`
}

type CapabilitiesCalls struct {
	Capabilities []string `json:"caps" yaml:"caps"`
	Syscall      string   `json:"syscall" yaml:"syscall"`
}

type DnsCalls struct {
	DnsName   string   `json:"dnsName" yaml:"dnsName"`
	Addresses []string `json:"addresses" yaml:"addresses"`
}

type ContainerProfile struct {
	Name            string              `json:"name" yaml:"name"`
	Execs           []ExecCalls         `json:"execs" yaml:"execs"`
	Opens           []OpenCalls         `json:"opens" yaml:"opens"`
	NetworkActivity NetworkActivity     `json:"networkActivity" yaml:"networkActivity"`
	Capabilities    []CapabilitiesCalls `json:"capabilities" yaml:"capabilities"`
	Dns             []DnsCalls          `json:"dns" yaml:"dns"`
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

func (a ExecCalls) Equals(b ExecCalls) bool {
	if a.Path != b.Path {
		return false
	}
	if len(a.Args) != len(b.Args) {
		return false
	}
	for i, arg := range a.Args {
		if arg != b.Args[i] {
			return false
		}
	}
	// TODO: compare envs
	return true
}

func (a OpenCalls) Equals(b OpenCalls) bool {
	if a.Path != b.Path {
		return false
	}
	if len(a.Flags) != len(b.Flags) {
		return false
	}
	for i, flag := range a.Flags {
		if flag != b.Flags[i] {
			return false
		}
	}
	return true
}

func (a CapabilitiesCalls) Equals(b CapabilitiesCalls) bool {
	if a.Syscall != b.Syscall {
		return false
	}
	if len(a.Capabilities) != len(b.Capabilities) {
		return false
	}
	for i, cap := range a.Capabilities {
		if cap != b.Capabilities[i] {
			return false
		}
	}
	return true
}

func (a DnsCalls) Equals(b DnsCalls) bool {
	if a.DnsName != b.DnsName {
		return false
	}
	if len(a.Addresses) != len(b.Addresses) {
		return false
	}
	for _, addr := range a.Addresses {
		found := false
		for _, baddr := range b.Addresses {
			if addr == baddr {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (a NetworkCalls) Equals(b NetworkCalls) bool {
	if a.Protocol != b.Protocol {
		return false
	}
	if a.Port != b.Port {
		return false
	}
	if a.DstEndpoint != b.DstEndpoint {
		return false
	}
	return true
}

func (a NetworkActivity) Equals(b NetworkActivity) bool {
	if len(a.Incoming) != len(b.Incoming) {
		return false
	}
	for i, inc := range a.Incoming {
		if !inc.Equals(b.Incoming[i]) {
			return false
		}
	}
	if len(a.Outgoing) != len(b.Outgoing) {
		return false
	}
	for i, out := range a.Outgoing {
		if !out.Equals(b.Outgoing[i]) {
			return false
		}
	}
	return true
}
