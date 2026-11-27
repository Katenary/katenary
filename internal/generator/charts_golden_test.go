package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestChartFixtures renders a set of representative compose fixtures once and
// compares the whole chart with a golden file. It also snapshots the unrendered
// template files: rendering with the default values evaluates the guards to
// true and would not catch a missing or broken condition.
// Run with -update to refresh the expected output.
func TestChartFixtures(t *testing.T) {
	fixtures := []string{
		"basic",
		"depends-on",
		"ingress",
		"ingressroute",
		"networkpolicy",
		"cronjob",
		"security",
		"values",
		"if",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			chartDir := filepath.Join(t.TempDir(), "chart")
			output := convertAndTemplate(t, testCompose(name), chartDir)
			assertGolden(t, output, testGolden(name))
			assertGolden(t, chartTemplateSources(t, chartDir), testTemplatesGolden(name))
		})
	}
}

// chartTemplateSources concatenates the generated template files, with their
// path in a comment, so the Helm source can be compared with a golden file.
func chartTemplateSources(t *testing.T, chartDir string) []byte {
	t.Helper()

	templateDir := filepath.Join(chartDir, "templates")
	var files []string
	err := filepath.WalkDir(templateDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cannot list the generated templates: %s", err)
	}
	slices.Sort(files)

	var content strings.Builder
	for _, file := range files {
		rel, err := filepath.Rel(chartDir, file)
		if err != nil {
			t.Fatalf("cannot compute the relative path of %s: %s", file, err)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("cannot read %s: %s", file, err)
		}
		fmt.Fprintf(&content, "# Source: %s\n%s\n", filepath.ToSlash(rel), data)
	}
	return []byte(content.String())
}
