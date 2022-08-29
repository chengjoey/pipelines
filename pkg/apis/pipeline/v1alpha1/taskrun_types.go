package v1alpha1

import (
	"github.com/chengjoey/pipelines/pkg/apis/pipeline"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskRun represents a single execution of a Task. TaskRuns are how the steps
// specified in a Task are executed; they specify the parameters and resources
// used to run the steps in a Task.
//
// +k8s:openapi-gen=true
type TaskRun struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec TaskRunSpec `json:"spec,omitempty"`
	// +optional
	Status TaskRunStatus `json:"status,omitempty"`
}

// TaskRunSpec defines the desired state of TaskRun
type TaskRunSpec struct {
	TaskRef *TaskRef `json:"taskRef,omitempty"`
	// +optional
	TaskSpec *TaskSpec `json:"taskSpec,omitempty"`
	// Used for cancelling a taskrun (and maybe more later on)
	// +optional
	Status TaskRunSpecStatus `json:"status,omitempty"`
}

// TaskRunSpecStatus defines the taskrun spec status the user can provide
type TaskRunSpecStatus string

// TaskRef can be used to refer to a specific instance of a task.
type TaskRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name,omitempty"`
}

// TaskRunStatus defines the observed state of TaskRun
type TaskRunStatus struct {
	duckv1.Status `json:",inline"`

	// TaskRunStatusFields inlines the status fields.
	TaskRunStatusFields `json:",inline"`
}

// TaskRunStatusFields holds the fields of TaskRun's status.  This is defined
// separately and inlined so that other types can readily consume these fields
// via duck typing.
type TaskRunStatusFields struct {
	// PodName is the name of the pod responsible for executing this task's steps.
	PodName string `json:"podName"`

	// StartTime is the time the build is actually started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the build completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Steps describes the state of each build step container.
	// +optional
	Steps *StepState `json:"steps,omitempty"`

	// TaskSpec contains the Spec from the dereferenced Task definition used to instantiate this TaskRun.
	TaskSpec *TaskSpec `json:"taskSpec,omitempty"`
}

// StepState reports the results of running a step in a Task.
type StepState struct {
	corev1.ContainerState `json:",inline"`
	Name                  string `json:"name,omitempty"`
	Container             string `json:"container,omitempty"`
	ImageID               string `json:"imageID,omitempty"`
}

var _ kmeta.OwnerRefable = (*TaskRun)(nil)

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*TaskRun) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.TaskRunControllerName)
}
