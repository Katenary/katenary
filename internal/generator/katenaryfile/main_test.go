package katenaryfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"katenary.io/internal/generator/labels"

	"github.com/compose-spec/compose-go/v2/cli"
)

func TestBuildSchema(t *testing.T) {
	sh := GenerateSchema()
	if len(sh) == 0 {
		t.Errorf("Expected schema to be defined")
	}
}

func TestOverrideProjectWithKatenaryFile(t *testing.T) {
	composeContent := `
services:
  webapp:
    image: nginx:latest
`

	katenaryfileContent := `
webapp:
  ports:
    - 80
`

	// create /tmp/katenary-test-override directory, save the compose.yaml file
	tmpDir, err := os.MkdirTemp("", "katenary-test-override")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %s", err.Error())
	}
	composeFile := filepath.Join(tmpDir, "compose.yaml")
	katenaryFile := filepath.Join(tmpDir, "katenary.yaml")

	os.MkdirAll(tmpDir, 0o755)
	if err := os.WriteFile(composeFile, []byte(composeContent), 0o644); err != nil {
		t.Log(err)
	}
	if err := os.WriteFile(katenaryFile, []byte(katenaryfileContent), 0o644); err != nil {
		t.Log(err)
	}
	defer os.RemoveAll(tmpDir)

	// chand dir to this directory
	os.Chdir(tmpDir)
	options, _ := cli.NewProjectOptions(nil,
		cli.WithWorkingDirectory(tmpDir),
		cli.WithDefaultConfigPath,
	)
	project, err := cli.ProjectFromOptions(context.TODO(), options)
	if err != nil {
		t.Fatalf("Failed to create project from options: %s", err.Error())
	}

	OverrideWithConfig(project)
	w := project.Services["webapp"].Labels
	if v, ok := w[labels.LabelPorts]; !ok {
		t.Fatal("Expected ports to be defined", v)
	}
}

func TestOverrideProjectWithIngress(t *testing.T) {
	composeContent := `
services:
  webapp:
    image: nginx:latest
`

	katenaryfileContent := `
webapp:
  ports:
    - 80
  ingress:
    port: 80
`

	// create /tmp/katenary-test-override directory, save the compose.yaml file
	tmpDir, err := os.MkdirTemp("", "katenary-test-override")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %s", err.Error())
	}
	composeFile := filepath.Join(tmpDir, "compose.yaml")
	katenaryFile := filepath.Join(tmpDir, "katenary.yaml")

	os.MkdirAll(tmpDir, 0o755)
	if err := os.WriteFile(composeFile, []byte(composeContent), 0o644); err != nil {
		t.Log(err)
	}
	if err := os.WriteFile(katenaryFile, []byte(katenaryfileContent), 0o644); err != nil {
		t.Log(err)
	}
	defer os.RemoveAll(tmpDir)

	// chand dir to this directory
	os.Chdir(tmpDir)
	options, _ := cli.NewProjectOptions(nil,
		cli.WithWorkingDirectory(tmpDir),
		cli.WithDefaultConfigPath,
	)
	project, err := cli.ProjectFromOptions(context.TODO(), options)
	if err != nil {
		t.Fatalf("Failed to create project from options: %s", err.Error())
	}

	OverrideWithConfig(project)
	w := project.Services["webapp"].Labels
	if v, ok := w[labels.LabelPorts]; !ok {
		t.Fatal("Expected ports to be defined", v)
	}
	if v, ok := w[labels.LabelIngress]; !ok {
		t.Fatal("Expected ingress to be defined", v)
	}
}

func TestOverrideConfigMapFiles(t *testing.T) {
	composeContent := `
services:
  webapp:
    image: nginx:latest
`

	katenaryfileContent := `
webapp:
  configmap-files:
    - foo/bar
  ports:
    - 80
  ingress:
    port: 80
`

	// create /tmp/katenary-test-override directory, save the compose.yaml file
	tmpDir, err := os.MkdirTemp("", "katenary-test-override")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %s", err.Error())
	}
	composeFile := filepath.Join(tmpDir, "compose.yaml")
	katenaryFile := filepath.Join(tmpDir, "katenary.yaml")

	os.MkdirAll(tmpDir, 0o755)
	if err := os.WriteFile(composeFile, []byte(composeContent), 0o644); err != nil {
		t.Log(err)
	}
	if err := os.WriteFile(katenaryFile, []byte(katenaryfileContent), 0o644); err != nil {
		t.Log(err)
	}
	defer os.RemoveAll(tmpDir)

	// chand dir to this directory
	os.Chdir(tmpDir)
	options, _ := cli.NewProjectOptions(nil,
		cli.WithWorkingDirectory(tmpDir),
		cli.WithDefaultConfigPath,
	)
	project, err := cli.ProjectFromOptions(context.TODO(), options)
	if err != nil {
		t.Fatalf("Failed to create project from options: %s", err.Error())
	}

	OverrideWithConfig(project)
	w := project.Services["webapp"].Labels
	if v, ok := w[labels.LabelConfigMapFiles]; !ok {
		t.Fatal("Expected configmap-files to be defined", v)
	}
}
