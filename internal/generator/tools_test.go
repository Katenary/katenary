package generator

import (
	"os/exec"
)

const unmarshalError = "Failed to unmarshal the output: %s"

func helmTemplate(options ConvertOptions, arguments ...string) (string, error) {
	// Pin the namespace: in CI the step runs inside the cluster and Helm would
	// otherwise use the ambient namespace (cicd) instead of the "default" one.
	args := []string{"template", options.OutputDir, "--namespace", "default"}
	args = append(args, arguments...)

	cmd := exec.Command("helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}
