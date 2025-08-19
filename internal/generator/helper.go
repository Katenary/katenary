package generator

import (
	_ "embed"
	"strings"

	"katenary.io/internal/generator/labels"
)

// helmHelper is a template for the _helpers.tpl file in the chart templates directory.
//
//go:embed helmHelper.tpl
var helmHelper string

// Helper returns the _helpers.tpl file for a chart.
func Helper(name string) string {
	helmHelper := strings.ReplaceAll(helmHelper, "__APP__", name)
	helmHelper = strings.ReplaceAll(helmHelper, "__PREFIX__", labels.KatenaryLabelPrefix)
	helmHelper = strings.ReplaceAll(helmHelper, "__VERSION__", "0.1.0")
	return helmHelper
}
