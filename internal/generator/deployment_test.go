package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"katenary.io/internal/generator/labels"

	"github.com/compose-spec/compose-go/v2/types"
	yaml3 "gopkg.in/yaml.v3"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"
)

const webTemplateOutput = `templates/web/deployment.yaml`

// TestNewDeployment is a unit test: it builds the Deployment directly from an
// in-memory service, without compose YAML, disk access or helm.
func TestNewDeployment(t *testing.T) {
	t.Parallel()
	chart := NewChart("testapp")
	service := newService("web").image("nginx:1.29").build()

	deployment := NewDeployment(service, chart)

	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %v", deployment.Spec.Replicas)
	}
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(deployment.Spec.Template.Spec.Containers))
	}

	value, ok := chart.Values["web"].(*Value)
	if !ok {
		t.Fatal("expected a value for service web")
	}
	if value.Repository.Image != "nginx" {
		t.Errorf("expected image nginx, got %s", value.Repository.Image)
	}
	if value.Repository.Tag != "1.29" {
		t.Errorf("expected tag 1.29, got %s", value.Repository.Tag)
	}
}

// TestSamePodChartGolden is an integration test: the compose fixture is
// converted once and the fully rendered chart is compared to a golden file.
func TestSamePodChartGolden(t *testing.T) {
	chartDir := filepath.Join(t.TempDir(), "chart")

	output := convertAndTemplate(t, testCompose("same-pod"), chartDir)
	assertGolden(t, output, testGolden("same-pod"))
}

func TestDependsOn(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").image("nginx:1.29").port(80).dependsOn("database").build(),
		newService("database").image("mariadb:10.5").port(3306).build(),
	)

	output := renderProjectTemplate(t, project, webTemplateOutput)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(deployment.Spec.Template.Spec.Containers))
	}
	if len(deployment.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(deployment.Spec.Template.Spec.InitContainers))
	}

	initContainer := deployment.Spec.Template.Spec.InitContainers[0]
	if !strings.Contains(initContainer.Image, "quay.io/curl/curl") {
		t.Errorf("expected quay.io/curl/curl image, got %s", initContainer.Image)
	}

	fullCommand := strings.Join(initContainer.Command, " ")
	for _, want := range []string{"curl", "/apis/apps/v1/namespaces/", "/deployments/", "readyReplicas"} {
		if !strings.Contains(fullCommand, want) {
			t.Errorf("expected %q in the init container command, got %s", want, fullCommand)
		}
	}
	if len(initContainer.Env) == 0 {
		t.Error("expected environment variables for the namespace")
	}

	hasNamespace := false
	for _, env := range initContainer.Env {
		if env.Name == "NAMESPACE" && env.ValueFrom != nil && env.ValueFrom.FieldRef != nil &&
			env.ValueFrom.FieldRef.FieldPath == "metadata.namespace" {
			hasNamespace = true
			break
		}
	}
	if !hasNamespace {
		t.Error("expected NAMESPACE env var with metadata.namespace fieldRef")
	}
}

func TestDependsOnLegacy(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("nginx:1.29").
			port(80).
			dependsOn("database").
			label(labels.LabelDependsOn, "legacy").
			build(),
		newService("database").image("mariadb:10.5").port(3306).build(),
	)

	output := renderProjectTemplate(t, project, webTemplateOutput)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	if len(deployment.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(deployment.Spec.Template.Spec.InitContainers))
	}
	initContainer := deployment.Spec.Template.Spec.InitContainers[0]
	if !strings.Contains(initContainer.Image, "busybox") {
		t.Errorf("expected busybox image, got %s", initContainer.Image)
	}
	if !strings.Contains(strings.Join(initContainer.Command, " "), "nc") {
		t.Errorf("expected nc (netcat) for the legacy method, got %v", initContainer.Command)
	}
}

func TestHelmDependencies(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").image("nginx:1.29").port(80).build(),
		newService("mariadb").
			image("mariadb:10.5").
			port(3306).
			label(labels.LabelDependencies, `
- name: mariadb
  repository: oci://registry-1.docker.io/bitnamicharts
  version: 18.x.X`).
			build(),
	)

	chart, err := Generate(project)
	if err != nil {
		t.Fatalf("generate failed: %s", err)
	}
	if _, ok := chart.Templates["mariadb.deployment.yaml"]; ok {
		t.Error("a helm dependency service must not have a deployment")
	}

	chart.Version = "0.1.0"
	chartFile := filepath.Join(t.TempDir(), "Chart.yaml")
	buildCharYamlFile(chart, project, chartFile)

	content, err := os.ReadFile(chartFile)
	if err != nil {
		t.Fatalf("cannot read Chart.yaml: %s", err)
	}
	helmChart := HelmChart{}
	if err := yaml.Unmarshal(content, &helmChart); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	if len(helmChart.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(helmChart.Dependencies))
	}
}

func TestReadinessProbesFromHealthCheck(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("nginx:1.29").
			port(80).
			healthcheckTest("CMD", "curl", "-f", "http://localhost").
			build(),
	)

	output := renderProjectTemplate(t, project, webTemplateOutput)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	container := deployment.Spec.Template.Spec.Containers[0]
	if container.ReadinessProbe == nil {
		t.Fatal("expected a readiness probe")
	}
	if container.LivenessProbe != nil {
		t.Error("a compose healthcheck must not create a liveness probe")
	}
	if command := container.ReadinessProbe.Exec.Command; len(command) != 3 || command[0] != "curl" {
		t.Errorf("unexpected probe command: %v", command)
	}
}

func TestReadinessProbeFromShellHealthCheck(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("postgres:17").
			port(5432).
			healthcheckTest("CMD-SHELL", "pg_isready -U postgres").
			build(),
	)

	output := renderProjectTemplate(t, project, webTemplateOutput)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	probe := deployment.Spec.Template.Spec.Containers[0].ReadinessProbe
	if probe == nil {
		t.Fatal("expected a readiness probe")
	}
	command := probe.Exec.Command
	if len(command) != 3 || command[0] != "/bin/sh" || command[1] != "-c" || command[2] != "pg_isready -U postgres" {
		t.Errorf("unexpected probe command: %v", command)
	}
}

func TestHealthCheckSettings(t *testing.T) {
	t.Parallel()

	interval := types.Duration(5 * time.Second)
	timeout := types.Duration(2 * time.Second)
	startPeriod := types.Duration(10 * time.Second)
	retries := uint64(5)

	service := newService("web").
		image("nginx:1.29").
		port(80).
		healthcheckTest("CMD", "true").
		build()
	service.HealthCheck.Interval = &interval
	service.HealthCheck.Timeout = &timeout
	service.HealthCheck.StartPeriod = &startPeriod
	service.HealthCheck.Retries = &retries

	output := renderProjectTemplate(t, newProject("testapp", service), webTemplateOutput)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	probe := deployment.Spec.Template.Spec.Containers[0].ReadinessProbe
	if probe == nil {
		t.Fatal("expected a readiness probe")
	}
	if probe.PeriodSeconds != 5 {
		t.Errorf("expected period 5, got %d", probe.PeriodSeconds)
	}
	if probe.TimeoutSeconds != 2 {
		t.Errorf("expected timeout 2, got %d", probe.TimeoutSeconds)
	}
	if probe.InitialDelaySeconds != 10 {
		t.Errorf("expected initial delay 10, got %d", probe.InitialDelaySeconds)
	}
	if probe.FailureThreshold != 5 {
		t.Errorf("expected failure threshold 5, got %d", probe.FailureThreshold)
	}
}

func TestHealthCheckDisabledOrNone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		service types.ServiceConfig
	}{
		{
			name: "none",
			service: newService("web").
				image("nginx:1.29").
				healthcheckTest("NONE").
				build(),
		},
		{
			name: "disabled",
			service: func() types.ServiceConfig {
				service := newService("web").
					image("nginx:1.29").
					healthcheckTest("CMD", "true").
					build()
				service.HealthCheck.Disable = true
				return service
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := renderProjectTemplate(t, newProject("testapp", test.service), webTemplateOutput)
			deployment := v1.Deployment{}
			if err := yaml.Unmarshal(output, &deployment); err != nil {
				t.Fatalf("%s\n%s", unmarshalError, err)
			}
			if probe := deployment.Spec.Template.Spec.Containers[0].ReadinessProbe; probe != nil {
				t.Errorf("expected no readiness probe, got %+v", probe)
			}
		})
	}
}

func TestProbesFromLabels(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("nginx:1.29").
			port(80).
			label(labels.LabelHealthCheck, `
livenessProbe:
  httpGet:
    path: /healthz
    port: 80
readinessProbe:
  httpGet:
    path: /ready
    port: 80`).
			build(),
	)

	output := renderProjectTemplate(t, project, webTemplateOutput)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	container := deployment.Spec.Template.Spec.Containers[0]
	if container.LivenessProbe == nil {
		t.Fatal("expected a liveness probe")
	}
	if container.ReadinessProbe == nil {
		t.Fatal("expected a readiness probe")
	}
	if container.LivenessProbe.HTTPGet.Path != "/healthz" {
		t.Errorf("expected liveness path /healthz, got %s", container.LivenessProbe.HTTPGet.Path)
	}
	if container.ReadinessProbe.HTTPGet.Path != "/ready" {
		t.Errorf("expected readiness path /ready, got %s", container.ReadinessProbe.HTTPGet.Path)
	}
}

func TestSetValues(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("nginx:1.29").
			env("FOO", "bar").
			env("BAZ", "qux").
			label(labels.LabelValues, "- FOO").
			build(),
	)

	values := projectValues(t, project)
	mapping := struct {
		Web struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"web"`
	}{}
	if err := yaml3.Unmarshal(values, &mapping); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	if value, ok := mapping.Web.Environment["FOO"]; !ok || value != "bar" {
		t.Errorf("expected FOO=bar in web environment, got %q (present: %v)", value, ok)
	}
	if value, ok := mapping.Web.Environment["BAZ"]; ok {
		t.Errorf("expected BAZ to stay in the configmap, got %q", value)
	}
}

func TestWithUnderscoreInContainerName(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web-app").
			image("nginx:1.29").
			containerName("web_app_container").
			env("FOO", "BAR").
			label(labels.LabelValues, "- FOO").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/web_app/deployment.yaml")
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	if name := deployment.Spec.Template.Spec.Containers[0].Name; strings.Contains(name, "_") {
		t.Errorf("expected no underscore in the container name, got %s", name)
	}
}

func TestWithDashes(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web-app").
			image("nginx:1.29").
			env("FOO", "BAR").
			label(labels.LabelValues, "- FOO").
			build(),
	)

	values := projectValues(t, project)
	mapping := struct {
		Web struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"web_app"`
	}{}
	if err := yaml3.Unmarshal(values, &mapping); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	if value, ok := mapping.Web.Environment["FOO"]; !ok || value != "BAR" {
		t.Errorf("expected FOO=BAR under web_app, got %q (present: %v)", value, ok)
	}
}

func TestDashesWithValueFrom(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web-app").
			image("nginx:1.29").
			env("FOO", "BAR").
			label(labels.LabelValues, "- FOO").
			build(),
		newService("web2").
			image("nginx:1.29").
			label(labels.LabelValuesFrom, "BAR: web-app.FOO").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/web2/deployment.yaml")
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	env := deployment.Spec.Template.Spec.Containers[0].Env[0]
	if env.Value != "" {
		t.Errorf("expected an empty value, got %q", env.Value)
	}
	if env.ValueFrom == nil {
		t.Error("expected valueFrom to be set")
	}

	values := projectValues(t, project)
	mapping := struct {
		Web struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"web_app"`
	}{}
	if err := yaml3.Unmarshal(values, &mapping); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	// web-app was renamed to web_app and the variable is kept in values.yaml
	if _, ok := mapping.Web.Environment["FOO"]; !ok {
		t.Error("expected FOO under web_app in values.yaml")
	}
}

func TestCheckCommand(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web-app").
			image("nginx:1.29").
			command("sh", "-c", "echo \"Hello, World!\"\necho \"Done\"").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/web_app/deployment.yaml")
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	args := deployment.Spec.Template.Spec.Containers[0].Args
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if args[0] != "sh" || args[1] != "-c" {
		t.Errorf("expected 'sh -c', got %v", args)
	}
}

func TestEntryPoint(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("nginx:1.29").
			entrypoint("/bin/foo").
			command("bar", "baz").
			build(),
	)

	output := renderProjectTemplate(t, project, webTemplateOutput)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	container := deployment.Spec.Template.Spec.Containers[0]
	if len(container.Command) == 0 || container.Command[0] != "/bin/foo" {
		t.Errorf("expected entrypoint /bin/foo, got %v", container.Command)
	}
	if len(container.Args) != 2 || container.Args[0] != "bar" || container.Args[1] != "baz" {
		t.Errorf("expected args [bar baz], got %v", container.Args)
	}
}

func TestRestrictedRBACGeneration(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").image("nginx:1.29").port(80).dependsOn("database").build(),
		newService("database").image("mariadb:10.5").port(3306).build(),
	)

	rbacOutput := string(renderProjectTemplate(t, project, "templates/web/depends-on.rbac.yaml"))

	docs := strings.Split(rbacOutput, "---\n")
	filteredDocs := make([]string, 0, len(docs))
	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		lines := strings.Split(doc, "\n")
		contentLines := make([]string, 0, len(lines))
		for _, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "# Source:") {
				contentLines = append(contentLines, line)
			}
		}
		filteredDocs = append(filteredDocs, strings.Join(contentLines, "\n"))
	}

	if len(filteredDocs) != 3 {
		t.Fatalf("expected 3 YAML documents in the RBAC file, got %d", len(filteredDocs))
	}

	docsByKind := make(map[string][]byte, len(filteredDocs))
	for _, doc := range filteredDocs {
		var meta struct {
			Kind string `yaml:"kind"`
		}
		if err := yaml.Unmarshal([]byte(doc), &meta); err != nil {
			t.Fatalf("cannot read the document kind: %v", err)
		}
		docsByKind[meta.Kind] = []byte(doc)
	}

	saDoc, ok := docsByKind["ServiceAccount"]
	if !ok {
		t.Fatal("ServiceAccount document not found")
	}
	var sa corev1.ServiceAccount
	if err := yaml.Unmarshal(saDoc, &sa); err != nil {
		t.Errorf("cannot unmarshal ServiceAccount: %v", err)
	}
	if sa.Kind != "ServiceAccount" {
		t.Errorf("expected Kind=ServiceAccount, got %s", sa.Kind)
	}
	if !strings.Contains(sa.Name, "web") {
		t.Errorf("expected the ServiceAccount name to contain web, got %s", sa.Name)
	}

	roleDoc, ok := docsByKind["Role"]
	if !ok {
		t.Fatal("Role document not found")
	}
	var role rbacv1.Role
	if err := yaml.Unmarshal(roleDoc, &role); err != nil {
		t.Errorf("cannot unmarshal Role: %v", err)
	}
	if role.Kind != "Role" {
		t.Errorf("expected Kind=Role, got %s", role.Kind)
	}
	if len(role.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(role.Rules))
	}
	rule := role.Rules[0]
	if !slices.Contains(rule.APIGroups, "apps") {
		t.Error("expected APIGroup to include apps")
	}
	if !slices.Contains(rule.Resources, "deployments") {
		t.Errorf("expected Resource to include deployments, got %v", rule.Resources)
	}
	if slices.Contains(rule.Resources, "*") || slices.Contains(rule.Verbs, "*") {
		t.Error("the restricted role must not use wildcards")
	}

	roleBindingDoc, ok := docsByKind["RoleBinding"]
	if !ok {
		t.Fatal("RoleBinding document not found")
	}
	var rb rbacv1.RoleBinding
	if err := yaml.Unmarshal(roleBindingDoc, &rb); err != nil {
		t.Errorf("cannot unmarshal RoleBinding: %v", err)
	}
	if rb.Kind != "RoleBinding" {
		t.Errorf("expected Kind=RoleBinding, got %s", rb.Kind)
	}
	if len(rb.Subjects) != 1 {
		t.Errorf("expected 1 subject, got %d", len(rb.Subjects))
	}
	if rb.Subjects[0].Kind != "ServiceAccount" {
		t.Errorf("expected a ServiceAccount subject, got %s", rb.Subjects[0].Kind)
	}
	if !strings.Contains(rb.RoleRef.Name, "web") {
		t.Errorf("expected the RoleRef name to contain web, got %s", rb.RoleRef.Name)
	}
	if rb.RoleRef.Kind != "Role" {
		t.Errorf("expected RoleRef Kind=Role, got %s", rb.RoleRef.Kind)
	}
}

func TestDeploymentReferencesServiceAccount(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").image("nginx:1.29").port(80).dependsOn("database").build(),
		newService("database").image("mariadb:10.5").port(3306).build(),
	)

	chart, err := Generate(project)
	if err != nil {
		t.Fatalf("generate failed: %s", err)
	}
	if _, ok := chart.Templates["web/depends-on.rbac.yaml"]; !ok {
		t.Error("the depends-on RBAC template should exist")
	}

	output := renderProjectTemplate(t, project, webTemplateOutput)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	serviceAccountName := deployment.Spec.Template.Spec.ServiceAccountName
	if !strings.Contains(serviceAccountName, "web") {
		t.Errorf("expected the ServiceAccount name to contain web, got %s", serviceAccountName)
	}
	if len(deployment.Spec.Template.Spec.InitContainers) == 0 {
		t.Fatal("expected an init container for depends_on")
	}

	initContainer := deployment.Spec.Template.Spec.InitContainers[0]
	if initContainer.Name != "wait-for-database" {
		t.Errorf("expected init container wait-for-database, got %s", initContainer.Name)
	}
	fullCommand := strings.Join(initContainer.Command, " ")
	for _, want := range []string{"curl", "/apis/apps/v1/namespaces/", "/deployments/", "readyReplicas"} {
		if !strings.Contains(fullCommand, want) {
			t.Errorf("expected %q in the init container command, got %s", want, fullCommand)
		}
	}
}
