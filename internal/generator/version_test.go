package generator

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	expected := "v1.0.0"

	errorMessage := func(value, expected any) string {
		return "Expected version to be " + expected.(string) + ", got " + value.(string)
	}

	// first, let's test the default behavior
	// when we compile from source, without setting the Version variable,

	// by default, when we are developing, the Version variable is set to "master"
	// we build on "devel" branch
	v := GetVersion()
	// by default, the version comes from build info and it's a development version
	if !strings.Contains(v, "(devel)") {
		errorMessage(v, "(devel)")
	}

	// now, imagine we are on a release branch
	Version = "1.0.0"
	v = GetVersion()
	if !strings.Contains(v, expected) {
		errorMessage(v, expected)
	}
	// now, imagine we are on v1.0.0
	Version = "v1.0.0"
	v = GetVersion()
	if !strings.Contains(v, expected) {
		errorMessage(v, expected)
	}
	// we can also compile a release branch
	Version = "release-1.0.0"
	v = GetVersion()
	if !strings.Contains(v, expected) {
		errorMessage(v, expected)
	}

	// for releases-* tags
	Version = "release-1.0.0"
	v = GetVersion()
	if !strings.Contains(v, expected) {
		errorMessage(v, expected)
	}

	// and for releases/* tags
	Version = "releases/1.0.0"
	v = GetVersion()
	if !strings.Contains(v, expected) {
		errorMessage(v, expected)
	}
}
