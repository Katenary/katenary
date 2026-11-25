package generator

import (
	"path/filepath"
	"regexp"
	"testing"

	"katenary.io/internal/generator/labels"

	"github.com/compose-spec/compose-go/v2/types"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestEnvInConfigMap(t *testing.T) {
	t.Parallel()
	service := newService("web").
		image("nginx:1.29").
		env("FOO", "bar").
		env("BAR", "baz").
		build()

	configMap := NewConfigMap(service, NewChart("testapp"), false)

	if len(configMap.Data) != 2 {
		t.Fatalf("expected 2 data entries, got %d", len(configMap.Data))
	}
	if configMap.Data["FOO"] != "bar" {
		t.Errorf("expected FOO to be bar, got %s", configMap.Data["FOO"])
	}
	if configMap.Data["BAR"] != "baz" {
		t.Errorf("expected BAR to be baz, got %s", configMap.Data["BAR"])
	}
}

func TestMapEnv(t *testing.T) {
	t.Parallel()
	service := newService("web").
		image("nginx:1.29").
		env("FOO", "bar").
		label(labels.LabelMapEnv, "FOO: 'baz'").
		build()

	configMap := NewConfigMap(service, NewChart("testapp"), false)

	if value, ok := configMap.Data["FOO"]; !ok || value != "baz" {
		t.Errorf("expected FOO to be baz, got %s", value)
	}
}

func TestAppendBadFile(t *testing.T) {
	t.Parallel()
	cm := NewConfigMap(types.ServiceConfig{}, NewChart("app"), true)
	err := cm.AppendFile("foo")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestAppendBadDir(t *testing.T) {
	t.Parallel()
	cm := NewConfigMap(types.ServiceConfig{}, NewChart("app"), true)
	err := cm.AppendDir("foo")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestRootConfigmapfile(t *testing.T) {
	chartDir := filepath.Join(t.TempDir(), "chart")
	convertFixture(t, testCompose("configmap-root"), chartDir)

	cmOutput := templateChart(t, chartDir, "-s", "templates/web/statics/configmap.yaml")
	depOutput := templateChart(t, chartDir, "-s", "templates/web/deployment.yaml")

	configMap := v1.ConfigMap{}
	if err := yaml.Unmarshal(cmOutput, &configMap); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	deployment := appv1.Deployment{}
	if err := yaml.Unmarshal(depOutput, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	if configMap.Data == nil {
		t.Error("expected configmap data to not be nil")
	}
	// the configmap name must end with an alphanumeric character
	valid := regexp.MustCompile(`.*[a-zA-Z0-9]+$`)
	if !valid.MatchString(configMap.Name) {
		t.Errorf("configmap name %s is not valid", configMap.Name)
	}

	// the volume and the volume mount must share the same name
	if deployment.Spec.Template.Spec.Volumes[0].Name != deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name {
		t.Errorf("expected volume name %s, got %s",
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name,
			deployment.Spec.Template.Spec.Volumes[0].Name,
		)
	}
}
