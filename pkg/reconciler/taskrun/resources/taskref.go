package resources

import (
	"context"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/chengjoey/pipelines/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTask is a function used to retrieve Tasks.
type GetTask func(context.Context, string) (*v1alpha1.Task, error)

// GetTaskFuncFromTaskRun is a factory function that will use the given TaskRef as context to return a valid GetTask function.
func GetTaskFuncFromTaskRun(ctx context.Context, clientSet clientset.Interface, tr *v1alpha1.TaskRun) (*v1alpha1.Task, error) {
	// if the spec is already in the status, do not try to fetch it again, just use it as source of truth
	if tr.Status.TaskSpec != nil {
		return &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tr.Name,
				Namespace: tr.Namespace,
			},
			Spec: *tr.Status.TaskSpec,
		}, nil
	}
	return clientSet.ZchengjoeyV1alpha1().Tasks(tr.Namespace).Get(ctx, tr.Spec.TaskRef.Name, metav1.GetOptions{})
}
