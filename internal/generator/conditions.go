package generator

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"katenary.io/internal/generator/labels"
	"katenary.io/internal/generator/labels/labelstructs"
	"katenary.io/internal/logger"

	"github.com/compose-spec/compose-go/v2/types"
)

// serviceConditions maps a service name to its parsed katenary.v4/if label.
type serviceConditions map[string]*labelstructs.Condition

// applyConditions reads the katenary.v4/if labels, injects the referenced flags
// in values.yaml and wraps every resource generated for the service in a Helm
// condition.
func (chart *HelmChart) applyConditions(project *types.Project) error {
	conditions, err := collectConditions(project)
	if err != nil {
		return err
	}
	if len(conditions) == 0 {
		return nil
	}

	effective := effectiveConditions(*project, conditions)
	warnConditionDependencies(*project, conditions)

	if err := chart.injectConditionDefaults(effective); err != nil {
		return err
	}
	chart.wrapConditionalTemplates(effective)

	return nil
}

// effectiveConditions resolves the condition applied to each service. A
// service merged into another deployment with same-pod does not own its
// resources anymore: its remaining templates (PVCs) follow the condition of
// the target service.
func effectiveConditions(project types.Project, conditions serviceConditions) serviceConditions {
	effective := serviceConditions{}

	for name, condition := range conditions {
		service := project.Services[name]
		if target, ok := service.Labels[labels.LabelSamePod]; ok && target != "" {
			if targetCondition, ok := conditions[target]; ok {
				effective[name] = targetCondition
			}
			continue
		}
		effective[name] = condition
	}

	return effective
}

// collectConditions parses the katenary.v4/if label of every service.
func collectConditions(project *types.Project) (serviceConditions, error) {
	conditions := serviceConditions{}

	for name, service := range project.Services {
		raw, ok := service.Labels[labels.LabelIf]
		if !ok {
			continue
		}
		condition, err := labelstructs.ConditionFrom(raw)
		if err != nil {
			return nil, fmt.Errorf("service %s: invalid %s label: %w", name, labels.LabelIf, err)
		}
		conditions[name] = condition
	}

	return conditions, nil
}

// injectConditionDefaults writes the condition flags in values.yaml. Existing
// values are preserved so that a flag can be shared between services.
func (chart *HelmChart) injectConditionDefaults(conditions serviceConditions) error {
	flags := map[string]bool{}

	for _, name := range slices.Sorted(maps.Keys(conditions)) {
		condition := conditions[name]
		for _, path := range condition.Paths() {
			value := condition.Default(path)
			if existing, ok := flags[path]; ok && existing != value {
				return fmt.Errorf(
					"service %s: conflicting default value for %q in %s label",
					name, path, labels.LabelIf,
				)
			}
			flags[path] = value
		}
	}

	for _, path := range slices.Sorted(maps.Keys(flags)) {
		if err := setValueDefault(chart.Values, path, flags[path]); err != nil {
			return fmt.Errorf("%s: %w", labels.LabelIf, err)
		}
	}

	chart.ConditionFlags = flags
	return nil
}

// setValueDefault creates the nested values.yaml path when it does not exist.
// It fails when the path collides with an existing non-map value (a service
// name for example).
func setValueDefault(values map[string]any, path string, defaultValue bool) error {
	segments := strings.Split(path, ".")
	current := values

	for i, segment := range segments {
		last := i == len(segments)-1
		existing, exists := current[segment]

		if last {
			if !exists {
				current[segment] = defaultValue
				return nil
			}
			if _, isBool := existing.(bool); !isBool {
				return fmt.Errorf(
					"values path %q already exists and is not a boolean (choose another prefix, e.g. \"features.%s\")",
					path, path,
				)
			}
			return nil
		}

		if !exists {
			child := map[string]any{}
			current[segment] = child
			current = child
			continue
		}

		child, isMap := existing.(map[string]any)
		if !isMap {
			return fmt.Errorf(
				"values path %q conflicts with the existing %q value (choose another prefix, e.g. \"features.%s\")",
				path, segment, path,
			)
		}
		current = child
	}

	return nil
}

// wrapConditionalTemplates wraps every template owned by a conditioned service
// in a Helm "if" action. All resources of the service are covered: deployment,
// service, ingress, configmap, secret, PVC, network policy, cronjob and RBAC.
func (chart *HelmChart) wrapConditionalTemplates(conditions serviceConditions) {
	for _, template := range chart.Templates {
		condition, ok := conditions[template.Servicename]
		if !ok {
			continue
		}

		content := []byte("{{- if " + condition.HelmExpr() + " }}\n")
		content = append(content, template.Content...)
		if len(template.Content) > 0 && template.Content[len(template.Content)-1] != '\n' {
			content = append(content, '\n')
		}
		content = append(content, "{{- end }}\n"...)
		template.Content = content
	}
}

// warnConditionDependencies reports services whose resources may break when a
// related service is disabled. Conditions are not propagated automatically.
func warnConditionDependencies(project types.Project, conditions serviceConditions) {
	for _, name := range slices.Sorted(maps.Keys(project.Services)) {
		service := project.Services[name]
		own := conditions[name]

		target, isSamePod := service.Labels[labels.LabelSamePod]
		if isSamePod && target != "" {
			if own != nil {
				logger.Warnf(
					"service %s: %s is not applied to the merged resources; they follow the condition of %s",
					name, labels.LabelIf, target,
				)
			}
			continue
		}

		for _, dependency := range service.GetDependencies() {
			if dependencyConditionDiffers(own, conditions[dependency]) {
				logger.Warnf(
					"service %s depends on %s but their %s conditions differ; disabling one may block the other",
					name, dependency, labels.LabelIf,
				)
			}
		}

		if raw, ok := service.Labels[labels.LabelEnvFrom]; ok {
			from, err := labelstructs.EnvFromFrom(raw)
			if err == nil {
				for _, dependency := range from {
					if dependencyConditionDiffers(own, conditions[dependency]) {
						logger.Warnf(
							"service %s reads the environment of %s but their %s conditions differ; the configmap may be missing",
							name, dependency, labels.LabelIf,
						)
					}
				}
			}
		}

		if raw, ok := service.Labels[labels.LabelValuesFrom]; ok {
			mapping, err := labelstructs.GetValueFrom(raw)
			if err == nil {
				for _, binding := range *mapping {
					dependency := strings.SplitN(binding, ".", 2)[0]
					if dependencyConditionDiffers(own, conditions[dependency]) {
						logger.Warnf(
							"service %s binds values from %s but their %s conditions differ; the configmap or secret may be missing",
							name, dependency, labels.LabelIf,
						)
					}
				}
			}
		}
	}
}

// dependencyConditionDiffers reports whether a conditional dependency can be
// disabled while the dependent service stays enabled. An unconditional
// dependency never breaks the dependent service.
func dependencyConditionDiffers(own, dependency *labelstructs.Condition) bool {
	if dependency == nil {
		return false
	}
	return conditionsDiffer(own, dependency)
}

// conditionsDiffer reports whether two services do not share the same
// condition. A nil condition means the service is always enabled.
func conditionsDiffer(a, b *labelstructs.Condition) bool {
	switch {
	case a == nil && b == nil:
		return false
	case a == nil || b == nil:
		return true
	default:
		return a.HelmExpr() != b.HelmExpr()
	}
}
