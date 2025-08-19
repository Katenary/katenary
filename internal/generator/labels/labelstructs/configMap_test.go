package labelstructs_test

import (
	"testing"

	"katenary.io/internal/generator/labels/labelstructs"
)

func TestConfigMapFileFrom(t *testing.T) {
	ts := "- foo/bar"
	tc2, _ := labelstructs.ConfigMapFileFrom(ts)
	if len(tc2) != 1 {
		t.Errorf("Expected ConfigMapFile to have 1 item, got %d", len(tc2))
	}
	if tc2[0] != "foo/bar" {
		t.Errorf("Expected ConfigMapFile to contain 'foo/bar', got %s", tc2[0])
	}
}
