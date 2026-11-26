package generator

import (
	"katenary.io/internal/generator/labels"
	"katenary.io/internal/generator/labels/labelstructs"
	"katenary.io/internal/logger"
	"katenary.io/internal/utils"

	"github.com/compose-spec/compose-go/v2/types"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ Yaml = (*NetworkPolicy)(nil)

const defaultDNSNamespace = "kube-system"

// NetworkPolicy is a Kubernetes NetworkPolicy generated from a compose service.
type NetworkPolicy struct {
	*networkingv1.NetworkPolicy `yaml:",inline"`
	service                     *types.ServiceConfig `yaml:"-"`
}

// NewNetworkPolicy creates a NetworkPolicy from the network-policy label. It
// returns nil when the policy is not enabled.
func NewNetworkPolicy(service types.ServiceConfig, chart *HelmChart, project *types.Project) *NetworkPolicy {
	raw, ok := service.Labels[labels.LabelNetworkPolicy]
	if !ok {
		return nil
	}
	config, err := labelstructs.NetworkPolicyFrom(raw)
	if err != nil {
		logger.Fatalf("error parsing the network-policy label: %s", err)
	}
	if !config.Enabled {
		return nil
	}

	appName := chart.Name
	servicePorts := networkPolicyPorts(service)
	policy := &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        utils.TplName(service.Name, appName),
			Labels:      GetLabels(service.Name, appName),
			Annotations: chart.Annotations,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: GetMatchLabels(service.Name, appName),
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
		},
	}

	if boolValue(config.Ingress.FromSameApp, true) {
		policy.Spec.Ingress = append(policy.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{{
				PodSelector: &metav1.LabelSelector{MatchLabels: GetAppSelectorLabels(appName)},
			}},
			Ports: servicePorts,
		})
	}

	if _, hasIngress := service.Labels[labels.LabelIngress]; hasIngress && boolValue(config.Ingress.FromIngress, true) {
		namespaces := config.Ingress.IngressNamespaces
		if len(namespaces) == 0 {
			// No namespace given: allow any pod of the same namespace (the
			// controller is usually in the application namespace in that case).
			policy.Spec.Ingress = append(policy.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{
				From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}}},
				Ports: servicePorts,
			})
		}
		for _, namespace := range namespaces {
			policy.Spec.Ingress = append(policy.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{
				From: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: namespaceSelector(namespace),
				}},
				Ports: servicePorts,
			})
		}
	}

	if len(config.Ingress.ExtraPeers) > 0 {
		policy.Spec.Ingress = append(policy.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{
			From:  toNetworkPolicyPeers(config.Ingress.ExtraPeers),
			Ports: servicePorts,
		})
	}

	if boolValue(config.Egress.AllowDNS, true) {
		dnsNamespace := config.Egress.DNSNamespace
		if dnsNamespace == "" {
			dnsNamespace = defaultDNSNamespace
		}
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{
				NamespaceSelector: namespaceSelector(dnsNamespace),
			}},
			Ports: []networkingv1.NetworkPolicyPort{
				{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 53}, Protocol: protocolPtr(corev1.ProtocolUDP)},
				{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 53}, Protocol: protocolPtr(corev1.ProtocolTCP)},
			},
		})
	}

	if boolValue(config.Egress.AllowSameApp, true) {
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{
				PodSelector: &metav1.LabelSelector{MatchLabels: GetAppSelectorLabels(appName)},
			}},
		})
	}

	if boolValue(config.Egress.AllowDependencies, true) {
		for _, dependency := range service.GetDependencies() {
			target, exists := project.Services[dependency]
			if !exists {
				continue
			}
			policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{MatchLabels: GetMatchLabels(dependency, appName)},
				}},
				Ports: networkPolicyPorts(target),
			})
		}
	}

	if boolValue(config.Egress.AllowKubernetesAPI, true) {
		// The depends_on init containers call the Kubernetes API. Allow HTTPS
		// egress so those checks keep working; disable it when explicit peers
		// are configured.
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
				{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 443}, Protocol: protocolPtr(corev1.ProtocolTCP)},
			},
		})
	}

	if boolValue(config.Egress.AllowInternet, false) {
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
				{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}},
			},
		})
	}

	if len(config.Egress.ExtraPeers) > 0 {
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: toNetworkPolicyPeers(config.Egress.ExtraPeers),
		})
	}

	return &NetworkPolicy{
		NetworkPolicy: policy,
		service:       &service,
	}
}

// Filename returns the filename of the NetworkPolicy template.
func (p *NetworkPolicy) Filename() string {
	return p.service.Name + ".networkpolicy.yaml"
}

// Yaml returns the YAML representation of the NetworkPolicy.
func (p *NetworkPolicy) Yaml() ([]byte, error) {
	y, err := ToK8SYaml(p)
	if err != nil {
		return nil, err
	}
	guard := []byte(`{{- if .Values.` + p.service.Name + `.networkPolicy.enabled -}}` + "\n")
	guard = append(guard, y...)
	guard = append(guard, []byte("\n{{- end -}}\n")...)
	return guard, nil
}

func networkPolicyPorts(service types.ServiceConfig) []networkingv1.NetworkPolicyPort {
	if err := fixPorts(&service); err != nil {
		logger.Warnf("error reading the ports of service %s: %s", service.Name, err)
		return nil
	}
	ports := make([]networkingv1.NetworkPolicyPort, 0, len(service.Ports))
	for _, port := range service.Ports {
		ports = append(ports, networkingv1.NetworkPolicyPort{
			Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: int32(port.Target)},
			Protocol: protocolPtr(corev1.ProtocolTCP),
		})
	}
	return ports
}

func namespaceSelector(namespace string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace},
	}
}

func toNetworkPolicyPeers(peers []labelstructs.NetworkPolicyPeer) []networkingv1.NetworkPolicyPeer {
	result := make([]networkingv1.NetworkPolicyPeer, 0, len(peers))
	for _, peer := range peers {
		result = append(result, networkingv1.NetworkPolicyPeer{
			PodSelector:       toLabelSelector(peer.PodSelector),
			NamespaceSelector: toLabelSelector(peer.NamespaceSelector),
			IPBlock:           toIPBlock(peer.IPBlock),
		})
	}
	return result
}

func toLabelSelector(selector *labelstructs.NetworkPolicyLabelSelector) *metav1.LabelSelector {
	if selector == nil {
		return nil
	}
	result := &metav1.LabelSelector{
		MatchLabels: selector.MatchLabels,
	}
	for _, expression := range selector.MatchExpressions {
		result.MatchExpressions = append(result.MatchExpressions, metav1.LabelSelectorRequirement{
			Key:      expression.Key,
			Operator: metav1.LabelSelectorOperator(expression.Operator),
			Values:   expression.Values,
		})
	}
	return result
}

func toIPBlock(block *labelstructs.NetworkPolicyIPBlock) *networkingv1.IPBlock {
	if block == nil {
		return nil
	}
	return &networkingv1.IPBlock{CIDR: block.CIDR, Except: block.Except}
}

func protocolPtr(protocol corev1.Protocol) *corev1.Protocol {
	return &protocol
}

func boolValue(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}
