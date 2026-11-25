package generator

import (
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
)

func TestCheckOldLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		wantErr  bool
		contains []string
	}{
		{
			name:   "current labels",
			labels: map[string]string{"katenary.v4/ports": "80"},
		},
		{
			name:   "unrelated labels",
			labels: map[string]string{"example.com/ports": "80"},
		},
		{
			name:     "v3 label",
			labels:   map[string]string{"katenary.v3/ports": "80"},
			wantErr:  true,
			contains: []string{"katenary.v3/ports", "katenary.v4/ports"},
		},
		{
			name:     "v1/v2 label",
			labels:   map[string]string{"katenary.io/mapenv": "FOO"},
			wantErr:  true,
			contains: []string{"katenary.io/mapenv", "katenary.v4/mapenv"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &types.Project{
				Name: "test",
				Services: types.Services{
					"web": {Name: "web", Labels: tt.labels},
				},
			}
			err := checkOldLabels(project)
			if tt.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			for _, want := range tt.contains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err.Error(), want)
				}
			}
		})
	}
}
