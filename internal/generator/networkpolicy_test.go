package generator

import (
	"testing"

	"katenary.io/internal/generator/labels"
)

func TestNetworkPolicyDisabledByDefault(t *testing.T) {
	project := newProject("testapp", newService("web").image("nginx").port(80).build())
	policy := NewNetworkPolicy(project.Services["web"], NewChart("testapp"), project)
	if policy != nil {
		t.Error("expected no NetworkPolicy without the label")
	}
}

func TestNetworkPolicyFromBooleanLabel(t *testing.T) {
	service := newService("web").image("nginx").port(80).
		label(labels.LabelNetworkPolicy, "true").build()
	project := newProject("testapp", service)

	policy := NewNetworkPolicy(service, NewChart("testapp"), project)
	if policy == nil {
		t.Fatal("expected a NetworkPolicy")
	}
	if len(policy.Spec.PolicyTypes) != 2 {
		t.Errorf("expected ingress and egress policy types, got %v", policy.Spec.PolicyTypes)
	}
}

func TestNetworkPolicyRules(t *testing.T) {
	web := newService("web").
		image("nginx").
		port(80).
		dependsOn("database").
		label(labels.LabelIngress, "hostname: my.test.tld\nport: 80\nenabled: true").
		label(labels.LabelNetworkPolicy, `
enabled: true
ingress:
  ingressNamespaces: [traefik]
egress:
  allowInternet: true`).
		build()
	database := newService("database").image("postgres").port(5432).
		label(labels.LabelNetworkPolicy, "enabled: true").build()
	project := newProject("testapp", web, database)

	policy := NewNetworkPolicy(project.Services["web"], NewChart("testapp"), project)
	if policy == nil {
		t.Fatal("expected a NetworkPolicy")
	}

	// same app + ingress controller
	if len(policy.Spec.Ingress) != 2 {
		t.Fatalf("expected 2 ingress rules, got %d", len(policy.Spec.Ingress))
	}
	if len(policy.Spec.Ingress[0].From) != 1 || policy.Spec.Ingress[0].From[0].PodSelector == nil {
		t.Error("expected a same-app pod selector in the first ingress rule")
	}
	namespaceSelector := policy.Spec.Ingress[1].From[0].NamespaceSelector
	if namespaceSelector == nil || namespaceSelector.MatchLabels["kubernetes.io/metadata.name"] != "traefik" {
		t.Errorf("expected the traefik namespace selector, got %+v", namespaceSelector)
	}

	// DNS, same app, dependency, Kubernetes API and internet
	if len(policy.Spec.Egress) != 5 {
		t.Fatalf("expected 5 egress rules, got %d", len(policy.Spec.Egress))
	}
	if policy.Spec.Egress[0].To[0].NamespaceSelector == nil {
		t.Error("expected a DNS namespace selector")
	}
	if policy.Spec.Egress[1].To[0].PodSelector == nil {
		t.Error("expected a same-app pod selector")
	}
	dependency := policy.Spec.Egress[2].To[0].PodSelector
	if dependency == nil {
		t.Error("expected a dependency pod selector")
	}
	if len(policy.Spec.Egress[2].Ports) != 1 || policy.Spec.Egress[2].Ports[0].Port == nil ||
		policy.Spec.Egress[2].Ports[0].Port.IntValue() != 5432 {
		t.Errorf("unexpected dependency ports: %+v", policy.Spec.Egress[2].Ports)
	}
	api := policy.Spec.Egress[3]
	if len(api.Ports) != 1 || api.Ports[0].Port.IntValue() != 443 {
		t.Errorf("expected the Kubernetes API HTTPS rule, got %+v", api)
	}
	internet := policy.Spec.Egress[4].To
	if len(internet) != 2 || internet[0].IPBlock == nil || internet[0].IPBlock.CIDR != "0.0.0.0/0" {
		t.Errorf("expected the internet IP blocks, got %+v", internet)
	}
}

func TestNetworkPolicyValues(t *testing.T) {
	service := newService("web").image("nginx").port(80).
		label(labels.LabelNetworkPolicy, "enabled: true").build()

	value := NewValue(service)
	if value.NetworkPolicy == nil || !value.NetworkPolicy.Enabled {
		t.Error("expected the network policy value to be enabled")
	}
}
