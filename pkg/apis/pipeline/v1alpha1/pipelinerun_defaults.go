package v1alpha1

import "context"

func (pr *PipelineRun) SetDefaults(ctx context.Context) {
	pr.Spec.SetDefaults(ctx)
}

func (prs *PipelineRunSpec) SetDefaults(ctx context.Context) {
	if prs.PipelineSpec != nil {
		prs.PipelineSpec.SetDefaults(ctx)
	}
}

func (ps *PipelineSpec) SetDefaults(ctx context.Context) {
	for _, pt := range ps.Tasks {
		if pt.TaskSpec != nil {
			pt.TaskSpec.SetDefaults(ctx)
		}
	}
}
