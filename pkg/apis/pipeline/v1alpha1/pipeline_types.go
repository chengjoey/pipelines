package v1alpha1

import (
	"github.com/chengjoey/pipelines/pkg/apis/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Pipeline describes a list of Tasks to execute
type Pipeline struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the Pipeline from the client
	// +optional
	Spec PipelineSpec `json:"spec"`
}

// PipelineSpec defines the desired state of Pipeline.
type PipelineSpec struct {
	// Description is a user-facing description of the pipeline that may be
	// used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`
	// Tasks declares the graph of Tasks that execute when this Pipeline is run.
	// +listType=atomic
	Tasks []PipelineTask `json:"tasks,omitempty"`
}

// PipelineTask defines a task in a Pipeline, passing inputs from both
// Params and from the output of previous tasks.
type PipelineTask struct {
	// Name is the name of this task within the context of a Pipeline. Name is
	// used as a coordinate with the `from` and `runAfter` fields to establish
	// the execution order of tasks relative to one another.
	Name string `json:"name,omitempty"`

	// TaskRef is a reference to a task definition.
	// +optional
	TaskRef *TaskRef `json:"taskRef,omitempty"`

	// TaskSpec is a specification of a task
	// +optional
	TaskSpec *EmbeddedTask `json:"taskSpec,omitempty"`
}

// EmbeddedTask is used to define a Task inline within a Pipeline's PipelineTasks.
type EmbeddedTask struct {
	// +optional
	runtime.TypeMeta `json:",inline,omitempty"`

	// Spec is a specification of a custom task
	// +optional
	Spec runtime.RawExtension `json:"spec,omitempty"`

	// +optional
	Metadata PipelineTaskMetadata `json:"metadata,omitempty"`

	// TaskSpec is a specification of a task
	// +optional
	TaskSpec `json:",inline,omitempty"`
}

// PipelineTaskMetadata contains the labels or annotations for an EmbeddedTask
type PipelineTaskMetadata struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

var _ kmeta.OwnerRefable = (*Pipeline)(nil)

// GetGroupVersionKind implements kmeta.OwnerRefable.
func (*Pipeline) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(pipeline.PipelineControllerName)
}
