package resource

import (
	"context"
	"fmt"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	"github.com/chengjoey/pipelines/pkg/dag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

type ResolvedPipelineTask struct {
	TaskRunName           string
	TaskRun               *v1alpha1.TaskRun
	TaskRunNames          []string
	PipelineTask          *v1alpha1.PipelineTask
	ResolvedTaskResources *ResolvedTaskResources
}

type ResolvedTaskResources struct {
	TaskName string
	TaskSpec *v1alpha1.TaskSpec
}

type PipelineRunState []*ResolvedPipelineTask

type PipelineRunFacts struct {
	State      PipelineRunState
	SpecStatus v1alpha1.PipelineRunSpecStatus
	TasksGraph *dag.Graph
}

// pipelineRunStatusCount holds the count of successful, failed, cancelled, skipped, and incomplete tasks
type pipelineRunStatusCount struct {
	// successful tasks count
	Succeeded int
	// failed tasks count
	Failed int
	// cancelled tasks count
	Cancelled int
	// number of tasks which are still pending, have not executed
	Incomplete int
}

func (facts *PipelineRunFacts) IsRunning() bool {
	for _, t := range facts.State {
		if facts.isDAGTask(t.PipelineTask.Name) {
			if t.IsRunning() {
				return true
			}
		}
	}
	return false
}

// IsCancelled returns true if the PipelineRun was cancelled
func (facts *PipelineRunFacts) IsCancelled() bool {
	return facts.SpecStatus == v1alpha1.PipelineRunSpecStatusCancelled
}

// IsGracefullyCancelled returns true if the PipelineRun was gracefully cancelled
func (facts *PipelineRunFacts) IsGracefullyCancelled() bool {
	return facts.SpecStatus == v1alpha1.PipelineRunSpecStatusCancelledRunFinally
}

// IsStopping returns true if the PipelineRun won't be scheduling any new Task because
// at least one task already failed or was cancelled in the specified dag
func (facts *PipelineRunFacts) IsStopping() bool {
	for _, t := range facts.State {
		if facts.isDAGTask(t.PipelineTask.Name) {
			if t.isFailure() {
				return true
			}
		}
	}
	return false
}

// IsGracefullyStopped returns true if the PipelineRun was gracefully stopped
func (facts *PipelineRunFacts) IsGracefullyStopped() bool {
	return facts.SpecStatus == v1alpha1.PipelineRunSpecStatusStoppedRunFinally
}

// DAGExecutionQueue returns a list of DAG tasks which needs to be scheduled next
func (facts *PipelineRunFacts) DAGExecutionQueue() (PipelineRunState, error) {
	var tasks PipelineRunState
	// when pipelinerun is cancelled or gracefully cancelled, do not schedule any new tasks,
	// and only wait for all running tasks to complete (without exhausting retries).
	if facts.IsCancelled() || facts.IsGracefullyCancelled() {
		return tasks, nil
	}
	// candidateTasks is initialized to DAG root nodes to start pipeline execution
	// candidateTasks is derived based on successfully finished tasks and/or skipped tasks
	candidateTasks, err := dag.FindSchedulableTasks(facts.TasksGraph, facts.completedOrSkippedDAGTasks()...)
	if err != nil {
		return tasks, err
	}
	if !facts.IsStopping() && !facts.IsGracefullyStopped() {
		tasks = facts.State.getNextTasks(candidateTasks)
	}
	return tasks, nil
}

// getNextTasks returns a list of tasks which should be executed next i.e.
// a list of tasks from candidateTasks which aren't yet indicated in state to be running and
// a list of cancelled/failed tasks from candidateTasks which haven't exhausted their retries
func (state PipelineRunState) getNextTasks(candidateTasks []string) []*ResolvedPipelineTask {
	tasks := []*ResolvedPipelineTask{}
	for _, t := range state {
		for _, taskName := range candidateTasks {
			if t.PipelineTask.Name == taskName {
				if t.TaskRun == nil {
					tasks = append(tasks, t)
				}
			}
		}
	}
	return tasks
}

func (state PipelineRunState) AdjustStartTime(unadjustedStartTime *metav1.Time) *metav1.Time {
	adjustedStartTime := unadjustedStartTime
	for _, rpt := range state {
		if rpt.TaskRun == nil {
		} else {
			if rpt.TaskRun.CreationTimestamp.Time.Before(adjustedStartTime.Time) {
				adjustedStartTime = &rpt.TaskRun.CreationTimestamp
			}
		}
	}
	return adjustedStartTime.DeepCopy()
}

func (facts *PipelineRunFacts) GetPipelineConditionStatus(ctx context.Context, pr *v1alpha1.PipelineRun) *apis.Condition {
	s := facts.getPipelineTasksCount()
	cmTasks := s.Succeeded + s.Failed + s.Cancelled
	reason := v1alpha1.PipelineRunReasonRunning.String()
	if s.Incomplete == 0 {
		status := corev1.ConditionTrue
		reason := v1alpha1.PipelineRunReasonSuccessful.String()
		message := fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d)",
			cmTasks, s.Failed, s.Cancelled)

		switch {
		case s.Failed > 0:
			reason = v1alpha1.PipelineRunReasonFailed.String()
			status = corev1.ConditionFalse
		case pr.IsGracefullyCancelled():
			reason = v1alpha1.PipelineRunReasonCancelled.String()
			status = corev1.ConditionFalse
			message = fmt.Sprintf("PipelineRun %q was cancelled", pr.Name)
		case s.Cancelled > 0:
			reason = v1alpha1.PipelineRunReasonCancelled.String()
			status = corev1.ConditionFalse
		}

		return &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  status,
			Reason:  reason,
			Message: message,
		}
	}

	switch {
	case pr.IsGracefullyCancelled():
		reason = v1alpha1.PipelineRunReasonCancelledRunningFinally.String()
	case s.Cancelled > 0 || s.Failed > 0:
		reason = v1alpha1.PipelineRunReasonStopping.String()
	}
	return &apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionUnknown,
		Reason: reason,
		Message: fmt.Sprintf("Tasks Completed: %d (Failed: %d, Cancelled %d), Incomplete: %d",
			cmTasks, s.Failed, s.Cancelled, s.Incomplete),
	}
}

func (facts *PipelineRunFacts) getPipelineTasksCount() pipelineRunStatusCount {
	s := pipelineRunStatusCount{
		Succeeded:  0,
		Failed:     0,
		Cancelled:  0,
		Incomplete: 0,
	}
	for _, t := range facts.State {
		switch {
		case t.isSuccessful():
			s.Succeeded++
		case t.isCancelled():
			s.Cancelled++
		case t.isFailure():
			s.Failed++
		default:
			s.Incomplete++
		}
	}
	return s
}

// completedOrSkippedTasks returns a list of the names of all of the PipelineTasks in state
// which have completed or skipped
func (facts *PipelineRunFacts) completedOrSkippedDAGTasks() []string {
	tasks := []string{}
	for _, t := range facts.State {
		if facts.isDAGTask(t.PipelineTask.Name) {
			if t.isDone() {
				tasks = append(tasks, t.PipelineTask.Name)
			}
		}
	}
	return tasks
}

// IsBeforeFirstTaskRun returns true if the PipelineRun has not yet started its first TaskRun
func (state PipelineRunState) IsBeforeFirstTaskRun() bool {
	for _, t := range state {
		if t.TaskRun != nil {
			return false
		}
	}
	return true
}

func (facts *PipelineRunFacts) isDAGTask(pipelineTaskName string) bool {
	if _, ok := facts.TasksGraph.Nodes[pipelineTaskName]; ok {
		return true
	}
	return false
}
