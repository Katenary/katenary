package labels

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"katenary.io/internal/utils"
)

// LegacyLabelsError returns the blocking error shown when a project still uses
// labels from a previous Katenary version. badLabels is a list of human
// readable lines such as "- web: katenary.v3/ports -> katenary.v4/ports".
func LegacyLabelsError(projectName string, badLabels []string) error {
	sort.Strings(badLabels)
	message := fmt.Sprintf(`Old labels detected in project "%s".

Katenary v4 uses the label prefix "%s/".
Labels from previous versions are not read anymore, convert them first:

%s

Some labels were renamed between v1/v2 and v3, see the migration guide:
https://katenary.readthedocs.io/latest/migrate-v4/`,
		projectName,
		KatenaryLabelPrefix,
		strings.Join(badLabels, "\n"),
	)

	return errors.New(utils.WordWrap(message, 80))
}
