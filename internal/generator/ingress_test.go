package generator

import (
	"testing"

	"katenary.io/internal/generator/labels"

	networkv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"
)

func TestSimpleIngress(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("nginx:1.29").
			port(80).
			port(443).
			label(labels.LabelIngress, "hostname: my.test.tld\nport: 80\nenabled: true").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/web/ingress.yaml")
	ingress := networkv1.Ingress{}
	if err := yaml.Unmarshal(output, &ingress); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	if len(ingress.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(ingress.Spec.Rules))
	}
	if ingress.Spec.Rules[0].Host != "my.test.tld" {
		t.Errorf("expected host my.test.tld, got %s", ingress.Spec.Rules[0].Host)
	}
}

func TestTLS(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("nginx:1.29").
			port(80).
			port(443).
			label(labels.LabelIngress, "hostname: my.test.tld\nport: 80\nenabled: true").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/web/ingress.yaml")
	ingress := networkv1.Ingress{}
	if err := yaml.Unmarshal(output, &ingress); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	if len(ingress.Spec.TLS) != 1 {
		t.Errorf("expected 1 TLS section, got %d", len(ingress.Spec.TLS))
	}
}

func TestTLSName(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").
			image("nginx:1.29").
			port(80).
			port(443).
			label(labels.LabelIngress, "hostname: my.test.tld\nport: 80\nenabled: true").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/web/ingress.yaml",
		"--set", "web.ingress.tls.secretName=mysecret")
	ingress := networkv1.Ingress{}
	if err := yaml.Unmarshal(output, &ingress); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	if len(ingress.Spec.TLS) != 1 {
		t.Fatalf("expected 1 TLS section, got %d", len(ingress.Spec.TLS))
	}
	if ingress.Spec.TLS[0].SecretName != "mysecret" {
		t.Errorf("expected secretName mysecret, got %s", ingress.Spec.TLS[0].SecretName)
	}
}
