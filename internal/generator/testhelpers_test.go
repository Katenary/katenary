package generator

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
)

// updateGolden regenerates the golden files instead of comparing them.
var updateGolden = flag.Bool("update", false, "update golden files")

// serviceBuilder builds a compose service without writing YAML.
type serviceBuilder struct {
	service types.ServiceConfig
}

func newService(name string) *serviceBuilder {
	return &serviceBuilder{service: types.ServiceConfig{
		Name:   name,
		Image:  "nginx:1.29",
		Labels: map[string]string{},
	}}
}

func (b *serviceBuilder) image(image string) *serviceBuilder {
	b.service.Image = image
	return b
}

func (b *serviceBuilder) label(name, value string) *serviceBuilder {
	b.service.Labels[name] = value
	return b
}

func (b *serviceBuilder) env(name, value string) *serviceBuilder {
	if b.service.Environment == nil {
		b.service.Environment = types.MappingWithEquals{}
	}
	b.service.Environment[name] = &value
	return b
}

func (b *serviceBuilder) port(target uint32) *serviceBuilder {
	b.service.Ports = append(b.service.Ports, types.ServicePortConfig{Target: target})
	return b
}

func (b *serviceBuilder) volume(source, target string) *serviceBuilder {
	b.service.Volumes = append(b.service.Volumes, types.ServiceVolumeConfig{
		Type:   types.VolumeTypeVolume,
		Source: source,
		Target: target,
	})
	return b
}

func (b *serviceBuilder) bind(source, target string) *serviceBuilder {
	b.service.Volumes = append(b.service.Volumes, types.ServiceVolumeConfig{
		Type:   types.VolumeTypeBind,
		Source: source,
		Target: target,
	})
	return b
}

func (b *serviceBuilder) dependsOn(names ...string) *serviceBuilder {
	if b.service.DependsOn == nil {
		b.service.DependsOn = types.DependsOnConfig{}
	}
	for _, name := range names {
		b.service.DependsOn[name] = types.ServiceDependency{Condition: types.ServiceConditionStarted}
	}
	return b
}

func (b *serviceBuilder) containerName(name string) *serviceBuilder {
	b.service.ContainerName = name
	return b
}

func (b *serviceBuilder) command(command ...string) *serviceBuilder {
	b.service.Command = command
	return b
}

func (b *serviceBuilder) entrypoint(entrypoint ...string) *serviceBuilder {
	b.service.Entrypoint = entrypoint
	return b
}

func (b *serviceBuilder) healthcheckTest(test ...string) *serviceBuilder {
	if b.service.HealthCheck == nil {
		b.service.HealthCheck = &types.HealthCheckConfig{}
	}
	b.service.HealthCheck.Test = test
	return b
}

func (b *serviceBuilder) build() types.ServiceConfig {
	return b.service
}

// newProject builds a compose project from in-memory services.
func newProject(name string, services ...types.ServiceConfig) *types.Project {
	projectServices := make(types.Services, len(services))
	for _, service := range services {
		projectServices[service.Name] = service
	}
	return &types.Project{Name: name, Services: projectServices}
}

// assertGolden compares got with the golden file at path. Run the test with
// -update to regenerate the file.
func assertGolden(t *testing.T, got []byte, path string) {
	t.Helper()

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("cannot create golden directory: %s", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("cannot write golden file: %s", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read golden file %s: %s (run go test -run %s -update)", path, err, t.Name())
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden file %s does not match (run go test -run %s -update to refresh)", path, t.Name())
		t.Logf("first difference:\n%s", firstDifference(want, got))
	}
}

// firstDifference returns the first differing line between want and got.
func firstDifference(want, got []byte) string {
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var wantLine, gotLine string
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if wantLine != gotLine {
			return fmt.Sprintf("line %d:\n  want: %q\n  got:  %q", i+1, wantLine, gotLine)
		}
	}
	return "no line difference (trailing bytes differ)"
}

// testdataPath returns the path of a file under the testdata directory.
func testdataPath(elem ...string) string {
	return filepath.Join(append([]string{"testdata"}, elem...)...)
}

// testCompose returns the compose fixture path for a test case:
// testdata/<name>/compose.yaml.
func testCompose(name string) string {
	return testdataPath(name, "compose.yaml")
}

// testGolden returns the golden file path for a test case:
// testdata/<name>/golden.yaml.
func testGolden(name string) string {
	return testdataPath(name, "golden.yaml")
}

// convertFixture converts a compose fixture into chartDir.
func convertFixture(t *testing.T, composeFile, chartDir string) {
	t.Helper()

	options := ConvertOptions{
		OutputDir:    chartDir,
		ChartVersion: "0.1.0",
		HelmUpdate:   true,
		Force:        true,
	}
	if err := Convert(options, composeFile); err != nil {
		t.Fatalf("convert failed: %s", err)
	}
}

// templateChart renders a converted chart with helm.
func templateChart(t *testing.T, chartDir string, args ...string) []byte {
	t.Helper()

	output, err := helmTemplate(ConvertOptions{OutputDir: chartDir}, args...)
	if err != nil {
		t.Fatalf("helm template failed: %s\n%s", err, output)
	}
	return []byte(output)
}

// convertAndTemplate converts a compose fixture and renders the whole chart
// with helm. It is meant for the few integration/golden tests that need the
// fully rendered output.
func convertAndTemplate(t *testing.T, composeFile, chartDir string) []byte {
	t.Helper()
	convertFixture(t, composeFile, chartDir)
	return templateChart(t, chartDir)
}

// projectValues generates the chart from an in-memory project and returns the
// rendered values.yaml, without helm.
func projectValues(t *testing.T, project *types.Project) []byte {
	t.Helper()

	chart, err := Generate(project)
	if err != nil {
		t.Fatalf("generate failed: %s", err)
	}

	path := filepath.Join(t.TempDir(), "values.yaml")
	buildValues(chart, project, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read values.yaml: %s", err)
	}
	return data
}

// renderProject generates a chart from an in-memory project and renders it
// with helm. It avoids compose YAML, the working directory and the helm
// lint/dependency steps of Convert.
func renderProject(t *testing.T, project *types.Project, args ...string) []byte {
	t.Helper()

	chart, err := Generate(project)
	if err != nil {
		t.Fatalf("generate failed: %s", err)
	}
	chart.Version = "0.1.0"

	dir := t.TempDir()
	templateDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("cannot create templates directory: %s", err)
	}
	chart.SaveTemplates(templateDir)
	buildCharYamlFile(chart, project, filepath.Join(dir, "Chart.yaml"))
	buildValues(chart, project, filepath.Join(dir, "values.yaml"))
	writeContent(filepath.Join(templateDir, "_helpers.tpl"), []byte(chart.Helper))

	output, err := helmTemplate(ConvertOptions{OutputDir: dir}, args...)
	if err != nil {
		t.Fatalf("helm template failed: %s\n%s", err, output)
	}
	return []byte(output)
}

// renderProjectTemplate renders a single template from an in-memory project.
func renderProjectTemplate(t *testing.T, project *types.Project, template string, args ...string) []byte {
	t.Helper()
	helmArgs := append([]string{"-s", template}, args...)
	return renderProject(t, project, helmArgs...)
}
