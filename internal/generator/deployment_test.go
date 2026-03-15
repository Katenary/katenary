package generator

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"katenary.io/internal/generator/labels"

	yaml3 "gopkg.in/yaml.v3"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"
)

const webTemplateOutput = `templates/web/deployment.yaml`

func TestGenerate(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
`
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", webTemplateOutput)

	// dt := DeploymentTest{}
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	if *dt.Spec.Replicas != 1 {
		t.Errorf("Expected replicas to be 1, got %d", dt.Spec.Replicas)
		t.Errorf("Output: %s", output)
	}

	if dt.Spec.Template.Spec.Containers[0].Image != "nginx:1.29" {
		t.Errorf("Expected image to be nginx:1.29, got %s", dt.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestGenerateOneDeploymentWithSamePod(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80

    fpm:
        image: php:fpm
        ports:
        - 9000:9000
        labels:
            katenary.v3/same-pod: web
`

	outDir := "./chart"
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", webTemplateOutput)
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	if len(dt.Spec.Template.Spec.Containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(dt.Spec.Template.Spec.Containers))
	}
	// endsure that the fpm service is not created

	var err error
	_, err = helmTemplate(ConvertOptions{
		OutputDir: outDir,
	}, "-s", "templates/fpm/deployment.yaml")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	// ensure that the web service is created and has got 2 ports
	output, err = helmTemplate(ConvertOptions{
		OutputDir: outDir,
	}, "-s", "templates/web/service.yaml")
	if err != nil {
		t.Errorf("Error: %s", err)
	}
	service := corev1.Service{}
	if err := yaml.Unmarshal([]byte(output), &service); err != nil {
		t.Errorf(unmarshalError, err)
	}

	if len(service.Spec.Ports) != 2 {
		t.Errorf("Expected 2 ports, got %d", len(service.Spec.Ports))
	}
}

func TestDependsOn(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        depends_on:
        - database

    database:
        image: mariadb:10.5
        ports:
        - 3306:3306
`
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", webTemplateOutput)
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	if len(dt.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(dt.Spec.Template.Spec.Containers))
	}
	// find an init container
	if len(dt.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("Expected 1 init container, got %d", len(dt.Spec.Template.Spec.InitContainers))
	}

	initContainer := dt.Spec.Template.Spec.InitContainers[0]
	if !strings.Contains(initContainer.Image, "busybox") {
		t.Errorf("Expected busybox image, got %s", initContainer.Image)
	}

	fullCommand := strings.Join(initContainer.Command, " ")
	if !strings.Contains(fullCommand, "wget") {
		t.Errorf("Expected wget command (K8s API method), got %s", fullCommand)
	}

	if !strings.Contains(fullCommand, "/api/v1/namespaces/") {
		t.Errorf("Expected Kubernetes API call to /api/v1/namespaces/, got %s", fullCommand)
	}

	if !strings.Contains(fullCommand, "/endpoints/") {
		t.Errorf("Expected Kubernetes API call to /endpoints/, got %s", fullCommand)
	}

	if len(initContainer.Env) == 0 {
		t.Errorf("Expected environment variables to be set for namespace")
	}

	hasNamespace := false
	for _, env := range initContainer.Env {
		if env.Name == "NAMESPACE" && env.ValueFrom != nil && env.ValueFrom.FieldRef != nil {
			if env.ValueFrom.FieldRef.FieldPath == "metadata.namespace" {
				hasNamespace = true
				break
			}
		}
	}
	if !hasNamespace {
		t.Errorf("Expected NAMESPACE env var with metadata.namespace fieldRef")
	}
}

func TestDependsOnLegacy(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        depends_on:
        - database
        labels:
            katenary.v3/depends-on: legacy

    database:
        image: mariadb:10.5
        ports:
        - 3306:3306
`
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", webTemplateOutput)
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	if len(dt.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("Expected 1 init container, got %d", len(dt.Spec.Template.Spec.InitContainers))
	}

	initContainer := dt.Spec.Template.Spec.InitContainers[0]
	if !strings.Contains(initContainer.Image, "busybox") {
		t.Errorf("Expected busybox image, got %s", initContainer.Image)
	}

	fullCommand := strings.Join(initContainer.Command, " ")
	if !strings.Contains(fullCommand, "nc") {
		t.Errorf("Expected nc (netcat) command for legacy method, got %s", fullCommand)
	}
}

func TestHelmDependencies(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80

    mariadb:
        image: mariadb:10.5
        ports:
        - 3306:3306
        labels:
            %s/dependencies: |
                - name: mariadb
                  repository: oci://registry-1.docker.io/bitnamicharts
                  version: 18.x.X

    `
	composeFile = fmt.Sprintf(composeFile, labels.Prefix())
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", webTemplateOutput)
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	// ensure that there is no mariasb deployment
	_, err := helmTemplate(ConvertOptions{
		OutputDir: "./chart",
	}, "-s", "templates/mariadb/deployment.yaml")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	// check that Chart.yaml has the dependency
	chart := HelmChart{}
	chartFile := "./chart/Chart.yaml"
	if _, err := os.Stat(chartFile); os.IsNotExist(err) {
		t.Errorf("Chart.yaml does not exist")
	}
	chartContent, err := os.ReadFile(chartFile)
	if err != nil {
		t.Errorf("Error reading Chart.yaml: %s", err)
	}
	if err := yaml.Unmarshal(chartContent, &chart); err != nil {
		t.Errorf(unmarshalError, err)
	}

	if len(chart.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(chart.Dependencies))
	}
}

func TestLivenessProbesFromHealthCheck(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        healthcheck:
            test: ["CMD", "curl", "-f", "http://localhost"]
            interval: 5s
            timeout: 3s
            retries: 3
        `
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", webTemplateOutput)
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	if dt.Spec.Template.Spec.Containers[0].LivenessProbe == nil {
		t.Errorf("Expected liveness probe to be set")
	}
}

func TestProbesFromLabels(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        labels:
            %s/health-check: |
                livenessProbe:
                    httpGet:
                        path: /healthz
                        port: 80
                readinessProbe:
                    httpGet:
                        path: /ready
                        port: 80
    `
	composeFile = fmt.Sprintf(composeFile, labels.Prefix())
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", webTemplateOutput)
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	if dt.Spec.Template.Spec.Containers[0].LivenessProbe == nil {
		t.Errorf("Expected liveness probe to be set")
	}
	if dt.Spec.Template.Spec.Containers[0].ReadinessProbe == nil {
		t.Errorf("Expected readiness probe to be set")
	}
	t.Logf("LivenessProbe: %+v", dt.Spec.Template.Spec.Containers[0].LivenessProbe)

	// ensure that the liveness probe is set to /healthz
	if dt.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Path != "/healthz" {
		t.Errorf("Expected liveness probe path to be /healthz, got %s", dt.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Path)
	}

	// ensure that the readiness probe is set to /ready
	if dt.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path != "/ready" {
		t.Errorf("Expected readiness probe path to be /ready, got %s", dt.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path)
	}
}

func TestSetValues(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        environment:
            FOO: bar
            BAZ: qux
        labels:
            %s/values: |
                - FOO
`

	composeFile = fmt.Sprintf(composeFile, labels.Prefix())
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", webTemplateOutput)
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	// readh the values.yaml, we must have FOO in web environment but not BAZ
	valuesFile := "./chart/values.yaml"
	if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
		t.Errorf("values.yaml does not exist")
	}
	valuesContent, err := os.ReadFile(valuesFile)
	if err != nil {
		t.Errorf("Error reading values.yaml: %s", err)
	}
	mapping := struct {
		Web struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"web"`
	}{}
	if err := yaml3.Unmarshal(valuesContent, &mapping); err != nil {
		t.Errorf(unmarshalError, err)
	}

	if v, ok := mapping.Web.Environment["FOO"]; !ok {
		t.Errorf("Expected FOO in web environment")
		if v != "bar" {
			t.Errorf("Expected FOO to be bar, got %s", v)
		}
	}
	if v, ok := mapping.Web.Environment["BAZ"]; ok {
		t.Errorf("Expected BAZ not in web environment")
		if v != "qux" {
			t.Errorf("Expected BAZ to be qux, got %s", v)
		}
	}
}

func TestWithUnderscoreInContainerName(t *testing.T) {
	composeFile := `
services:
    web-app:
        image: nginx:1.29
        container_name: web_app_container
        environment:
            FOO: BAR
        labels:
            %s/values: |
                - FOO
`
	composeFile = fmt.Sprintf(composeFile, labels.Prefix())
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", "templates/web_app/deployment.yaml")
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}
	// find container.name
	containerName := dt.Spec.Template.Spec.Containers[0].Name
	if strings.Contains(containerName, "_") {
		t.Errorf("Expected container name to not contain underscores, got %s", containerName)
	}
}

func TestWithDashes(t *testing.T) {
	composeFile := `
services:
    web-app:
        image: nginx:1.29
        environment:
            FOO: BAR
        labels:
            %s/values: |
                - FOO
`

	composeFile = fmt.Sprintf(composeFile, labels.Prefix())
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", "templates/web_app/deployment.yaml")
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	valuesFile := "./chart/values.yaml"
	if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
		t.Errorf("values.yaml does not exist")
	}
	valuesContent, err := os.ReadFile(valuesFile)
	if err != nil {
		t.Errorf("Error reading values.yaml: %s", err)
	}
	mapping := struct {
		Web struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"web_app"`
	}{}
	if err := yaml3.Unmarshal(valuesContent, &mapping); err != nil {
		t.Errorf(unmarshalError, err)
	}

	// we must have FOO in web_app environment (not web-app)
	// this validates that the service name is converted to a valid k8s name
	if v, ok := mapping.Web.Environment["FOO"]; !ok {
		t.Errorf("Expected FOO in web_app environment")
		if v != "BAR" {
			t.Errorf("Expected FOO to be BAR, got %s", v)
		}
	}
}

func TestDashesWithValueFrom(t *testing.T) {
	composeFile := `
services:
    web-app:
        image: nginx:1.29
        environment:
            FOO: BAR
        labels:
            %[1]s/values: |
                - FOO
    web2:
        image: nginx:1.29
        labels:
            %[1]s/values-from: |
                BAR: web-app.FOO
`

	composeFile = fmt.Sprintf(composeFile, labels.Prefix())
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", "templates/web2/deployment.yaml")
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}

	valuesFile := "./chart/values.yaml"
	if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
		t.Errorf("values.yaml does not exist")
	}
	valuesContent, err := os.ReadFile(valuesFile)
	if err != nil {
		t.Errorf("Error reading values.yaml: %s", err)
	}
	mapping := struct {
		Web struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"web_app"`
	}{}
	if err := yaml3.Unmarshal(valuesContent, &mapping); err != nil {
		t.Errorf(unmarshalError, err)
	}

	// we must have FOO in web_app environment (not web-app)
	// this validates that the service name is converted to a valid k8s name
	if v, ok := mapping.Web.Environment["FOO"]; !ok {
		t.Errorf("Expected FOO in web_app environment")
		if v != "BAR" {
			t.Errorf("Expected FOO to be BAR, got %s", v)
		}
	}

	// ensure that the deployment has the value from the other service
	barenv := dt.Spec.Template.Spec.Containers[0].Env[0]
	if barenv.Value != "" {
		t.Errorf("Expected value to be empty")
	}
	if barenv.ValueFrom == nil {
		t.Errorf("Expected valueFrom to be set")
	}
}

func TestCheckCommand(t *testing.T) {
	composeFile := `
services:
    web-app:
        image: nginx:1.29
        command:
            - sh
            - -c
            - |-
                echo "Hello, World!"
                echo "Done"
`

	// composeFile = fmt.Sprintf(composeFile, labels.Prefix())
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", "templates/web_app/deployment.yaml")
	dt := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf(unmarshalError, err)
	}
	// find the command in the container
	command := dt.Spec.Template.Spec.Containers[0].Args
	if len(command) != 3 {
		t.Errorf("Expected command to have 3 elements, got %d", len(command))
	}
	if command[0] != "sh" || command[1] != "-c" {
		t.Errorf("Expected command to be 'sh -c', got %s", strings.Join(command, " "))
	}
}

func TestEntryPoint(t *testing.T) {
	composeFile := `
services:
  web:
    image: nginx:1.29
    entrypoint: /bin/foo
    command: bar baz
`
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", "templates/web/deployment.yaml")
	t.Logf("Output: %s", output)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal([]byte(output), &deployment); err != nil {
		t.Errorf(unmarshalError, err)
	}
	entryPoint := deployment.Spec.Template.Spec.Containers[0].Command
	command := deployment.Spec.Template.Spec.Containers[0].Args
	if entryPoint[0] != "/bin/foo" {
		t.Errorf("Expected entrypoint to be /bin/foo, got %s", entryPoint[0])
	}
	if len(command) != 2 || command[0] != "bar" || command[1] != "baz" {
		t.Errorf("Expected command to be 'bar baz', got %s", strings.Join(command, " "))
	}
}

func TestRestrictedRBACGeneration(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        depends_on:
        - database

    database:
        image: mariadb:10.5
        ports:
        - 3306:3306
`
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	rbacOutput := internalCompileTest(t, "-s", "templates/web/depends-on.rbac.yaml")

	docs := strings.Split(rbacOutput, "---\n")

	// Filter out empty documents and strip helm template comments
	var filteredDocs []string
	for _, doc := range docs {
		if strings.TrimSpace(doc) != "" {
			// Remove '# Source:' comment lines that helm template adds
			lines := strings.Split(doc, "\n")
			var contentLines []string
			for _, line := range lines {
				if !strings.HasPrefix(strings.TrimSpace(line), "# Source:") {
					contentLines = append(contentLines, line)
				}
			}
			filteredDocs = append(filteredDocs, strings.Join(contentLines, "\n"))
		}
	}

	if len(filteredDocs) != 3 {
		t.Fatalf("Expected 3 YAML documents in RBAC file, got %d (filtered from %d)", len(filteredDocs), len(docs))
	}

	var sa corev1.ServiceAccount
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(filteredDocs[0])), &sa); err != nil {
		t.Errorf("Failed to unmarshal ServiceAccount: %v", err)
	}
	if sa.Kind != "ServiceAccount" {
		t.Errorf("Expected Kind=ServiceAccount, got %s", sa.Kind)
	}
	if !strings.Contains(sa.Name, "web") {
		t.Errorf("Expected ServiceAccount name to contain 'web', got %s", sa.Name)
	}

	var role rbacv1.Role
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(filteredDocs[1])), &role); err != nil {
		t.Errorf("Failed to unmarshal Role: %v", err)
	}
	if role.Kind != "Role" {
		t.Errorf("Expected Kind=Role, got %s", role.Kind)
	}
	if len(role.Rules) != 1 {
		t.Errorf("Expected 1 rule in Role, got %d", len(role.Rules))
	}

	rule := role.Rules[0]
	if !contains(rule.APIGroups, "") {
		t.Error("Expected APIGroup to include core API ('')")
	}
	if !contains(rule.Resources, "endpoints") {
		t.Errorf("Expected Resource to include 'endpoints', got %v", rule.Resources)
	}

	for _, res := range rule.Resources {
		if res == "*" {
			t.Error("Role should not have wildcard (*) resource permissions")
		}
	}
	for _, verb := range rule.Verbs {
		if verb == "*" {
			t.Error("Role should not have wildcard (*) verb permissions")
		}
	}

	var rb rbacv1.RoleBinding
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(filteredDocs[2])), &rb); err != nil {
		t.Errorf("Failed to unmarshal RoleBinding: %v", err)
	}
	if rb.Kind != "RoleBinding" {
		t.Errorf("Expected Kind=RoleBinding, got %s", rb.Kind)
	}
	if len(rb.Subjects) != 1 {
		t.Errorf("Expected 1 subject in RoleBinding, got %d", len(rb.Subjects))
	}
	if rb.Subjects[0].Kind != "ServiceAccount" {
		t.Errorf("Expected Subject Kind=ServiceAccount, got %s", rb.Subjects[0].Kind)
	}

	// Helm template renders the name, so check if it contains "web"
	if !strings.Contains(rb.RoleRef.Name, "web") {
		t.Errorf("Expected RoleRef Name to contain 'web', got %s", rb.RoleRef.Name)
	}
	if rb.RoleRef.Kind != "Role" {
		t.Errorf("Expected RoleRef Kind=Role, got %s", rb.RoleRef.Kind)
	}
}

func TestDeploymentReferencesServiceAccount(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        ports:
        - 80:80
        depends_on:
        - database

    database:
        image: mariadb:10.5
        ports:
        - 3306:3306
`
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", "templates/web/deployment.yaml")

	var dt v1.Deployment
	if err := yaml.Unmarshal([]byte(output), &dt); err != nil {
		t.Errorf("Failed to unmarshal Deployment: %v", err)
	}

	serviceAccountName := dt.Spec.Template.Spec.ServiceAccountName
	if !strings.Contains(serviceAccountName, "web") {
		t.Errorf("Expected ServiceAccountName to contain 'web', got %s", serviceAccountName)
	}

	if len(dt.Spec.Template.Spec.InitContainers) == 0 {
		t.Fatal("Expected at least one init container for depends_on")
	}

	initContainer := dt.Spec.Template.Spec.InitContainers[0]
	if initContainer.Name != "wait-for-database" {
		t.Errorf("Expected init container name 'wait-for-database', got %s", initContainer.Name)
	}

	fullCommand := strings.Join(initContainer.Command, " ")
	if !strings.Contains(fullCommand, "wget") {
		t.Error("Expected init container to use wget for K8s API calls")
	}
	if !strings.Contains(fullCommand, "/api/v1/namespaces/") {
		t.Error("Expected init container to call /api/v1/namespaces/ endpoint")
	}
	if !strings.Contains(fullCommand, "/endpoints/") {
		t.Error("Expected init container to access /endpoints/ resource")
	}

	hasNamespace := false
	for _, env := range initContainer.Env {
		if env.Name == "NAMESPACE" && env.ValueFrom != nil && env.ValueFrom.FieldRef != nil {
			if env.ValueFrom.FieldRef.FieldPath == "metadata.namespace" {
				hasNamespace = true
				break
			}
		}
	}
	if !hasNamespace {
		t.Error("Expected NAMESPACE env var with metadata.namespace fieldRef")
	}

	_, err := os.Stat("./chart/templates/web/depends-on.rbac.yaml")
	if os.IsNotExist(err) {
		t.Error("RBAC file depends-on.rbac.yaml should exist for service using depends_on with K8s API")
	} else if err != nil {
		t.Errorf("Unexpected error checking RBAC file: %v", err)
	}
}

func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}
