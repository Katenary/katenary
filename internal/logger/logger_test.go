package logger

import (
	"testing"
)

func TestIcons(t *testing.T) {
	tests := []struct {
		name     string
		got      Icon
		expected Icon
	}{
		{"IconSuccess", IconSuccess, "✅"},
		{"IconFailure", IconFailure, "❌"},
		{"IconWarning", IconWarning, "❕"},
		{"IconNote", IconNote, "📝"},
		{"IconWorld", IconWorld, "🌐"},
		{"IconPlug", IconPlug, "🔌"},
		{"IconPackage", IconPackage, "📦"},
		{"IconCabinet", IconCabinet, "🗄️"},
		{"IconInfo", IconInfo, "🔵"},
		{"IconSecret", IconSecret, "🔒"},
		{"IconConfig", IconConfig, "🔧"},
		{"IconDependency", IconDependency, "🔗"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %q, want %q", tt.got, tt.expected)
			}
		})
	}
}

func TestInfo(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Info panicked: %v", r)
		}
	}()
	Info("test message")
}

func TestWarn(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Warn panicked: %v", r)
		}
	}()
	Warn("test warning")
}

func TestSuccess(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Success panicked: %v", r)
		}
	}()
	Success("test success")
}

func TestFailure(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Failure panicked: %v", r)
		}
	}()
	Failure("test failure")
}

func TestLog(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Log panicked: %v", r)
		}
	}()
	Log(IconInfo, "test log")
}

func TestWarningsCollection(t *testing.T) {
	// Clear any existing warnings
	warnings = nil

	// Add some warnings
	Warn("test warning 1")
	Warnf("test warning 2: %s", "value")

	// Check that warnings were collected
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(warnings))
	}

	// Check the content of warnings
	if warnings[0] != "test warning 1" {
		t.Errorf("expected 'test warning 1', got '%s'", warnings[0])
	}
	if warnings[1] != "test warning 2: value" {
		t.Errorf("expected 'test warning 2: value', got '%s'", warnings[1])
	}
}
