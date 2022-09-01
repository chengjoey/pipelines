package v1alpha1

import (
	"context"

	"github.com/chengjoey/pipelines/pkg/apis/config"
	"knative.dev/pkg/apis"
)

const ManagedByLabelKey = "zchengjoey.dev/managed-by"

var _ apis.Defaultable = (*TaskRun)(nil)

// SetDefaults implements apis.Defaultable
func (tr *TaskRun) SetDefaults(ctx context.Context) {
	ctx = apis.WithinParent(ctx, tr.ObjectMeta)
	tr.Spec.SetDefaults(ctx)

	if tr.ObjectMeta.Labels == nil {
		tr.ObjectMeta.Labels = map[string]string{}
	}
	if _, found := tr.ObjectMeta.Labels[ManagedByLabelKey]; !found {
		tr.ObjectMeta.Labels[ManagedByLabelKey] = config.DefaultManagedByLabelValue
	}
}

// SetDefaults implements apis.Defaultable
func (trs *TaskRunSpec) SetDefaults(ctx context.Context) {
	trs.TaskSpec.SetDefaults(ctx)
}
