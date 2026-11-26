package labelstructs

import (
	"gopkg.in/yaml.v3"
)

// SeccompProfile is the seccomp profile applied to a pod or container.
type SeccompProfile struct {
	Type             string  `yaml:"type" json:"type"`
	LocalhostProfile *string `yaml:"localhostProfile,omitempty" json:"localhostProfile,omitempty"`
}

// Capabilities is the set of Linux capabilities added to or dropped from a container.
type Capabilities struct {
	Add  []string `yaml:"add,omitempty" json:"add,omitempty"`
	Drop []string `yaml:"drop,omitempty" json:"drop,omitempty"`
}

// Sysctl is a kernel parameter set for the pod.
type Sysctl struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

// PodSecurityContext is the pod-level security context.
type PodSecurityContext struct {
	RunAsUser           *int64          `yaml:"runAsUser,omitempty" json:"runAsUser,omitempty"`
	RunAsGroup          *int64          `yaml:"runAsGroup,omitempty" json:"runAsGroup,omitempty"`
	RunAsNonRoot        *bool           `yaml:"runAsNonRoot,omitempty" json:"runAsNonRoot,omitempty"`
	FSGroup             *int64          `yaml:"fsGroup,omitempty" json:"fsGroup,omitempty"`
	FSGroupChangePolicy *string         `yaml:"fsGroupChangePolicy,omitempty" json:"fsGroupChangePolicy,omitempty"`
	SupplementalGroups  []int64         `yaml:"supplementalGroups,omitempty" json:"supplementalGroups,omitempty"`
	SeccompProfile      *SeccompProfile `yaml:"seccompProfile,omitempty" json:"seccompProfile,omitempty"`
	Sysctls             []Sysctl        `yaml:"sysctls,omitempty" json:"sysctls,omitempty"`
}

// ContainerSecurityContext is the container-level security context.
type ContainerSecurityContext struct {
	AllowPrivilegeEscalation *bool           `yaml:"allowPrivilegeEscalation,omitempty" json:"allowPrivilegeEscalation,omitempty"`
	Privileged               *bool           `yaml:"privileged,omitempty" json:"privileged,omitempty"`
	ReadOnlyRootFilesystem   *bool           `yaml:"readOnlyRootFilesystem,omitempty" json:"readOnlyRootFilesystem,omitempty"`
	RunAsUser                *int64          `yaml:"runAsUser,omitempty" json:"runAsUser,omitempty"`
	RunAsGroup               *int64          `yaml:"runAsGroup,omitempty" json:"runAsGroup,omitempty"`
	RunAsNonRoot             *bool           `yaml:"runAsNonRoot,omitempty" json:"runAsNonRoot,omitempty"`
	Capabilities             *Capabilities   `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	SeccompProfile           *SeccompProfile `yaml:"seccompProfile,omitempty" json:"seccompProfile,omitempty"`
}

// SecurityContext is the security context configuration of a service.
type SecurityContext struct {
	Pod       *PodSecurityContext       `yaml:"pod,omitempty" json:"pod,omitempty"`
	Container *ContainerSecurityContext `yaml:"container,omitempty" json:"container,omitempty"`
}

// SecurityContextFrom parses the security-context label value.
func SecurityContextFrom(data string) (*SecurityContext, error) {
	mapping := SecurityContext{}
	if err := yaml.Unmarshal([]byte(data), &mapping); err != nil {
		return nil, err
	}
	return &mapping, nil
}
