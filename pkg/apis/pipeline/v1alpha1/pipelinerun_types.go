package v1alpha1

import (
	"github.com/chengjoey/pipelines/pkg/apis/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRun represents a single execution of a Pipeline. PipelineRuns are how
// the graph of Tasks declared in a Pipeline are executed; they specify inputs
// to Pipelines such as parameter values and capture operational aspects of the
// Tasks execution such as service account and tolerations. Creating a
// PipelineRun creates TaskRuns for Tasks in the referenced Pipeline.
//
// +k8s:openapi-gen=true
type PipelineRun struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec PipelineRunSpec `json:"spec,omitempty"`
	// +optional
	Status PipelineRunStatus `json:"status,omitempty"`
}

// PipelineRef can be used to refer to a specific instance of a Pipeline.
type PipelineRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name,omitempty"`
}

// PipelineRunSpec defines the desired state of PipelineRun
type PipelineRunSpec struct {
	// +optional
	PipelineRef *PipelineRef `json:"pipelineRef,omitempty"`
	// +optional
	PipelineSpec *PipelineSpec `json:"pipelineSpec,omitempty"`

	// Used for cancelling a pipelinerun (and maybe more later on)
	// +optional
	Status PipelineRunSpecStatus `json:"status,omitempty"`
}

// PipelineRunSpecStatus defines the pipelinerun spec status the user can provide
type PipelineRunSpecStatus string

const (
	// PipelineRunSpecStatusCancelled indicates that the user wants to cancel the task,
	// if not already cancelled or terminated
	PipelineRunSpecStatusCancelled = "Cancelled"

	// PipelineRunSpecStatusCancelledRunFinally indicates that the user wants to cancel the pipeline run,
	// if not already cancelled or terminated, but ensure finally is run normally
	PipelineRunSpecStatusCancelledRunFinally = "CancelledRunFinally"

	// PipelineRunSpecStatusStoppedRunFinally indicates that the user wants to stop the pipeline run,
	// wait for already running tasks to be completed and run finally
	// if not already cancelled or terminated
	PipelineRunSpecStatusStoppedRunFinally = "StoppedRunFinally"

	// PipelineRunSpecStatusPending indicates that the user wants to postpone starting a PipelineRun
	// until some condition is met
	PipelineRunSpecStatusPending = "PipelineRunPending"
)

// PipelineRunStatusFields holds the fields of PipelineRunStatus' status.
// This is defined separately and inlined so that other types can readily
// consume these fields via duck typing.
type PipelineRunStatusFields struct {
	// StartTime is the time the PipelineRun is actually started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the PipelineRun completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// PipelineRunSpec contains the exact spec used to instantiate the run
	PipelineSpec *PipelineSpec `json:"pipelineSpec,omitempty"`

	// Deprecated - use ChildReferences instead.
	// map of PipelineRunTaskRunStatus with the taskRun name as the key
	// +optional
	TaskRuns map[string]*PipelineRunTaskRunStatus `json:"taskRuns,omitempty"`

	// list of TaskRun and Run names, PipelineTask names, and API versions/kinds for children of this PipelineRun.
	// +optional
	// +listType=atomic
	ChildReferences []ChildStatusReference `json:"childReferences,omitempty"`

	// FinallyStartTime is when all non-finally tasks have been completed and only finally tasks are being executed.
	// +optional
	FinallyStartTime *metav1.Time `json:"finallyStartTime,omitempty"`
}

// PipelineRunReason represents a reason for the pipeline run "Succeeded" condition
type PipelineRunReason string

const (
	// PipelineRunReasonStarted is the reason set when the PipelineRun has just started
	PipelineRunReasonStarted PipelineRunReason = "Started"
	// PipelineRunReasonRunning is the reason set when the PipelineRun is running
	PipelineRunReasonRunning PipelineRunReason = "Running"
	// PipelineRunReasonSuccessful is the reason set when the PipelineRun completed successfully
	PipelineRunReasonSuccessful PipelineRunReason = "Succeeded"
	// PipelineRunReasonCompleted is the reason set when the PipelineRun completed successfully with one or more skipped Tasks
	PipelineRunReasonCompleted PipelineRunReason = "Completed"
	// PipelineRunReasonFailed is the reason set when the PipelineRun completed with a failure
	PipelineRunReasonFailed PipelineRunReason = "Failed"
	// PipelineRunReasonCancelled is the reason set when the PipelineRun cancelled by the user
	// This reason may be found with a corev1.ConditionFalse status, if the cancellation was processed successfully
	// This reason may be found with a corev1.ConditionUnknown status, if the cancellation is being processed or failed
	PipelineRunReasonCancelled PipelineRunReason = "Cancelled"
	// PipelineRunReasonPending is the reason set when the PipelineRun is in the pending state
	PipelineRunReasonPending PipelineRunReason = "PipelineRunPending"
	// PipelineRunReasonTimedOut is the reason set when the PipelineRun has timed out
	PipelineRunReasonTimedOut PipelineRunReason = "PipelineRunTimeout"
	// PipelineRunReasonStopping indicates that no new Tasks will be scheduled by the controller, and the
	// pipeline will stop once all running tasks complete their work
	PipelineRunReasonStopping PipelineRunReason = "PipelineRunStopping"
	// PipelineRunReasonCancelledRunningFinally indicates that pipeline has been gracefully cancelled
	// and no new Tasks will be scheduled by the controller, but final tasks are now running
	PipelineRunReasonCancelledRunningFinally PipelineRunReason = "CancelledRunningFinally"
	// PipelineRunReasonStoppedRunningFinally indicates that pipeline has been gracefully stopped
	// and no new Tasks will be scheduled by the controller, but final tasks are now running
	PipelineRunReasonStoppedRunningFinally PipelineRunReason = "StoppedRunningFinally"
)

func (pr PipelineRunReason) String() string {
	return string(pr)
}

// PipelineRunTaskRunStatus contains the name of the PipelineTask for this TaskRun and the TaskRun's Status
type PipelineRunTaskRunStatus struct {
	// PipelineTaskName is the name of the PipelineTask.
	PipelineTaskName string `json:"pipelineTaskName,omitempty"`
	// Status is the TaskRunStatus for the corresponding TaskRun
	// +optional
	Status *TaskRunStatus `json:"status,omitempty"`
}

// ChildStatusReference is used to point to the statuses of individual TaskRuns and Runs within this PipelineRun.
type ChildStatusReference struct {
	runtime.TypeMeta `json:",inline"`
	// Name is the name of the TaskRun or Run this is referencing.
	Name string `json:"name,omitempty"`
	// PipelineTaskName is the name of the PipelineTask this is referencing.
	PipelineTaskName string `json:"pipelineTaskName,omitempty"`
}

// PipelineRunStatus defines the observed state of PipelineRun
type PipelineRunStatus struct {
	duckv1beta1.Status `json:",inline"`

	// PipelineRunStatusFields inlines the status fields.
	PipelineRunStatusFields `json:",inline"`
}

// MarkRunning changes the Succeeded condition to Unknown with the provided reason and message.
func (pr *PipelineRunStatus) MarkRunning(reason, messageFormat string, messageA ...interface{}) {
	pipelineRunCondSet.Manage(pr).MarkUnknown(apis.ConditionSucceeded, reason, messageFormat, messageA...)
}

// MarkFailed changes the Succeeded condition to False with the provided reason and message.
func (pr *PipelineRunStatus) MarkFailed(reason, messageFormat string, messageA ...interface{}) {
	pipelineRunCondSet.Manage(pr).MarkFalse(apis.ConditionSucceeded, reason, messageFormat, messageA...)
	succeeded := pr.GetCondition(apis.ConditionSucceeded)
	pr.CompletionTime = &succeeded.LastTransitionTime.Inner
}

// MarkSucceeded changes the Succeeded condition to True with the provided reason and message.
func (pr *PipelineRunStatus) MarkSucceeded(reason, messageFormat string, messageA ...interface{}) {
	pipelineRunCondSet.Manage(pr).MarkTrueWithReason(apis.ConditionSucceeded, reason, messageFormat, messageA...)
	succeeded := pr.GetCondition(apis.ConditionSucceeded)
	pr.CompletionTime = &succeeded.LastTransitionTime.Inner
}

var pipelineRunCondSet = apis.NewBatchConditionSet()

// GetCondition returns the Condition matching the given type.
func (prs *PipelineRunStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pipelineRunCondSet.Manage(prs).GetCondition(t)
}

// SetCondition sets the condition, unsetting previous conditions with the same
// type as necessary.
func (pr *PipelineRunStatus) SetCondition(newCond *apis.Condition) {
	if newCond != nil {
		pipelineRunCondSet.Manage(pr).SetCondition(*newCond)
	}
}

func (prs *PipelineRunStatus) InitializeConditions(c clock.PassiveClock) {
	started := false
	if prs.TaskRuns == nil {
		prs.TaskRuns = make(map[string]*PipelineRunTaskRunStatus)
	}
	if prs.StartTime.IsZero() {
		prs.StartTime = &metav1.Time{Time: c.Now()}
		started = true
	}
	conditionManager := pipelineRunCondSet.Manage(prs)
	conditionManager.InitializeConditions()
	if started {
		initialCondition := conditionManager.GetCondition(apis.ConditionSucceeded)
		initialCondition.Reason = PipelineRunReasonStarted.String()
		conditionManager.SetCondition(*initialCondition)
	}
}

func (pr *PipelineRun) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: pr.Name, Namespace: pr.Namespace}
}

func (pr *PipelineRun) IsPending() bool {
	return pr.Spec.Status == PipelineRunSpecStatusPending
}

func (pr *PipelineRun) IsDone() bool {
	return !pr.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
}

func (pr *PipelineRun) HasStarted() bool {
	return pr.Status.StartTime != nil && !pr.Status.StartTime.IsZero()
}

func (pr *PipelineRun) IsCancelled() bool {
	return pr.Spec.Status == PipelineRunSpecStatusCancelled
}

func (pr *PipelineRun) IsGracefullyCancelled() bool {
	return pr.Spec.Status == PipelineRunSpecStatusCancelledRunFinally
}

func (pr *PipelineRun) isGracefullyStopped() bool {
	return pr.Spec.Status == PipelineRunSpecStatusStoppedRunFinally
}

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*PipelineRun) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.PipelineRunControllerName)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineRunList contains a list of PipelineRun
type PipelineRunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineRun `json:"items,omitempty"`
}
