package generator

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"testing"

	"katenary.io/internal/generator/labels"

	"github.com/compose-spec/compose-go/v2/types"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestEnvInConfigMap(t *testing.T) {
	composeFile := `
services:
    web:
        image: nginx:1.29
        environment:
        - FOO=bar
        - BAR=baz
`
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", "templates/web/configmap.yaml")
	configMap := v1.ConfigMap{}
	if err := yaml.Unmarshal([]byte(output), &configMap); err != nil {
		t.Errorf(unmarshalError, err)
	}
	data := configMap.Data
	if len(data) != 2 {
		t.Errorf("Expected 2 data, got %d", len(data))
	}
	if data["FOO"] != "bar" {
		t.Errorf("Expected FOO to be bar, got %s", data["FOO"])
	}
	if data["BAR"] != "baz" {
		t.Errorf("Expected BAR to be baz, got %s", data["BAR"])
	}
}

func TestMapEnv(t *testing.T) {
	composeFile := `
services:
  web:
    image: nginx:1.29
    environment:
      FOO: bar
    labels:
      %[1]s/map-env: |-
        FOO: 'baz'
`

	composeFile = fmt.Sprintf(composeFile, labels.KatenaryLabelPrefix)
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)

	output := internalCompileTest(t, "-s", "templates/web/configmap.yaml")
	configMap := v1.ConfigMap{}
	if err := yaml.Unmarshal([]byte(output), &configMap); err != nil {
		t.Errorf(unmarshalError, err)
	}
	data := configMap.Data
	if v, ok := data["FOO"]; !ok || v != "baz" {
		t.Errorf("Expected FOO to be baz, got %s", v)
	}
}

func TestAppendBadFile(t *testing.T) {
	cm := NewConfigMap(types.ServiceConfig{}, "app", true)
	err := cm.AppendFile("foo")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestAppendBadDir(t *testing.T) {
	cm := NewConfigMap(types.ServiceConfig{}, "app", true)
	err := cm.AppendDir("foo")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestRootConfigmapfile(t *testing.T) {
	composeFile := `
services:
  web:
    image: nginx
    volumes:
      - ./foo.txt:/etc/foo.txt
    labels:
      %[1]s/configmap-files: |-
        - ./foo.txt
`
	composeFile = fmt.Sprintf(composeFile, labels.KatenaryLabelPrefix)
	tmpDir := setup(composeFile)
	defer teardown(tmpDir)

	currentDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(currentDir)
	fooTxt := "foo content"
	fooFp, _ := os.Create("foo.txt")
	io.WriteString(fooFp, fooTxt)
	fooFp.Close()

	cmOutput := internalCompileTestForce(t, "-s", "templates/web/statics/configmap.yaml")
	depOutput := internalCompileTestForce(t, "-s", "templates/web/deployment.yaml")

	configMap := v1.ConfigMap{}
	if err := yaml.Unmarshal([]byte(cmOutput), &configMap); err != nil {
		t.Errorf(unmarshalError, err)
	}
	deployment := appv1.Deployment{}
	if err := yaml.Unmarshal([]byte(depOutput), &deployment); err != nil {
		t.Errorf(unmarshalError, err)
	}
	if configMap.Data == nil {
		t.Error("Expected configmap data to not be nil")
	}
	// if the configmap.Name ends by anything that is not alphanumeric, there is a problem
	valid := regexp.MustCompile(`.*[a-zA-Z0-9]+$`)
	if !valid.MatchString(configMap.Name) {
		t.Errorf("ConfigMap name %s is not valid", configMap.Name)
	}

	// the volume mount should be named "configmap-<configmap name>"
	if deployment.Spec.Template.Spec.Volumes[0].Name != deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name {
		t.Errorf("Expected volume name to be %s, got %s",
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name,
			deployment.Spec.Template.Spec.Volumes[0].Name,
		)
	}
}
