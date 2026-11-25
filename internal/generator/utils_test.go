package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"katenary.io/internal/generator/labels"
	"katenary.io/internal/utils"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
)

func TestHasExposedPorts(t *testing.T) {
	tests := []struct {
		name    string
		service types.ServiceConfig
		want    bool
	}{
		{
			name:    "no ports",
			service: types.ServiceConfig{},
			want:    false,
		},
		{
			name:    "compose ports",
			service: types.ServiceConfig{Ports: []types.ServicePortConfig{{Target: 80}}},
			want:    true,
		},
		{
			name:    "ports label as list",
			service: types.ServiceConfig{Labels: map[string]string{labels.LabelPorts: "[80, 443]"}},
			want:    true,
		},
		{
			name:    "ports label as comma separated string",
			service: types.ServiceConfig{Labels: map[string]string{labels.LabelPorts: "80,443"}},
			want:    true,
		},
		{
			name:    "empty ports label",
			service: types.ServiceConfig{Labels: map[string]string{labels.LabelPorts: ""}},
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasExposedPorts(tt.service); got != tt.want {
				t.Errorf("HasExposedPorts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitPorts(t *testing.T) {
	composeFileContent := `
services:
  foo:
    image: nginx:latest
    labels:
      %[1]s/ports: 80,443
`
	composeFileContent = fmt.Sprintf(composeFileContent, labels.KatenaryLabelPrefix)
	tmpDir, err := os.MkdirTemp("", "katenary-test-override")
	if err != nil {
		t.Fatal(err.Error())
	}
	composeFile := filepath.Join(tmpDir, "compose.yaml")

	os.MkdirAll(tmpDir, utils.DirectoryPermission)
	if err := os.WriteFile(composeFile, []byte(composeFileContent), 0o644); err != nil {
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
		t.Fatal(err)
	}
	s := project.Services["foo"]
	if err := fixPorts(&s); err != nil {
		t.Errorf("Expected no error, got %s", err)
	}
	project.Services["foo"] = s
	found := 0
	for _, p := range project.Services["foo"].Ports {
		switch p.Target {
		case 80, 443:
			found++
		}
	}
	if found != 2 {
		t.Errorf("Expected 2 ports, got %d", found)
	}
}

func TestSplitPortsWithDefault(t *testing.T) {
	composeFileContent := `
services:
  foo:
    image: nginx:latest
    ports:
      - 8080
    labels:
      %[1]s/ports: 80,443
`
	composeFileContent = fmt.Sprintf(composeFileContent, labels.KatenaryLabelPrefix)
	tmpDir, err := os.MkdirTemp("", "katenary-test-override")
	if err != nil {
		t.Fatal(err)
	}
	composeFile := filepath.Join(tmpDir, "compose.yaml")

	os.MkdirAll(tmpDir, utils.DirectoryPermission)
	if err := os.WriteFile(composeFile, []byte(composeFileContent), 0o644); err != nil {
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
		t.Fatal(err)
	}
	s := project.Services["foo"]
	if err := fixPorts(&s); err != nil {
		t.Errorf("Expected no error, got %s", err)
	}
	project.Services["foo"] = s
	found := 0
	for _, p := range project.Services["foo"].Ports {
		switch p.Target {
		case 80, 443, 8080:
			found++
		}
	}
	if found != 3 {
		t.Errorf("Expected 3 ports, got %d", found)
	}
}
