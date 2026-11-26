package labelstructs

import (
	"gopkg.in/yaml.v3"
)

// NetworkPolicyLabelSelector is a Kubernetes label selector.
type NetworkPolicyLabelSelector struct {
	MatchLabels      map[string]string                       `yaml:"matchLabels,omitempty" json:"matchLabels,omitempty"`
	MatchExpressions []NetworkPolicyLabelSelectorRequirement `yaml:"matchExpressions,omitempty" json:"matchExpressions,omitempty"`
}

// NetworkPolicyLabelSelectorRequirement is a label selector expression.
type NetworkPolicyLabelSelectorRequirement struct {
	Key      string   `yaml:"key" json:"key"`
	Operator string   `yaml:"operator" json:"operator"`
	Values   []string `yaml:"values,omitempty" json:"values,omitempty"`
}

// NetworkPolicyIPBlock is an IP block peer.
type NetworkPolicyIPBlock struct {
	CIDR   string   `yaml:"cidr" json:"cidr"`
	Except []string `yaml:"except,omitempty" json:"except,omitempty"`
}

// NetworkPolicyPeer is a NetworkPolicy peer.
type NetworkPolicyPeer struct {
	PodSelector       *NetworkPolicyLabelSelector `yaml:"podSelector,omitempty" json:"podSelector,omitempty"`
	NamespaceSelector *NetworkPolicyLabelSelector `yaml:"namespaceSelector,omitempty" json:"namespaceSelector,omitempty"`
	IPBlock           *NetworkPolicyIPBlock       `yaml:"ipBlock,omitempty" json:"ipBlock,omitempty"`
}

// NetworkPolicyIngress is the ingress configuration of a NetworkPolicy.
type NetworkPolicyIngress struct {
	FromSameApp       *bool               `yaml:"fromSameApp,omitempty" json:"fromSameApp,omitempty"`
	FromIngress       *bool               `yaml:"fromIngress,omitempty" json:"fromIngress,omitempty"`
	IngressNamespaces []string            `yaml:"ingressNamespaces,omitempty" json:"ingressNamespaces,omitempty"`
	ExtraPeers        []NetworkPolicyPeer `yaml:"extraPeers,omitempty" json:"extraPeers,omitempty"`
}

// NetworkPolicyEgress is the egress configuration of a NetworkPolicy.
type NetworkPolicyEgress struct {
	AllowDNS           *bool               `yaml:"allowDNS,omitempty" json:"allowDNS,omitempty"`
	AllowSameApp       *bool               `yaml:"allowSameApp,omitempty" json:"allowSameApp,omitempty"`
	AllowDependencies  *bool               `yaml:"allowDependencies,omitempty" json:"allowDependencies,omitempty"`
	AllowInternet      *bool               `yaml:"allowInternet,omitempty" json:"allowInternet,omitempty"`
	AllowKubernetesAPI *bool               `yaml:"allowKubernetesAPI,omitempty" json:"allowKubernetesAPI,omitempty"`
	DNSNamespace       string              `yaml:"dnsNamespace,omitempty" json:"dnsNamespace,omitempty"`
	ExtraPeers         []NetworkPolicyPeer `yaml:"extraPeers,omitempty" json:"extraPeers,omitempty"`
}

// NetworkPolicy is the network policy configuration of a service.
type NetworkPolicy struct {
	Enabled bool                 `yaml:"enabled" json:"enabled"`
	Ingress NetworkPolicyIngress `yaml:"ingress,omitempty" json:"ingress,omitempty"`
	Egress  NetworkPolicyEgress  `yaml:"egress,omitempty" json:"egress,omitempty"`
}

// NetworkPolicyFrom parses the network-policy label value. A plain boolean is
// accepted as a shortcut for `enabled: <bool>`.
func NetworkPolicyFrom(data string) (*NetworkPolicy, error) {
	var raw any
	if err := yaml.Unmarshal([]byte(data), &raw); err != nil {
		return nil, err
	}
	if enabled, ok := raw.(bool); ok {
		return &NetworkPolicy{Enabled: enabled}, nil
	}
	mapping := NetworkPolicy{}
	if err := yaml.Unmarshal([]byte(data), &mapping); err != nil {
		return nil, err
	}
	return &mapping, nil
}
