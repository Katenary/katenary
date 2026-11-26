package generator

import (
	"testing"

	"katenary.io/internal/generator/labels"

	corev1 "k8s.io/api/core/v1"
)

func TestContainerSecurityContextDefaults(t *testing.T) {
	service := newService("web").image("nginx").build()

	security := containerSecurityContext(service)
	if security == nil {
		t.Fatal("expected a container security context")
	}
	if security.AllowPrivilegeEscalation == nil || *security.AllowPrivilegeEscalation {
		t.Error("allowPrivilegeEscalation must default to false")
	}
}

func TestContainerSecurityContextFromCompose(t *testing.T) {
	service := newService("web").
		image("nginx").
		build()
	service.Privileged = true
	service.ReadOnly = true
	service.User = "1000:1001"
	service.CapAdd = []string{"NET_ADMIN"}
	service.CapDrop = []string{"ALL"}

	security := containerSecurityContext(service)

	if security.Privileged == nil || !*security.Privileged {
		t.Error("expected privileged")
	}
	if security.AllowPrivilegeEscalation == nil || !*security.AllowPrivilegeEscalation {
		t.Error("privileged implies allowPrivilegeEscalation")
	}
	if security.ReadOnlyRootFilesystem == nil || !*security.ReadOnlyRootFilesystem {
		t.Error("read_only should set readOnlyRootFilesystem")
	}
	if security.RunAsUser == nil || *security.RunAsUser != 1000 {
		t.Errorf("expected runAsUser 1000, got %v", security.RunAsUser)
	}
	if security.RunAsGroup == nil || *security.RunAsGroup != 1001 {
		t.Errorf("expected runAsGroup 1001, got %v", security.RunAsGroup)
	}
	if security.RunAsNonRoot == nil || !*security.RunAsNonRoot {
		t.Error("a non-root user should set runAsNonRoot")
	}
	if security.Capabilities == nil || len(security.Capabilities.Add) != 1 || len(security.Capabilities.Drop) != 1 {
		t.Errorf("unexpected capabilities: %+v", security.Capabilities)
	}
}

func TestContainerSecurityContextFromLabel(t *testing.T) {
	service := newService("web").
		image("nginx").
		label(labels.LabelSecurityContext, `
pod:
  fsGroup: 1000
  fsGroupChangePolicy: OnRootMismatch
  supplementalGroups: [1000, 1001]
  seccompProfile:
    type: RuntimeDefault
container:
  allowPrivilegeEscalation: true
  runAsNonRoot: true
  readOnlyRootFilesystem: true
  capabilities:
    drop: [ALL]
  seccompProfile:
    type: RuntimeDefault
`).
		build()

	security := containerSecurityContext(service)
	if security.AllowPrivilegeEscalation == nil || !*security.AllowPrivilegeEscalation {
		t.Error("the label must override allowPrivilegeEscalation")
	}
	if security.ReadOnlyRootFilesystem == nil || !*security.ReadOnlyRootFilesystem {
		t.Error("expected readOnlyRootFilesystem")
	}
	if security.RunAsNonRoot == nil || !*security.RunAsNonRoot {
		t.Error("expected runAsNonRoot")
	}
	if security.SeccompProfile == nil || security.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("unexpected seccomp profile: %+v", security.SeccompProfile)
	}

	pod := podSecurityContext(service)
	if pod == nil {
		t.Fatal("expected a pod security context")
	}
	if pod.FSGroup == nil || *pod.FSGroup != 1000 {
		t.Errorf("expected fsGroup 1000, got %v", pod.FSGroup)
	}
	if pod.FSGroupChangePolicy == nil || string(*pod.FSGroupChangePolicy) != "OnRootMismatch" {
		t.Errorf("unexpected fsGroupChangePolicy: %v", pod.FSGroupChangePolicy)
	}
	if len(pod.SupplementalGroups) != 2 {
		t.Errorf("expected 2 supplemental groups, got %v", pod.SupplementalGroups)
	}
}

func TestPodSecurityContextFromCompose(t *testing.T) {
	service := newService("web").image("nginx").build()
	service.GroupAdd = []string{"1000", "1001"}

	pod := podSecurityContext(service)
	if pod == nil || len(pod.SupplementalGroups) != 2 {
		t.Fatalf("expected 2 supplemental groups, got %+v", pod)
	}
}

func TestParseComposeUser(t *testing.T) {
	tests := []struct {
		user    string
		wantUID *int64
		wantGID *int64
	}{
		{"", nil, nil},
		{"1000", int64Ptr(1000), nil},
		{"1000:1001", int64Ptr(1000), int64Ptr(1001)},
		{"www-data", nil, nil},
	}
	for _, tt := range tests {
		uid, gid := parseComposeUser(tt.user)
		if !equalInt64Ptr(uid, tt.wantUID) || !equalInt64Ptr(gid, tt.wantGID) {
			t.Errorf("parseComposeUser(%q) = (%v, %v), want (%v, %v)", tt.user, uid, gid, tt.wantUID, tt.wantGID)
		}
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}

func equalInt64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
