package generator

import (
	"strings"
	"testing"

	"katenary.io/internal/generator/labels"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func TestValuesFrom(t *testing.T) {
	t.Parallel()
	service := newService("aa").image("nginx:latest").env("AA_USER", "foo").build()

	configMap := NewConfigMap(service, NewChart("testapp"), false)

	if value, ok := configMap.Data["AA_USER"]; !ok || value != "foo" {
		t.Errorf("expected AA_USER to be foo, got %s", value)
	}
}

func TestValuesFromCopy(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("aa").image("nginx:latest").env("AA_USER", "foo").build(),
		newService("bb").
			image("nginx:latest").
			label(labels.LabelValuesFrom, "BB_USER: aa.AA_USER").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/bb/deployment.yaml")
	deployment := appsv1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	environment := deployment.Spec.Template.Spec.Containers[0].Env[0]
	envFrom := environment.ValueFrom.ConfigMapKeyRef
	if envFrom.Key != "AA_USER" {
		t.Errorf("expected AA_USER, got %s", envFrom.Key)
	}
	if !strings.Contains(envFrom.Name, "aa") {
		t.Errorf("expected a reference to aa, got %s", envFrom.Name)
	}
}

func TestValuesFromSecret(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("aa").
			image("nginx:latest").
			env("AA_USER", "foo").
			label(labels.LabelSecrets, "- AA_USER").
			build(),
		newService("bb").
			image("nginx:latest").
			label(labels.LabelValuesFrom, "BB_USER: aa.AA_USER").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/bb/deployment.yaml")
	deployment := appsv1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	environment := deployment.Spec.Template.Spec.Containers[0].Env[0]
	envFrom := environment.ValueFrom.SecretKeyRef
	if envFrom.Key != "AA_USER" {
		t.Errorf("expected AA_USER, got %s", envFrom.Key)
	}
	if !strings.Contains(envFrom.Name, "aa") {
		t.Errorf("expected a reference to aa, got %s", envFrom.Name)
	}
}

func TestEnvFrom(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("nginx:1.29").
			env("Foo", "bar").
			env("BAZ", "qux").
			build(),
		newService("db").
			image("postgres").
			label(labels.LabelEnvFrom, "- web").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/db/deployment.yaml")
	deployment := appsv1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	envFrom := deployment.Spec.Template.Spec.Containers[0].EnvFrom
	if len(envFrom) != 1 {
		t.Fatalf("expected 1 envFrom, got %d", len(envFrom))
	}
}
