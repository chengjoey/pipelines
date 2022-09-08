package taskrun

import (
	"context"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline"
	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	pipelineclient "github.com/chengjoey/pipelines/pkg/client/injection/client"
	taskruninformer "github.com/chengjoey/pipelines/pkg/client/injection/informers/pipeline/v1alpha1/taskrun"
	taskrunreconciler "github.com/chengjoey/pipelines/pkg/client/injection/reconciler/pipeline/v1alpha1/taskrun"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	podinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

// NewController creates a Reconciler
// instantiates a new controller.Impl from knative.dev/pkg/controller
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	kubeClientSet := kubeclient.Get(ctx)
	taskRunInformer := taskruninformer.Get(ctx)
	podInformer := podinformer.Get(ctx)
	pipelineClientSet := pipelineclient.Get(ctx)

	c := &Reconciler{
		PipelineClientSet: pipelineClientSet,
		KubeClientSet:     kubeClientSet,
		Clock:             clock.RealClock{},
		taskRunLister:     taskRunInformer.Lister(),
		podLister:         podInformer.Lister(),
	}
	impl := taskrunreconciler.NewImpl(ctx, c, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			AgentName: pipeline.TaskRunControllerName,
		}
	})

	taskRunInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1alpha1.TaskRun{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
