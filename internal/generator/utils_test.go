package generator

import (
	"testing"

	"katenary.io/internal/generator/labels"

	"github.com/compose-spec/compose-go/v2/types"
)

func TestHasExposedPorts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		service types.ServiceConfig
		want    bool
	}{
		{
			name:    "no ports",
			service: types.ServiceConfig{},
			want:    false,
		},
		{
			name:    "compose ports",
			service: types.ServiceConfig{Ports: []types.ServicePortConfig{{Target: 80}}},
			want:    true,
		},
		{
			name:    "ports label as list",
			service: types.ServiceConfig{Labels: map[string]string{labels.LabelPorts: "[80, 443]"}},
			want:    true,
		},
		{
			name:    "ports label as comma separated string",
			service: types.ServiceConfig{Labels: map[string]string{labels.LabelPorts: "80,443"}},
			want:    true,
		},
		{
			name:    "empty ports label",
			service: types.ServiceConfig{Labels: map[string]string{labels.LabelPorts: ""}},
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasExposedPorts(tt.service); got != tt.want {
				t.Errorf("HasExposedPorts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitPorts(t *testing.T) {
	t.Parallel()
	service := newService("foo").
		image("nginx:latest").
		label(labels.LabelPorts, "80,443").
		build()

	if err := fixPorts(&service); err != nil {
		t.Fatalf("fixPorts failed: %s", err)
	}

	if len(service.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(service.Ports))
	}
	found := 0
	for _, port := range service.Ports {
		if port.Target == 80 || port.Target == 443 {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected ports 80 and 443, got %+v", service.Ports)
	}
}

func TestSplitPortsWithDefault(t *testing.T) {
	t.Parallel()
	service := newService("foo").
		image("nginx:latest").
		port(8080).
		label(labels.LabelPorts, "80,443").
		build()

	if err := fixPorts(&service); err != nil {
		t.Fatalf("fixPorts failed: %s", err)
	}

	if len(service.Ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(service.Ports))
	}
	found := 0
	for _, port := range service.Ports {
		if port.Target == 80 || port.Target == 443 || port.Target == 8080 {
			found++
		}
	}
	if found != 3 {
		t.Errorf("expected ports 8080, 80 and 443, got %+v", service.Ports)
	}
}
