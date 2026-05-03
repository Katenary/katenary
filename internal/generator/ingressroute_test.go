package generator

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"katenary.io/internal/generator/labels"
)

func TestIngressRoute(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        labels:
            %s/ingress: |-
                hostname: my.test.tld
                port: 80
                type: ingressroute
                enabled: true
 `
	composeFile = fmt.Sprintf(composeFile, labels.KatenaryLabelPrefix)
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	// Test that the ingressroute file is generated
	output := internalCompileTest(
		t,
		"-s", "templates/web/ingressroute.yaml",
	)

	// Check that it's a Traefik IngressRoute
	if !strings.Contains(output, "kind: IngressRoute") {
		t.Errorf("Expected IngressRoute kind in output")
	}
	if !strings.Contains(output, "apiVersion: traefik.io/v1alpha1") {
		t.Errorf("Expected traefik.io/v1alpha1 apiVersion in output")
	}
	if !strings.Contains(output, "my.test.tld") {
		t.Errorf("Expected host my.test.tld in output")
	}
}

func TestIngressRouteWithTLS(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 443:443
        labels:
            %s/ingress: |-
                hostname: secure.example.com
                port: 443
                type: ingressroute
                enabled: true
                tls:
                  enabled: true
 `
	composeFile = fmt.Sprintf(composeFile, labels.KatenaryLabelPrefix)
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(
		t,
		"-s", "templates/web/ingressroute.yaml",
	)

	// Check TLS configuration
	if !strings.Contains(output, "tls:") {
		t.Errorf("Expected TLS configuration in IngressRoute, got: %s", output)
	}
	if !strings.Contains(output, "secretName:") {
		t.Errorf("Expected SecretName in TLS configuration, got: %s", output)
	}
}

func TestIngressRouteWithPath(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        labels:
            %s/ingress: |-
                hostname: app.example.com
                port: 80
                path: /api
                type: ingressroute
                enabled: true
 `
	composeFile = fmt.Sprintf(composeFile, labels.KatenaryLabelPrefix)
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(
		t,
		"-s", "templates/web/ingressroute.yaml",
	)

	// Check path prefix in match rule
	if !strings.Contains(output, "PathPrefix") {
		t.Errorf("Expected PathPrefix in match rule")
	}
	if !strings.Contains(output, "/api") {
		t.Errorf("Expected path /api in match rule")
	}
}

func TestIngressRouteEntryPoints(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        labels:
            %s/ingress: |-
                hostname: app.example.com
                port: 80
                type: ingressroute
                enabled: true
 `
	composeFile = fmt.Sprintf(composeFile, labels.KatenaryLabelPrefix)
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(
		t,
		"-s", "templates/web/ingressroute.yaml",
	)

	// Check default entryPoints
	if !strings.Contains(output, "web") {
		t.Errorf("Expected 'web' entryPoint in IngressRoute")
	}
	if !strings.Contains(output, "websecure") {
		t.Errorf("Expected 'websecure' entryPoint in IngressRoute")
	}
}

func TestIngressRouteDisabled(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        labels:
            %s/ingress: |-
                hostname: app.example.com
                port: 80
                type: ingressroute
                enabled: false
 `
	composeFile = fmt.Sprintf(composeFile, labels.KatenaryLabelPrefix)
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	// When ingress is disabled, the file is generated but the helm template
	// with -s flag will fail because output is empty due to conditional
	// Instead, just verify the chart is valid by running helm template without -s
	// The chart should lint successfully
	output := internalCompileTest(
		t,
	)

	// The output should not contain IngressRoute kind since it's disabled
	if strings.Contains(output, "kind: IngressRoute") {
		t.Errorf("IngressRoute should not be in output when disabled")
	}
}

func TestIngressRouteMetadata(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        labels:
            %s/ingress: |-
                hostname: meta.example.com
                port: 80
                type: ingressroute
                enabled: true
 `
	composeFile = fmt.Sprintf(composeFile, labels.KatenaryLabelPrefix)
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(
		t,
		"-s", "templates/web/ingressroute.yaml",
	)

	// Check that labels are present (like other objects - no metadata: block)
	if !strings.Contains(output, "labels:") {
		t.Errorf("Expected labels: in IngressRoute output")
	}

	// Check that katenary labels are present (set by GetLabels)
	if !strings.Contains(output, "katenary.v3/component") {
		t.Errorf("Expected katenary.v3/component label in IngressRoute")
	}

	// Check that the standard labels are present
	if !strings.Contains(output, "app.kubernetes.io/name") {
		t.Errorf("Expected app.kubernetes.io/name label in IngressRoute")
	}
}
