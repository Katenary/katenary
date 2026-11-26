package generator

import (
	"strconv"
	"strings"

	"katenary.io/internal/generator/labels"
	"katenary.io/internal/generator/labels/labelstructs"
	"katenary.io/internal/logger"

	"github.com/compose-spec/compose-go/v2/types"
	corev1 "k8s.io/api/core/v1"
)

// containerSecurityContext builds the container security context from the
// compose service and the security-context label. Privilege escalation is
// denied by default.
func containerSecurityContext(service types.ServiceConfig) *corev1.SecurityContext {
	allowEscalation := false
	context := &labelstructs.ContainerSecurityContext{
		AllowPrivilegeEscalation: &allowEscalation,
	}

	if service.Privileged {
		privileged := true
		context.Privileged = &privileged
		context.AllowPrivilegeEscalation = &privileged
	}
	if service.ReadOnly {
		readOnly := true
		context.ReadOnlyRootFilesystem = &readOnly
	}
	if len(service.CapAdd) > 0 || len(service.CapDrop) > 0 {
		context.Capabilities = &labelstructs.Capabilities{
			Add:  service.CapAdd,
			Drop: service.CapDrop,
		}
	}
	if uid, gid := parseComposeUser(service.User); uid != nil || gid != nil {
		context.RunAsUser = uid
		context.RunAsGroup = gid
		if uid != nil && *uid != 0 {
			nonRoot := true
			context.RunAsNonRoot = &nonRoot
		}
	}

	if raw, ok := service.Labels[labels.LabelSecurityContext]; ok {
		security, err := labelstructs.SecurityContextFrom(raw)
		if err != nil {
			logger.Fatalf("error parsing the security-context label: %s", err)
		}
		if security.Container != nil {
			mergeContainerSecurityContext(context, security.Container)
		}
	}

	return toKubernetesContainerSecurityContext(context)
}

// podSecurityContext builds the pod security context from the compose service
// and the security-context label. It returns nil when no option is set.
func podSecurityContext(service types.ServiceConfig) *corev1.PodSecurityContext {
	context := &labelstructs.PodSecurityContext{}

	for _, group := range service.GroupAdd {
		if gid, err := strconv.ParseInt(group, 10, 64); err == nil {
			context.SupplementalGroups = append(context.SupplementalGroups, gid)
		}
	}
	for name, value := range service.Sysctls {
		context.Sysctls = append(context.Sysctls, labelstructs.Sysctl{Name: name, Value: value})
	}

	if raw, ok := service.Labels[labels.LabelSecurityContext]; ok {
		security, err := labelstructs.SecurityContextFrom(raw)
		if err != nil {
			logger.Fatalf("error parsing the security-context label: %s", err)
		}
		if security.Pod != nil {
			mergePodSecurityContext(context, security.Pod)
		}
	}

	return toKubernetesPodSecurityContext(context)
}

func mergeContainerSecurityContext(dst, src *labelstructs.ContainerSecurityContext) {
	if src.AllowPrivilegeEscalation != nil {
		dst.AllowPrivilegeEscalation = src.AllowPrivilegeEscalation
	}
	if src.Privileged != nil {
		dst.Privileged = src.Privileged
	}
	if src.ReadOnlyRootFilesystem != nil {
		dst.ReadOnlyRootFilesystem = src.ReadOnlyRootFilesystem
	}
	if src.RunAsUser != nil {
		dst.RunAsUser = src.RunAsUser
	}
	if src.RunAsGroup != nil {
		dst.RunAsGroup = src.RunAsGroup
	}
	if src.RunAsNonRoot != nil {
		dst.RunAsNonRoot = src.RunAsNonRoot
	}
	if src.Capabilities != nil {
		dst.Capabilities = src.Capabilities
	}
	if src.SeccompProfile != nil {
		dst.SeccompProfile = src.SeccompProfile
	}
}

func mergePodSecurityContext(dst, src *labelstructs.PodSecurityContext) {
	if src.RunAsUser != nil {
		dst.RunAsUser = src.RunAsUser
	}
	if src.RunAsGroup != nil {
		dst.RunAsGroup = src.RunAsGroup
	}
	if src.RunAsNonRoot != nil {
		dst.RunAsNonRoot = src.RunAsNonRoot
	}
	if src.FSGroup != nil {
		dst.FSGroup = src.FSGroup
	}
	if src.FSGroupChangePolicy != nil {
		dst.FSGroupChangePolicy = src.FSGroupChangePolicy
	}
	if len(src.SupplementalGroups) > 0 {
		dst.SupplementalGroups = src.SupplementalGroups
	}
	if src.SeccompProfile != nil {
		dst.SeccompProfile = src.SeccompProfile
	}
	if len(src.Sysctls) > 0 {
		dst.Sysctls = src.Sysctls
	}
}

func toKubernetesContainerSecurityContext(context *labelstructs.ContainerSecurityContext) *corev1.SecurityContext {
	security := &corev1.SecurityContext{
		AllowPrivilegeEscalation: context.AllowPrivilegeEscalation,
		Privileged:               context.Privileged,
		ReadOnlyRootFilesystem:   context.ReadOnlyRootFilesystem,
		RunAsUser:                context.RunAsUser,
		RunAsGroup:               context.RunAsGroup,
		RunAsNonRoot:             context.RunAsNonRoot,
		SeccompProfile:           toKubernetesSeccompProfile(context.SeccompProfile),
	}
	if context.Capabilities != nil {
		security.Capabilities = &corev1.Capabilities{}
		for _, capability := range context.Capabilities.Add {
			security.Capabilities.Add = append(security.Capabilities.Add, corev1.Capability(capability))
		}
		for _, capability := range context.Capabilities.Drop {
			security.Capabilities.Drop = append(security.Capabilities.Drop, corev1.Capability(capability))
		}
	}
	return security
}

func toKubernetesPodSecurityContext(context *labelstructs.PodSecurityContext) *corev1.PodSecurityContext {
	if context.RunAsUser == nil && context.RunAsGroup == nil && context.RunAsNonRoot == nil &&
		context.FSGroup == nil && context.FSGroupChangePolicy == nil &&
		len(context.SupplementalGroups) == 0 && context.SeccompProfile == nil &&
		len(context.Sysctls) == 0 {
		return nil
	}

	security := &corev1.PodSecurityContext{
		RunAsUser:          context.RunAsUser,
		RunAsGroup:         context.RunAsGroup,
		RunAsNonRoot:       context.RunAsNonRoot,
		FSGroup:            context.FSGroup,
		SupplementalGroups: context.SupplementalGroups,
		SeccompProfile:     toKubernetesSeccompProfile(context.SeccompProfile),
	}
	if context.FSGroupChangePolicy != nil {
		policy := corev1.PodFSGroupChangePolicy(*context.FSGroupChangePolicy)
		security.FSGroupChangePolicy = &policy
	}
	for _, sysctl := range context.Sysctls {
		security.Sysctls = append(security.Sysctls, corev1.Sysctl{
			Name:  sysctl.Name,
			Value: sysctl.Value,
		})
	}
	return security
}

func toKubernetesSeccompProfile(profile *labelstructs.SeccompProfile) *corev1.SeccompProfile {
	if profile == nil {
		return nil
	}
	return &corev1.SeccompProfile{
		Type:             corev1.SeccompProfileType(profile.Type),
		LocalhostProfile: profile.LocalhostProfile,
	}
}

// parseComposeUser parses a compose user ("uid" or "uid:gid"). Named users are
// ignored because Kubernetes needs numeric IDs.
func parseComposeUser(user string) (uid, gid *int64) {
	if user == "" {
		return nil, nil
	}
	parts := strings.SplitN(user, ":", 2)
	if value, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
		uid = &value
	}
	if len(parts) == 2 {
		if value, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			gid = &value
		}
	}
	return uid, gid
}
