package generator

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"katenary.io/internal/generator/labels"

	"github.com/compose-spec/compose-go/v2/types"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	htmlContent      = "<html><body><h1>Hello, World!</h1></body></html>"
	developementFile = "templates/web/deployment.yaml"
	indexHMLFile     = "index.html"
)

// newWorkingProject creates a project whose relative paths resolve from a
// temporary working directory.
func newWorkingProject(t *testing.T, services ...types.ServiceConfig) *types.Project {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return newProject("testapp", services...)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("cannot create directory: %s", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("cannot write %s: %s", name, err)
	}
}

func writePNG(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("cannot create directory: %s", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	red := color.RGBA{255, 0, 0, 255}
	for y := range 100 {
		for x := range 100 {
			img.Set(x, y, red)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("cannot create png: %s", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("cannot encode png: %s", err)
	}
}

func TestGenerateWithBoundVolume(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").image("nginx:1.29").volume("data", "/var/www").build(),
	)
	project.Volumes = types.Volumes{"data": {Name: "data"}}

	output := renderProjectTemplate(t, project, developementFile)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	if name := deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name; name != "data" {
		t.Errorf("expected volume name data, got %s", name)
	}
}

func TestWithStaticFiles(t *testing.T) {
	project := newWorkingProject(t,
		newService("web").
			image("nginx:1.29").
			bind("./static", "/var/www").
			label(labels.LabelConfigMapFiles, "- ./static").
			build(),
	)
	writeFile(t, "static", indexHMLFile, htmlContent)

	output := renderProjectTemplate(t, project, developementFile)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	if mountPath := deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath; mountPath != "/var/www" {
		t.Errorf("expected mount path /var/www, got %s", mountPath)
	}

	cmOutput := renderProjectTemplate(t, project, "templates/web/statics/static/configmap.yaml")
	configMap := corev1.ConfigMap{}
	if err := yaml.Unmarshal(cmOutput, &configMap); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	if len(configMap.Data) != 1 {
		t.Fatalf("expected 1 data entry, got %d", len(configMap.Data))
	}
	if configMap.Data[indexHMLFile] != htmlContent {
		t.Errorf("expected %s in the configmap, got %s", htmlContent, configMap.Data[indexHMLFile])
	}
}

func TestWithFileMapping(t *testing.T) {
	project := newWorkingProject(t,
		newService("web").
			image("nginx:1.29").
			bind("./static/index.html", "/var/www/index.html").
			label(labels.LabelConfigMapFiles, "- ./static/index.html").
			build(),
	)
	writeFile(t, filepath.Join("static", indexHMLFile), "", htmlContent)

	output := renderProjectTemplate(t, project, developementFile)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	mount := deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0]
	if mount.MountPath != "/var/www/index.html" {
		t.Errorf("expected mount path /var/www/index.html, got %s", mount.MountPath)
	}
	if mount.SubPath != indexHMLFile {
		t.Errorf("expected subPath index.html, got %s", mount.SubPath)
	}
}

func TestBinaryMount(t *testing.T) {
	project := newWorkingProject(t,
		newService("web").
			image("nginx").
			bind("./images/foo.png", "/var/www/foo").
			label(labels.LabelConfigMapFiles, "- ./images/foo.png").
			build(),
	)
	writePNG(t, "images", "foo.png")

	output := renderProjectTemplate(t, project, developementFile)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	if len(deployment.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(deployment.Spec.Template.Spec.Volumes))
	}

	cmOutput := renderProjectTemplate(t, project, "templates/web/statics/images/configmap.yaml")
	configMap := corev1.ConfigMap{}
	if err := yaml.Unmarshal(cmOutput, &configMap); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	image, ok := configMap.BinaryData["foo.png"]
	if !ok {
		t.Fatal("expected foo.png in the configmap BinaryData")
	}
	if len(image) == 0 {
		t.Error("expected the image to be non-empty")
	}
}

func TestGloballyBinaryMount(t *testing.T) {
	project := newWorkingProject(t,
		newService("web").
			image("nginx").
			bind("./images", "/var/www/foo").
			label(labels.LabelConfigMapFiles, "- ./images").
			build(),
	)
	writePNG(t, "images", "foo.png")

	output := renderProjectTemplate(t, project, developementFile)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	if len(deployment.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(deployment.Spec.Template.Spec.Volumes))
	}

	cmOutput := renderProjectTemplate(t, project, "templates/web/statics/images/configmap.yaml")
	configMap := corev1.ConfigMap{}
	if err := yaml.Unmarshal(cmOutput, &configMap); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}
	image, ok := configMap.BinaryData["foo.png"]
	if !ok {
		t.Fatal("expected foo.png in the configmap BinaryData")
	}
	if len(image) == 0 {
		t.Error("expected the image to be non-empty")
	}
}

func TestBindFrom(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("web").image("nginx:1.29").volume("data", "/var/www").build(),
		newService("fpm").
			image("php:fpm").
			volume("data", "/var/www").
			label(labels.LabelPorts, "[9000]").
			label(labels.LabelSamePod, "web").
			build(),
	)
	project.Volumes = types.Volumes{"data": {Name: "data"}}

	output := renderProjectTemplate(t, project, developementFile)
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	containers := deployment.Spec.Template.Spec.Containers
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(containers))
	}
	for i, container := range containers {
		if name := container.VolumeMounts[0].Name; name != "data" {
			t.Errorf("expected container %d volume name data, got %s", i, name)
		}
	}
}

func TestExchangeVolume(t *testing.T) {
	t.Parallel()
	project := newProject("testapp",
		newService("app1").
			image("nginx:1.29").
			label(labels.LabelExchangeVolume, "- name: data\n  mountPath: /var/www").
			build(),
		newService("app2").
			image("foo:bar").
			label(labels.LabelSamePod, "app1").
			label(labels.LabelExchangeVolume, "- name: data\n  mountPath: /opt\n  init: cp -r /var/www /opt").
			build(),
	)

	output := renderProjectTemplate(t, project, "templates/app1/deployment.yaml")
	deployment := v1.Deployment{}
	if err := yaml.Unmarshal(output, &deployment); err != nil {
		t.Fatalf("%s\n%s", unmarshalError, err)
	}

	found := false
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == "exchange-data" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected an exchange-data volume, got %v", deployment.Spec.Template.Spec.Volumes)
	}

	mounted := 0
	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == "exchange-data" {
				mounted++
			}
		}
	}
	if mounted != 2 {
		t.Errorf("expected 2 mounts of exchange-data, got %d", mounted)
	}
}
