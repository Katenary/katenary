package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"katenary.io/internal/generator/labels"

	"github.com/compose-spec/compose-go/v2/types"
)

const apiConditionLabel = `condition: proxy.enabled && !maintenance.enabled
defaults:
  proxy.enabled: true
  maintenance.enabled: false
`

func conditionalTestProject() *types.Project {
	return newProject("cond",
		newService("traefik").
			port(80).
			label(labels.LabelIf, "proxy.enabled").
			build(),
		newService("api").
			port(8080).
			label(labels.LabelIf, apiConditionLabel).
			label(labels.LabelIngress, "hostname: api.test.tld\nport: 8080\nenabled: true").
			build(),
	)
}

func TestConditionalResourcesAreWrapped(t *testing.T) {
	chart, err := Generate(conditionalTestProject())
	if err != nil {
		t.Fatalf("generate failed: %s", err)
	}

	tests := []struct {
		template string
		guard    string
	}{
		{template: "traefik.deployment.yaml", guard: "{{- if .Values.proxy.enabled }}"},
		{template: "traefik.service.yaml", guard: "{{- if .Values.proxy.enabled }}"},
		{template: "api.deployment.yaml", guard: "{{- if and .Values.proxy.enabled (not .Values.maintenance.enabled) }}"},
		{template: "api.service.yaml", guard: "{{- if and .Values.proxy.enabled (not .Values.maintenance.enabled) }}"},
		{template: "api.ingress.yaml", guard: "{{- if and .Values.proxy.enabled (not .Values.maintenance.enabled) }}"},
	}

	for _, test := range tests {
		t.Run(test.template, func(t *testing.T) {
			template, ok := chart.Templates[test.template]
			if !ok {
				t.Fatalf("template %s not generated", test.template)
			}
			content := string(template.Content)
			if !strings.HasPrefix(content, test.guard+"\n") {
				t.Errorf("template does not start with %q:\n%s", test.guard, content)
			}
			if !strings.HasSuffix(strings.TrimSpace(content), "{{- end }}") {
				t.Errorf("template does not end with the condition guard:\n%s", content)
			}
		})
	}
}

func TestConditionDefaultsInValues(t *testing.T) {
	values := string(projectValues(t, conditionalTestProject()))

	for _, expected := range []string{
		"# Conditional activation flags used by the katenary.v4/if label.",
		"#   proxy.enabled (default: true)",
		"#   maintenance.enabled (default: false)",
		"\nproxy:\n",
		"\nmaintenance:\n",
		"\n  enabled: true\n",
		"\n  enabled: false\n",
	} {
		if !strings.Contains(values, expected) {
			t.Errorf("values.yaml does not contain %q:\n%s", expected, values)
		}
	}
}

func TestConditionalResourcesRendering(t *testing.T) {
	output := string(renderProject(t, conditionalTestProject()))
	for _, name := range []string{"release-name-cond-traefik", "release-name-cond-api"} {
		if !strings.Contains(output, name) {
			t.Errorf("enabled output does not contain %s", name)
		}
	}

	// Disabling the shared flag removes every resource of both services.
	output = string(renderProject(t, conditionalTestProject(), "--set", "proxy.enabled=false"))
	for _, name := range []string{"release-name-cond-traefik", "release-name-cond-api"} {
		if strings.Contains(output, name) {
			t.Errorf("disabled output still contains %s", name)
		}
	}

	// The second flag only disables the api service.
	output = string(renderProject(t, conditionalTestProject(), "--set", "maintenance.enabled=true"))
	if strings.Contains(output, "release-name-cond-api") {
		t.Error("api should be disabled when maintenance is enabled")
	}
	if !strings.Contains(output, "release-name-cond-traefik") {
		t.Error("traefik should stay enabled when maintenance is enabled")
	}
}

func TestConditionSamePodUsesTargetCondition(t *testing.T) {
	project := newProject("cond",
		newService("app").
			port(80).
			label(labels.LabelIf, "flags.app.enabled").
			build(),
		newService("sidecar").
			label(labels.LabelSamePod, "app").
			label(labels.LabelIf, "sidecar.enabled").
			volume("data", "/data").
			build(),
	)

	chart, err := Generate(project)
	if err != nil {
		t.Fatalf("generate failed: %s", err)
	}

	pvc, ok := chart.Templates["sidecar.data.volumeclaim.yaml"]
	if !ok {
		t.Fatal("the PVC of the merged service was not generated")
	}
	if !strings.HasPrefix(string(pvc.Content), "{{- if .Values.flags.app.enabled }}\n") {
		t.Errorf("the PVC should follow the target condition:\n%s", pvc.Content)
	}
	if _, ok := chart.ConditionFlags["sidecar.enabled"]; ok {
		t.Error("the ignored condition flags should not be injected in values.yaml")
	}
}

func TestConditionFromKatenaryFile(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, "compose.yaml")
	katenaryFile := filepath.Join(dir, "katenary.yaml")

	compose := `name: condk
services:
  webapp:
    image: nginx:1.29
    ports:
      - "80:80"
`
	katenary := `webapp:
  if:
    condition: proxy.enabled && !debug.enabled
    defaults:
      debug.enabled: false
`
	if err := os.WriteFile(composeFile, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(katenaryFile, []byte(katenary), 0o644); err != nil {
		t.Fatal(err)
	}

	chartDir := filepath.Join(dir, "chart")
	convertFixture(t, composeFile, chartDir)

	deployment, err := os.ReadFile(filepath.Join(chartDir, "templates", "webapp", "deployment.yaml"))
	if err != nil {
		t.Fatalf("cannot read the deployment template: %s", err)
	}
	guard := "{{- if and .Values.proxy.enabled (not .Values.debug.enabled) }}"
	if !strings.HasPrefix(string(deployment), guard+"\n") {
		t.Errorf("the deployment should start with %q:\n%s", guard, deployment)
	}

	values, err := os.ReadFile(filepath.Join(chartDir, "values.yaml"))
	if err != nil {
		t.Fatalf("cannot read values.yaml: %s", err)
	}
	for _, expected := range []string{
		"#   proxy.enabled (default: true)",
		"#   debug.enabled (default: false)",
		"\n  enabled: false\n",
	} {
		if !strings.Contains(string(values), expected) {
			t.Errorf("values.yaml does not contain %q:\n%s", expected, values)
		}
	}

	if output := string(templateChart(t, chartDir, "--set", "proxy.enabled=false")); strings.Contains(output, "release-name-condk-webapp") {
		t.Error("the webapp resources should be disabled when proxy.enabled is false")
	}
}

func TestConditionPathConflict(t *testing.T) {
	project := newProject("cond",
		newService("proxy").port(80).build(),
		newService("app").port(8080).label(labels.LabelIf, "proxy.enabled").build(),
	)

	_, err := Generate(project)
	if err == nil {
		t.Fatal("generate should fail when the flag collides with a service name")
	}
	if !strings.Contains(err.Error(), `values path "proxy.enabled" conflicts`) {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestConditionInvalidExpression(t *testing.T) {
	project := newProject("cond",
		newService("app").port(8080).label(labels.LabelIf, "a &&").build(),
	)

	_, err := Generate(project)
	if err == nil {
		t.Fatal("generate should fail with an invalid condition")
	}
	if !strings.Contains(err.Error(), "invalid katenary.v4/if label") {
		t.Errorf("unexpected error: %s", err)
	}
}
