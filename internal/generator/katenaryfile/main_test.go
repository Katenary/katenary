package katenaryfile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"katenary.io/internal/generator/labels"
	"katenary.io/internal/generator/labels/labelstructs"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
)

// projectWithKatenaryFile parses a compose project and applies a katenary.yaml
// file to it. Both files are written in a temporary directory used as the
// working directory while the project is parsed.
func projectWithKatenaryFile(t *testing.T, composeContent, katenaryContent string) (*types.Project, error) {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(composeContent), 0o644); err != nil {
		t.Fatalf("cannot write compose file: %s", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "katenary.yaml"), []byte(katenaryContent), 0o644); err != nil {
		t.Fatalf("cannot write katenary file: %s", err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(currentDir) //nolint:errcheck

	options, err := cli.NewProjectOptions(nil,
		cli.WithWorkingDirectory(tmpDir),
		cli.WithDefaultConfigPath,
	)
	if err != nil {
		t.Fatalf("cannot create project options: %s", err)
	}
	project, err := cli.ProjectFromOptions(context.TODO(), options)
	if err != nil {
		t.Fatalf("cannot create project from options: %s", err)
	}

	return project, OverrideWithConfig(project)
}

func TestBuildSchema(t *testing.T) {
	sh := GenerateSchema()
	if len(sh) == 0 {
		t.Errorf("Expected schema to be defined")
	}
}

func TestOverrideProjectWithKatenaryFile(t *testing.T) {
	project, err := projectWithKatenaryFile(t, `
services:
  webapp:
    image: nginx:latest
`, `
webapp:
  ports:
    - 80
`)
	if err != nil {
		t.Fatalf("OverrideWithConfig failed: %s", err)
	}

	if _, ok := project.Services["webapp"].Labels[labels.LabelPorts]; !ok {
		t.Fatal("Expected ports to be defined")
	}
}

func TestOverrideProjectWithIngress(t *testing.T) {
	project, err := projectWithKatenaryFile(t, `
services:
  webapp:
    image: nginx:latest
`, `
webapp:
  ports:
    - 80
  ingress:
    port: 80
`)
	if err != nil {
		t.Fatalf("OverrideWithConfig failed: %s", err)
	}

	serviceLabels := project.Services["webapp"].Labels
	if _, ok := serviceLabels[labels.LabelPorts]; !ok {
		t.Fatal("Expected ports to be defined")
	}
	if _, ok := serviceLabels[labels.LabelIngress]; !ok {
		t.Fatal("Expected ingress to be defined")
	}
}

func TestOverrideConfigMapFiles(t *testing.T) {
	project, err := projectWithKatenaryFile(t, `
services:
  webapp:
    image: nginx:latest
`, `
webapp:
  configmap-files:
    - foo/bar
  ports:
    - 80
  ingress:
    port: 80
`)
	if err != nil {
		t.Fatalf("OverrideWithConfig failed: %s", err)
	}

	if _, ok := project.Services["webapp"].Labels[labels.LabelConfigMapFiles]; !ok {
		t.Fatal("Expected configmap-files to be defined")
	}
}

func TestOverrideProjectWithCondition(t *testing.T) {
	project, err := projectWithKatenaryFile(t, `
services:
  webapp:
    image: nginx:latest
  worker:
    image: busybox:latest
`, `
webapp:
  if: proxy.enabled
worker:
  if:
    condition: proxy.enabled && !debug.enabled
    defaults:
      debug.enabled: false
`)
	if err != nil {
		t.Fatalf("OverrideWithConfig failed: %s", err)
	}

	tests := []struct {
		service string
		want    string
	}{
		{service: "webapp", want: ".Values.proxy.enabled"},
		{service: "worker", want: "and .Values.proxy.enabled (not .Values.debug.enabled)"},
	}

	for _, test := range tests {
		t.Run(test.service, func(t *testing.T) {
			raw, ok := project.Services[test.service].Labels[labels.LabelIf]
			if !ok {
				t.Fatalf("Expected if to be defined for %s", test.service)
			}
			condition, err := labelstructs.ConditionFrom(raw)
			if err != nil {
				t.Fatalf("ConditionFrom failed: %s", err)
			}
			if got := condition.HelmExpr(); got != test.want {
				t.Errorf("HelmExpr() = %q, want %q", got, test.want)
			}
		})
	}

	// The default provided in katenary.yaml must survive the round-trip.
	raw := project.Services["worker"].Labels[labels.LabelIf]
	condition, err := labelstructs.ConditionFrom(raw)
	if err != nil {
		t.Fatalf("ConditionFrom failed: %s", err)
	}
	if condition.Default("debug.enabled") {
		t.Error("debug.enabled should default to false")
	}
}

func TestOverrideProjectWithInvalidCondition(t *testing.T) {
	_, err := projectWithKatenaryFile(t, `
services:
  webapp:
    image: nginx:latest
`, `
webapp:
  if:
    condition: proxy.enabled
    expr: typo
`)
	if err == nil {
		t.Fatal("expected an error for an unknown field in the if object")
	}
	if !strings.Contains(err.Error(), `unknown field "expr"`) {
		t.Errorf("unexpected error message: %s", err)
	}
}

func TestGetLabelContentWithCondition(t *testing.T) {
	service := types.ServiceConfig{}
	label := &labelstructs.ConditionLabel{Condition: "proxy.enabled && !debug.enabled"}

	if err := getLabelContent(label, &service, labels.LabelIf); err != nil {
		t.Fatalf("getLabelContent failed: %s", err)
	}

	raw, ok := service.Labels[labels.LabelIf]
	if !ok {
		t.Fatal("the if label was not set")
	}
	condition, err := labelstructs.ConditionFrom(raw)
	if err != nil {
		t.Fatalf("ConditionFrom failed on the generated label: %s", err)
	}
	if got := condition.HelmExpr(); got != "and .Values.proxy.enabled (not .Values.debug.enabled)" {
		t.Errorf("HelmExpr() = %q", got)
	}
}

func TestOverrideProjectWithLegacyLabels(t *testing.T) {
	_, err := projectWithKatenaryFile(t, `
services:
  webapp:
    image: nginx:latest
`, `
webapp:
  katenary.v3/ports:
    - 80
`)
	if err == nil {
		t.Fatal("expected an error for legacy labels")
	}
	if !strings.Contains(err.Error(), "katenary.v3/ports") || !strings.Contains(err.Error(), "katenary.v4/ports") {
		t.Errorf("unexpected error message: %s", err)
	}
}
