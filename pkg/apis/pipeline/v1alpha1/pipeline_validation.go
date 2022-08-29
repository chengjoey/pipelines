package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

// Validate implements apis.Validatable
func (p *Pipeline) Validate(ctx context.Context) *apis.FieldError {
	return p.Spec.Validate(ctx).ViaField("spec")
}

// Validate implements apis.Validatable
func (ps *PipelineSpec) Validate(ctx context.Context) *apis.FieldError {
	if len(ps.Tasks) == 0 {
		return apis.ErrMissingField("tasks")
	}
	return nil
}
