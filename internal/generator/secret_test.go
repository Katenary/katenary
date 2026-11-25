package generator

import (
	"bytes"
	"testing"

	"katenary.io/internal/generator/labels"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestCreateSecretFromEnvironment(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			env("FOO", "bar").
			env("BAR", "baz").
			label(labels.LabelSecrets, "- BAR").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/web/secret.yaml")
	secret := corev1.Secret{}
	if err := yaml.Unmarshal(output, &secret); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	if len(secret.Data) != 1 {
		t.Fatalf("expected 1 secret entry, got %d", len(secret.Data))
	}
	if string(secret.Data["BAR"]) != "baz" {
		t.Errorf("expected BAR to be baz, got %s", secret.Data["BAR"])
	}
}

func TestCreateSecretFromEnvironmentWithValue(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			env("FOO", "bar").
			env("BAR", "baz").
			label(labels.LabelSecrets, "- BAR").
			label(labels.LabelValues, "- BAR").
			build(),
	)

	values := projectValues(t, project)
	if !bytes.Contains(values, []byte("BAR: baz")) {
		t.Errorf("expected values.yaml to contain BAR: baz, got:\n%s", values)
	}
}
