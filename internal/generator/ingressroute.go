package generator

import (
	"strings"

	"sigs.k8s.io/yaml"

	"katenary.io/internal/generator/labels/labelstructs"
	"katenary.io/internal/utils"

	"github.com/compose-spec/compose-go/v2/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Yaml = (*IngressRoute)(nil)

// IngressRoute represents a Traefik IngressRoute CRD
type IngressRoute struct {
	metav1.TypeMeta   `yaml:",inline"`
	metav1.ObjectMeta `yaml:"metadata"`
	Spec              IngressRouteSpec `yaml:"spec"`
	service           *types.ServiceConfig `yaml:"-"`
	appName           string               `yaml:"-"`
	serviceName       string               `yaml:"-"`
}

// IngressRouteSpec defines the spec for Traefik IngressRoute
type IngressRouteSpec struct {
	EntryPoints []string              `json:"entryPoints,omitempty" yaml:"entryPoints,omitempty"`
	Routes      []IngressRouteRoute  `json:"routes" yaml:"routes"`
	TLS         *IngressRouteTLS     `json:"tls,omitempty" yaml:"tls,omitempty"`
}

// IngressRouteRoute defines a route in the IngressRoute
type IngressRouteRoute struct {
	Match    string             `json:"match" yaml:"match"`
	Kind     string             `json:"kind" yaml:"kind"`
	Services []IngressRouteService `json:"services" yaml:"services"`
}

// IngressRouteService defines a service backend in IngressRoute
type IngressRouteService struct {
	Name string `json:"name" yaml:"name"`
	Port int    `json:"port" yaml:"port"`
}

// IngressRouteTLS defines TLS configuration for IngressRoute
type IngressRouteTLS struct {
	SecretName string   `json:"secretName,omitempty" yaml:"secretName,omitempty"`
	Domains    []IngressRouteTLSDomain `json:"domains,omitempty" yaml:"domains,omitempty"`
}

// IngressRouteTLSDomain defines a domain for TLS
type IngressRouteTLSDomain struct {
	Main string `json:"main" yaml:"main"`
}

// NewIngressRoute creates a new Traefik IngressRoute from a compose service.
func NewIngressRoute(service types.ServiceConfig, Chart *HelmChart, mapping *labelstructs.Ingress, serviceName, appName string) *IngressRoute {
	fullName := `{{ $fullname }}-` + serviceName

	// Build the route match rule
	match := `Host("{{ tpl .Values.` + serviceName + `.ingress.host . }}")`
	path := utils.TplValue(serviceName, "ingress.path")
	if path != "/" && path != "" {
		match += ` && PathPrefix("` + path + `")`
	}

	ir := &IngressRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IngressRoute",
			APIVersion: "traefik.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        fullName,
			Labels:      GetLabels(serviceName, appName),
			Annotations: Annotations,
		},
		Spec: IngressRouteSpec{
			EntryPoints: []string{"web", "websecure"},
			Routes: []IngressRouteRoute{
				{
					Match: match,
					Kind:  "Rule",
					Services: []IngressRouteService{
						{
							Name: fullName,
							Port: int(*mapping.Port),
						},
					},
				},
			},
		},
		service:     &service,
		appName:     appName,
		serviceName: serviceName,
	}

	// Add TLS configuration if enabled
	if mapping.TLS != nil && mapping.TLS.Enabled {
		tlsSecretName := `{{ .Values.` + serviceName + `.ingress.tls.secretName | default $tlsname }}`
		ir.Spec.TLS = &IngressRouteTLS{
			SecretName: tlsSecretName,
			Domains: []IngressRouteTLSDomain{
				{
					Main: `{{ tpl .Values.` + serviceName + `.ingress.host . }}`,
				},
			},
		}
	}

	return ir
}

func (ir *IngressRoute) Filename() string {
	return ir.serviceName + ".ingressroute.yaml"
}

func (ir *IngressRoute) Yaml() ([]byte, error) {
	var ret []byte
	var err error

	// Manually construct YAML - sigs.k8s.io/yaml doesn't handle metav1.ObjectMeta
	// with yaml:"metadata" correctly unless embedded in a standard K8s type with JSON tags.
	// We build the YAML as a string to ensure proper nesting.

	// Build metadata block
	metadata, err := yaml.Marshal(map[string]interface{}{
		"name":        ir.Name,
		"labels":      ir.Labels,
		"annotations": ir.Annotations,
	})
	if err != nil {
		return nil, err
	}

	// Build spec block
	spec, err := yaml.Marshal(ir.Spec)
	if err != nil {
		return nil, err
	}

	// Build final YAML with proper structure
	ret = []byte("apiVersion: " + ir.APIVersion + "\n")
	ret = append(ret, "kind: "+ir.Kind+"\n"...)
	ret = append(ret, "metadata:\n"...)
	// Indent metadata content by 2 spaces
	for _, line := range strings.Split(strings.TrimRight(string(metadata), "\n"), "\n") {
		ret = append(ret, "  "+line+"\n"...)
	}
	ret = append(ret, "spec:\n"...)
	// Indent spec content by 2 spaces
	for _, line := range strings.Split(strings.TrimRight(string(spec), "\n"), "\n") {
		ret = append(ret, "  "+line+"\n"...)
	}

	ret = UnWrapTPL(ret)

	lines := strings.Split(string(ret), "\n")

	out := []string{
		`{{- if .Values.` + ir.serviceName + `.ingress.ingressRouteEnabled -}}`,
		`{{- $fullname := include "` + ir.appName + `.fullname" . -}}`,
		`{{- $tlsname := printf "%s-%s-tls" $fullname "` + ir.serviceName + `" -}}`,
	}

	for _, line := range lines {
		if strings.Contains(line, "labels:") {
			// add annotations above labels from values.yaml (inside metadata block)
			indent := strings.Repeat(" ", utils.CountStartingSpaces(line))
			content := `` +
				indent + `{{- if .Values.` + ir.serviceName + `.ingress.annotations -}}` + "\n" +
				indent + `    {{- toYaml .Values.` + ir.serviceName + `.ingress.annotations | nindent __indent__ }}` + "\n" +
				indent + `    {{- end }}` + "\n" +
				line

			out = append(out, content)
		} else {
			out = append(out, line)
		}
	}
	out = append(out, `{{- end -}}`)
	ret = []byte(strings.Join(out, "\n"))
	return ret, nil
}

