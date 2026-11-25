package generator

import (
	"strings"
	"testing"

	"katenary.io/internal/generator/labels"

	"github.com/compose-spec/compose-go/v2/types"
)

func newIngressRouteProject(ingress string) *types.Project {
	service := newService("web").
		image("nginx:1.29").
		port(80).
		label(labels.LabelIngress, ingress).
		build()
	return newProject("testapp", service)
}

func TestIngressRoute(t *testing.T) {
	t.Parallel()
	project := newIngressRouteProject(`
hostname: my.test.tld
port: 80
type: ingressroute
enabled: true`)

	output := renderProjectTemplate(t, project, "templates/web/ingressroute.yaml")

	if !strings.Contains(string(output), "kind: IngressRoute") {
		t.Errorf("expected an IngressRoute, got:\n%s", output)
	}
	if !strings.Contains(string(output), "apiVersion: traefik.io/v1alpha1") {
		t.Errorf("expected traefik.io/v1alpha1, got:\n%s", output)
	}
	if !strings.Contains(string(output), "my.test.tld") {
		t.Errorf("expected host my.test.tld, got:\n%s", output)
	}
}

func TestIngressRouteWithoutTLS(t *testing.T) {
	t.Parallel()
	project := newIngressRouteProject(`
hostname: plain.example.com
port: 80
type: ingressroute
enabled: true
tls:
  enabled: false`)

	output := renderProjectTemplate(t, project, "templates/web/ingressroute.yaml")

	if strings.Contains(string(output), "websecure") {
		t.Errorf("unexpected websecure entrypoint when TLS is disabled:\n%s", output)
	}
	if strings.Contains(string(output), "tls:") {
		t.Errorf("unexpected TLS configuration when TLS is disabled:\n%s", output)
	}
}

func TestIngressRouteDisablesStandardIngress(t *testing.T) {
	t.Parallel()
	project := newIngressRouteProject(`
hostname: my.test.tld
port: 80
type: ingressroute
enabled: true`)

	output := renderProject(t, project)

	if !strings.Contains(string(output), "kind: IngressRoute") {
		t.Errorf("expected an IngressRoute, got:\n%s", output)
	}
	if strings.Contains(string(output), "kind: Ingress\n") {
		t.Errorf("standard Ingress should not be rendered when type is ingressroute:\n%s", output)
	}
}

func TestIngressRouteWithTLS(t *testing.T) {
	t.Parallel()
	project := newIngressRouteProject(`
hostname: secure.example.com
port: 80
type: ingressroute
enabled: true
tls:
  enabled: true`)

	output := renderProjectTemplate(t, project, "templates/web/ingressroute.yaml")

	if !strings.Contains(string(output), "tls:") {
		t.Errorf("expected TLS configuration, got:\n%s", output)
	}
	if !strings.Contains(string(output), "secretName:") {
		t.Errorf("expected a secret name, got:\n%s", output)
	}
	if !strings.Contains(string(output), "websecure") {
		t.Errorf("expected the websecure entrypoint when TLS is enabled, got:\n%s", output)
	}
}

func TestIngressRouteWithPath(t *testing.T) {
	t.Parallel()
	project := newIngressRouteProject(`
hostname: app.example.com
port: 80
path: /api
type: ingressroute
enabled: true`)

	output := renderProjectTemplate(t, project, "templates/web/ingressroute.yaml")

	if !strings.Contains(string(output), "PathPrefix") || !strings.Contains(string(output), "/api") {
		t.Errorf("expected a PathPrefix rule for /api, got:\n%s", output)
	}
}

func TestIngressRouteEntryPoints(t *testing.T) {
	t.Parallel()
	project := newIngressRouteProject(`
hostname: app.example.com
port: 80
type: ingressroute
enabled: true`)

	output := renderProjectTemplate(t, project, "templates/web/ingressroute.yaml")

	if !strings.Contains(string(output), "web") {
		t.Errorf("expected the web entrypoint, got:\n%s", output)
	}
	if !strings.Contains(string(output), "websecure") {
		t.Errorf("expected the websecure entrypoint with the default TLS, got:\n%s", output)
	}
}

func TestIngressRouteDisabled(t *testing.T) {
	t.Parallel()
	project := newIngressRouteProject(`
hostname: app.example.com
port: 80
type: ingressroute
enabled: false`)

	output := renderProject(t, project)

	if strings.Contains(string(output), "kind: IngressRoute") {
		t.Errorf("IngressRoute should not be rendered when disabled:\n%s", output)
	}
}

func TestIngressRouteMetadata(t *testing.T) {
	t.Parallel()
	project := newIngressRouteProject(`
hostname: meta.example.com
port: 80
type: ingressroute
enabled: true`)

	output := renderProjectTemplate(t, project, "templates/web/ingressroute.yaml")

	if !strings.Contains(string(output), "app.kubernetes.io/component") {
		t.Errorf("expected the app.kubernetes.io/component label, got:\n%s", output)
	}
	if !strings.Contains(string(output), "app.kubernetes.io/name") {
		t.Errorf("expected the app.kubernetes.io/name label, got:\n%s", output)
	}
	if strings.Contains(string(output), "katenary.v3/component") {
		t.Errorf("legacy katenary label found in output:\n%s", output)
	}
}
