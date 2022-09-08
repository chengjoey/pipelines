package resource

import (
	"context"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/chengjoey/pipelines/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetPipelineFromPipelineRun(ctx context.Context, clientSet clientset.Interface, pipelineRun *v1alpha1.PipelineRun) (*v1alpha1.Pipeline, error) {
	pr := pipelineRun.Spec.PipelineRef
	namespace := pipelineRun.Namespace
	if pipelineRun.Spec.PipelineSpec != nil {
		return &v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pipelineRun.Name,
				Namespace: pipelineRun.Namespace,
			},
			Spec: *pipelineRun.Spec.PipelineSpec,
		}, nil
	}
	return clientSet.ZchengjoeyV1alpha1().Pipelines(namespace).Get(ctx, pr.Name, metav1.GetOptions{})
}
