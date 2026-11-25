package labels

import (
	_ "embed"
	"reflect"
	"testing"
)

var testingKatenaryPrefix = Prefix()

const mainAppLabel = "main-app"

func TestPrefix(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "TestPrefix",
			want: "katenary.v4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Prefix(); got != tt.want {
				t.Errorf("Prefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLabelName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want Label
	}{
		{
			name: "Test_labelName",
			args: args{
				name: mainAppLabel,
			},
			want: testingKatenaryPrefix + "/" + mainAppLabel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LabelName(tt.args.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("labelName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetLabelHelp(t *testing.T) {
	help := GetLabelHelp(false)
	if help == "" {
		t.Errorf("GetLabelHelp() = %v, want %v", help, "Help")
	}
	help = GetLabelHelp(true)
	if help == "" {
		t.Errorf("GetLabelHelp() = %v, want %v", help, "Help")
	}
}

func TestGetLabelHelpFor(t *testing.T) {
	help := GetLabelHelpFor(mainAppLabel, false)
	if help == "" {
		t.Errorf("GetLabelHelpFor() = %v, want %v", help, "Help")
	}
	help = GetLabelHelpFor("main-app", true)
	if help == "" {
		t.Errorf("GetLabelHelpFor() = %v, want %v", help, "Help")
	}
}

func TestIsLegacyLabel(t *testing.T) {
	tests := []struct {
		label string
		want  bool
	}{
		{"katenary.io/ports", true},
		{"katenary.v1/ports", true},
		{"katenary.v2/ports", true},
		{"katenary.v3/ports", true},
		{"katenary.v4/ports", false},
		{"example.com/ports", false},
		{"ports", false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := IsLegacyLabel(tt.label); got != tt.want {
				t.Errorf("IsLegacyLabel(%q) = %v, want %v", tt.label, got, tt.want)
			}
		})
	}
}

func TestLegacyLabelSuggestion(t *testing.T) {
	tests := map[string]string{
		"katenary.v3/ports":   "katenary.v4/ports",
		"katenary.io/ingress": "katenary.v4/ingress",
		"katenary.v2/mapenv":  "katenary.v4/mapenv",
	}
	for legacy, want := range tests {
		if got := LegacyLabelSuggestion(legacy); got != want {
			t.Errorf("LegacyLabelSuggestion(%q) = %q, want %q", legacy, got, want)
		}
	}
}
