package generator

import (
	"path/filepath"
	"testing"
)

// TestChartFixtures renders a set of representative compose fixtures once and
// compares the whole chart with a golden file. Run with -update to refresh the
// expected output.
func TestChartFixtures(t *testing.T) {
	fixtures := []string{
		"basic",
		"depends-on",
		"ingress",
		"ingressroute",
		"cronjob",
		"values",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			chartDir := filepath.Join(t.TempDir(), "chart")
			output := convertAndTemplate(t, testCompose(name), chartDir)
			assertGolden(t, output, testGolden(name))
		})
	}
}
