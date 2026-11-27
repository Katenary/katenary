package labelstructs

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConditionFromExpression(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected string
		paths    []string
	}{
		{
			name:     "single path",
			raw:      "proxy.enabled",
			expected: ".Values.proxy.enabled",
			paths:    []string{"proxy.enabled"},
		},
		{
			name:     "negation",
			raw:      "!proxy.enabled",
			expected: "not .Values.proxy.enabled",
			paths:    []string{"proxy.enabled"},
		},
		{
			name:     "and has priority over or",
			raw:      "a || b && c",
			expected: "or .Values.a (and .Values.b .Values.c)",
			paths:    []string{"a", "b", "c"},
		},
		{
			name:     "parentheses override priority",
			raw:      "(a || b) && c",
			expected: "and (or .Values.a .Values.b) .Values.c",
			paths:    []string{"a", "b", "c"},
		},
		{
			name:     "negation and or",
			raw:      "!proxy.enabled || tls.enabled",
			expected: "or (not .Values.proxy.enabled) .Values.tls.enabled",
			paths:    []string{"proxy.enabled", "tls.enabled"},
		},
		{
			name:     "double negation",
			raw:      "!!debug.enabled",
			expected: "not (not .Values.debug.enabled)",
			paths:    []string{"debug.enabled"},
		},
		{
			name:     "word aliases",
			raw:      "not a and b or c",
			expected: "or (and (not .Values.a) .Values.b) .Values.c",
			paths:    []string{"a", "b", "c"},
		},
		{
			name:     "literals",
			raw:      "true && false",
			expected: "and true false",
			paths:    nil,
		},
		{
			name:     "keyword prefix in path",
			raw:      "not.ready",
			expected: ".Values.not.ready",
			paths:    []string{"not.ready"},
		},
		{
			name:     "deep path",
			raw:      "features.proxy.enabled",
			expected: ".Values.features.proxy.enabled",
			paths:    []string{"features.proxy.enabled"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			condition, err := ConditionFrom(test.raw)
			if err != nil {
				t.Fatalf("ConditionFrom(%q) failed: %s", test.raw, err)
			}
			if got := condition.HelmExpr(); got != test.expected {
				t.Errorf("HelmExpr() = %q, want %q", got, test.expected)
			}
			assertPaths(t, condition.Paths(), test.paths)
		})
	}
}

func TestConditionFromObject(t *testing.T) {
	raw := `condition: proxy.enabled && !debug.enabled
defaults:
  proxy.enabled: true
  debug.enabled: false
`

	condition, err := ConditionFrom(raw)
	if err != nil {
		t.Fatalf("ConditionFrom() failed: %s", err)
	}

	expected := "and .Values.proxy.enabled (not .Values.debug.enabled)"
	if got := condition.HelmExpr(); got != expected {
		t.Errorf("HelmExpr() = %q, want %q", got, expected)
	}
	if !condition.Default("proxy.enabled") {
		t.Error("proxy.enabled should default to true")
	}
	if condition.Default("debug.enabled") {
		t.Error("debug.enabled should default to false")
	}
	// Flags without an explicit default are enabled.
	if !condition.Default("unknown.flag") {
		t.Error("unknown flags should default to true")
	}
	if unused := condition.UnusedDefaults(); len(unused) != 0 {
		t.Errorf("UnusedDefaults() = %v, want empty", unused)
	}
}

func TestConditionUnusedDefaults(t *testing.T) {
	raw := `condition: proxy.enabled
defaults:
  proxy.enabled: true
  typo.enabled: false
`

	condition, err := ConditionFrom(raw)
	if err != nil {
		t.Fatalf("ConditionFrom() failed: %s", err)
	}

	unused := condition.UnusedDefaults()
	if len(unused) != 1 || unused[0] != "typo.enabled" {
		t.Errorf("UnusedDefaults() = %v, want [typo.enabled]", unused)
	}
}

func TestConditionLabelFromKatenaryFile(t *testing.T) {
	var stringForm ConditionLabel
	if err := yaml.Unmarshal([]byte("proxy.enabled"), &stringForm); err != nil {
		t.Fatalf("cannot unmarshal the string form: %s", err)
	}
	if stringForm.Condition != "proxy.enabled" {
		t.Fatalf("condition = %q, want proxy.enabled", stringForm.Condition)
	}

	// getLabelContent marshals the field back to a label value.
	raw, err := yaml.Marshal(&stringForm)
	if err != nil {
		t.Fatalf("cannot marshal the label: %s", err)
	}
	condition, err := ConditionFrom(string(raw))
	if err != nil {
		t.Fatalf("ConditionFrom() failed on the marshaled value: %s", err)
	}
	if got := condition.HelmExpr(); got != ".Values.proxy.enabled" {
		t.Errorf("HelmExpr() = %q, want .Values.proxy.enabled", got)
	}

	var objectForm ConditionLabel
	raw = []byte("condition: a && !b\ndefaults:\n  b: false\n")
	if err := yaml.Unmarshal(raw, &objectForm); err != nil {
		t.Fatalf("cannot unmarshal the object form: %s", err)
	}
	if objectForm.Condition != "a && !b" || objectForm.Defaults["b"] {
		t.Errorf("unexpected object form: %+v", objectForm)
	}
}

func TestConditionFromErrors(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		message string
	}{
		{name: "empty", raw: "   ", message: "empty"},
		{name: "trailing operator", raw: "a &&", message: "unexpected end of expression"},
		{name: "missing closing paren", raw: "(a || b", message: `missing ")"`},
		{name: "two expressions", raw: "a b", message: "unexpected value"},
		{name: "single ampersand", raw: "a & b", message: `unexpected '&'`},
		{name: "invalid character", raw: "a @ b", message: `unexpected character '@'`},
		{name: "invalid path", raw: "a..b", message: `invalid value path "a..b"`},
		{name: "path starting with digit", raw: "1abc", message: "unexpected character"},
		{name: "unsupported operator", raw: "a == b", message: `unexpected character '='`},
		{name: "unknown object key", raw: "condition: a\nexpr: b\n", message: "field expr not found"},
		{name: "missing condition", raw: "defaults:\n  a: true\n", message: `missing "condition"`},
		{name: "default not boolean", raw: "condition: a\ndefaults:\n  a: nope\n", message: "cannot unmarshal"},
		{name: "invalid default path", raw: "condition: a\ndefaults:\n  a..b: true\n", message: `invalid default path "a..b"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ConditionFrom(test.raw)
			if err == nil {
				t.Fatalf("ConditionFrom(%q) should fail", test.raw)
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Errorf("error %q does not contain %q", err, test.message)
			}
		})
	}
}

func assertPaths(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("paths = %v, want %v", got, want)
		}
	}
}
