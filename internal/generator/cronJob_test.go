package generator

import (
	"testing"

	"katenary.io/internal/generator/labels"

	corev1 "k8s.io/api/core/v1"
)

func TestNewCronJob(t *testing.T) {
	t.Parallel()
	service := newService("cron").image("fedora").label(labels.LabelCronJob, `
image: alpine
command: echo hello
schedule: "*/1 * * * *"
rbac: false`).build()

	chart := NewChart("testapp")
	NewDeployment(service, chart) // populate the values like Generate does

	cronJob, rbac := NewCronJob(service, chart)
	if cronJob == nil {
		t.Fatal("expected a cronjob")
	}
	if rbac != nil {
		t.Errorf("expected no RBAC, got %+v", rbac)
	}

	container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	if len(container.Command) != 3 || container.Command[0] != "sh" || container.Command[1] != "-c" || container.Command[2] != "echo hello" {
		t.Errorf("expected command [sh -c echo hello], got %v", container.Command)
	}
	if len(container.Args) != 0 {
		t.Errorf("expected no args, got %v", container.Args)
	}
	if cronJob.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyOnFailure {
		t.Errorf("expected restartPolicy OnFailure, got %s", cronJob.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy)
	}

	value := chart.Values["cron"].(*Value)
	if value.CronJob.Schedule != "*/1 * * * *" {
		t.Errorf("expected schedule */1 * * * *, got %s", value.CronJob.Schedule)
	}
	if value.CronJob.Repository.Image != "alpine" {
		t.Errorf("expected cronjob image alpine, got %s", value.CronJob.Repository.Image)
	}
	if value.Repository.Image != "fedora" {
		t.Errorf("expected deployment image fedora, got %s", value.Repository.Image)
	}
}

func TestNewCronJobWithRBAC(t *testing.T) {
	t.Parallel()
	service := newService("cron").image("fedora").label(labels.LabelCronJob, `
image: alpine
command: echo hello
schedule: "*/1 * * * *"
rbac: true`).build()

	chart := NewChart("testapp")
	cronJob, rbac := NewCronJob(service, chart)
	if cronJob == nil {
		t.Fatal("expected a cronjob")
	}
	if rbac == nil || rbac.ServiceAccount == nil {
		t.Fatal("expected a service account")
	}
	if cronJob.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName == "" {
		t.Error("expected the cronjob to reference a service account")
	}
	if cronJob.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName != rbac.ServiceAccount.Name {
		t.Errorf("service account mismatch: cronjob uses %s, RBAC defines %s",
			cronJob.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName,
			rbac.ServiceAccount.Name,
		)
	}
}
