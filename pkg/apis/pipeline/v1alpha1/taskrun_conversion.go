package v1alpha1

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*TaskRun)(nil)

func (tr *TaskRun) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *TaskRun:
		sink.ObjectMeta = tr.ObjectMeta
		return tr.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

func (tr *TaskRun) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch source := from.(type) {
	case *TaskRun:
		tr.ObjectMeta = source.ObjectMeta
		return tr.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", tr)
	}
}

func (trs *TaskRunSpec) ConvertTo(ctx context.Context, sink *TaskRunSpec) error {
	if trs.TaskRef != nil {
		sink.TaskRef = &TaskRef{}
		trs.TaskRef.convertTo(ctx, sink.TaskRef)
	}
	if trs.TaskSpec != nil {
		sink.TaskSpec = &TaskSpec{}
		trs.TaskSpec.ConvertTo(ctx, sink.TaskSpec)
	}
	sink.Status = trs.Status
	return nil
}

func (trs *TaskRunSpec) ConvertFrom(ctx context.Context, source *TaskRunSpec) error {
	if source.TaskSpec != nil {
		newSpec := TaskSpec{}
		newSpec.ConvertFrom(ctx, source.TaskSpec)
		trs.TaskSpec = &newSpec
	}
	if source.TaskRef != nil {
		newRef := TaskRef{}
		newRef.convertFrom(ctx, *source.TaskRef)
		trs.TaskRef = &newRef
	}
	trs.Status = source.Status
	return nil
}

func (tr TaskRef) convertTo(ctx context.Context, sink *TaskRef) {
	sink.Name = tr.Name
}

func (tr *TaskRef) convertFrom(ctx context.Context, source TaskRef) {
	tr.Name = source.Name
}

func (ts *TaskSpec) ConvertTo(ctx context.Context, sink *TaskSpec) error {
	sink.Name = ts.Name
	sink.Args = nil
	for _, arg := range ts.Args {
		sink.Args = append(sink.Args, arg)
	}
	sink.Command = nil
	for _, cmd := range ts.Command {
		sink.Command = append(sink.Command, cmd)
	}
	sink.Image = ts.Image
	sink.Description = ts.Description
	return nil
}

func (ts *TaskSpec) ConvertFrom(ctx context.Context, source *TaskSpec) error {
	ts.Name = source.Name
	ts.Image = source.Image
	ts.Description = source.Description
	ts.Args = nil
	for _, arg := range source.Args {
		ts.Args = append(ts.Args, arg)
	}
	ts.Command = nil
	for _, cmd := range source.Command {
		ts.Command = append(ts.Command, cmd)
	}
	return nil
}
