package resource

import (
	"context"
	"fmt"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
)

func (t ResolvedPipelineTask) isDone() bool {
	return t.isSuccessful() || t.isFailure()
}

func (t ResolvedPipelineTask) IsRunning() bool {
	if t.TaskRun == nil {
		return false
	}
	return !t.isSuccessful() && !t.isFailure()
}

func (t ResolvedPipelineTask) isFailure() bool {
	if t.isCancelled() {
		return true
	}
	if t.isSuccessful() {
		return false
	}

	if t.TaskRun == nil {
		return false
	}
	c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	return t.TaskRun.IsDone() && c.IsFalse()
}

func (t ResolvedPipelineTask) isCancelled() bool {
	if t.TaskRun == nil {
		return false
	}
	c := t.TaskRun.Status.GetCondition(apis.ConditionSucceeded)
	return c != nil && c.IsFalse() && c.Reason == v1alpha1.TaskRunReasonCancelled.String()
}

func (t ResolvedPipelineTask) isSuccessful() bool {
	return t.TaskRun.IsSuccessful()
}

// GetTask is a function used to retrieve Tasks.
type GetTask func(context.Context, string) (*v1alpha1.Task, error)

// GetTaskRun is a function used to retrieve TaskRuns
type GetTaskRun func(string) (*v1alpha1.TaskRun, error)

func ResolvePipelineTask(ctx context.Context,
	pipelineRun v1alpha1.PipelineRun,
	getTask GetTask,
	getTaskRun GetTaskRun,
	pipelineTask v1alpha1.PipelineTask,
) (*ResolvedPipelineTask, error) {
	rpt := ResolvedPipelineTask{
		PipelineTask: &pipelineTask,
	}
	rpt.TaskRunName = GetTaskRunName(pipelineRun.Status.TaskRuns, pipelineRun.Status.ChildReferences, pipelineTask.Name, pipelineRun.Name)
	if err := rpt.resolvePipelineRunTaskWithTaskRun(ctx, rpt.TaskRunName, getTask, getTaskRun, pipelineTask); err != nil {
		return nil, err
	}

	return &rpt, nil
}

func (rpt *ResolvedPipelineTask) resolvePipelineRunTaskWithTaskRun(
	ctx context.Context,
	taskRunName string,
	getTask GetTask,
	getTaskRun GetTaskRun,
	pipelineTask v1alpha1.PipelineTask,
) error {
	taskRun, err := getTaskRun(taskRunName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to get TaskRun: %s, err: %v", taskRunName, err)
		}
	}
	if taskRun != nil {
		rpt.TaskRun = taskRun
	}
	if err := rpt.resolveTaskResources(ctx, getTask, pipelineTask, taskRun); err != nil {
		return err
	}

	return nil
}

func (rpt *ResolvedPipelineTask) resolveTaskResources(
	ctx context.Context,
	getTask GetTask,
	pipelineTask v1alpha1.PipelineTask,
	taskRun *v1alpha1.TaskRun,
) error {
	spec, taskName, err := resolveTask(ctx, taskRun, getTask, pipelineTask)
	if err != nil {
		return err
	}
	spec.SetDefaults(ctx)
	rtr, err := resolvePipelineTaskResources(pipelineTask, &spec, taskName)
	if err != nil {
		return err
	}
	rpt.ResolvedTaskResources = rtr
	return nil
}

func resolveTask(
	ctx context.Context,
	taskRun *v1alpha1.TaskRun,
	getTask GetTask,
	pipelineTask v1alpha1.PipelineTask,
) (v1alpha1.TaskSpec, string, error) {
	var (
		t        *v1alpha1.Task
		err      error
		spec     v1alpha1.TaskSpec
		taskName string
	)

	if pipelineTask.TaskRef != nil {
		// If the TaskRun has already a stored TaskSpec in its status, use it as source of truth
		if taskRun != nil && taskRun.Status.TaskSpec != nil {
			spec = *taskRun.Status.TaskSpec
			taskName = pipelineTask.TaskRef.Name
		} else {
			t, err = getTask(ctx, pipelineTask.TaskRef.Name)
			switch {
			case err != nil:
				return v1alpha1.TaskSpec{}, "", fmt.Errorf("task: %s is not found, err: %v", pipelineTask.TaskRef.Name, err)
			default:
				spec = t.Spec
				taskName = t.ObjectMeta.Name
			}
		}
	} else {
		spec = pipelineTask.TaskSpec.TaskSpec
	}
	return spec, taskName, err
}

func resolvePipelineTaskResources(pt v1alpha1.PipelineTask, ts *v1alpha1.TaskSpec, taskName string) (*ResolvedTaskResources, error) {
	rtr := ResolvedTaskResources{
		TaskName: taskName,
		TaskSpec: ts,
	}
	return &rtr, nil
}

func GetTaskRunName(taskRunsStatus map[string]*v1alpha1.PipelineRunTaskRunStatus, childRefs []v1alpha1.ChildStatusReference, ptName, prName string) string {
	for _, cr := range childRefs {
		if cr.PipelineTaskName == ptName {
			return cr.Name
		}
	}
	for k, v := range taskRunsStatus {
		if v.PipelineTaskName == ptName {
			return k
		}
	}

	return kmeta.ChildName(prName, fmt.Sprintf("-%s", ptName))
}
