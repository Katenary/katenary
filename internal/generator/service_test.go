package generator

import (
	"testing"
)

func TestNewService(t *testing.T) {
	t.Parallel()
	service := newService("web").port(80).port(443).build()

	kubeService := NewService(service, NewChart("testapp"))

	if len(kubeService.Spec.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(kubeService.Spec.Ports))
	}
	found := 0
	for _, port := range kubeService.Spec.Ports {
		if port.Port == 80 && port.TargetPort.StrVal == "http" {
			found++
		}
		if port.Port == 443 && port.TargetPort.StrVal == "https" {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected http and https ports, got %+v", kubeService.Spec.Ports)
	}
}

func TestNewServiceWithUnknownPorts(t *testing.T) {
	t.Parallel()
	service := newService("multi").image("nginx").port(12443).port(12480).build()

	kubeService := NewService(service, NewChart("testapp"))

	if len(kubeService.Spec.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(kubeService.Spec.Ports))
	}
	if kubeService.Spec.Ports[0].Name == kubeService.Spec.Ports[1].Name {
		t.Errorf("expected different port names, got %s and %s", kubeService.Spec.Ports[0].Name, kubeService.Spec.Ports[1].Name)
	}
	for _, port := range kubeService.Spec.Ports {
		if port.Name != "port-12443" && port.Name != "port-12480" {
			t.Errorf("unexpected port name %q", port.Name)
		}
	}
}
