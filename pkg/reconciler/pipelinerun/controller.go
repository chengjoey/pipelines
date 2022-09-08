package pipelinerun

import (
	"context"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline"
	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	pipelineclient "github.com/chengjoey/pipelines/pkg/client/injection/client"
	pipelineruninformer "github.com/chengjoey/pipelines/pkg/client/injection/informers/pipeline/v1alpha1/pipelinerun"
	taskruninformer "github.com/chengjoey/pipelines/pkg/client/injection/informers/pipeline/v1alpha1/taskrun"
	pipelinerunreconciler "github.com/chengjoey/pipelines/pkg/client/injection/reconciler/pipeline/v1alpha1/pipelinerun"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

// NewController instantiates a new controller.Impl from knative.dev/pkg/controller
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	kubeClientSet := kubeclient.Get(ctx)
	taskRunInformer := taskruninformer.Get(ctx)
	pipelineClientSet := pipelineclient.Get(ctx)
	pipelineRunInformer := pipelineruninformer.Get(ctx)

	r := &Reconciler{
		pipelineClientSet: pipelineClientSet,
		kubeClientSet:     kubeClientSet,
		Clock:             clock.RealClock{},
		taskRunLister:     taskRunInformer.Lister(),
		pipelineRunLister: pipelineRunInformer.Lister(),
	}
	impl := pipelinerunreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			AgentName: pipeline.PipelineRunControllerName,
		}
	})

	pipelineRunInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	taskRunInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1alpha1.PipelineRun{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
