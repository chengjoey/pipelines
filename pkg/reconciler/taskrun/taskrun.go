package taskrun

import (
	"context"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	taskrunreconciler "github.com/chengjoey/pipelines/pkg/client/injection/reconciler/pipeline/v1alpha1/taskrun"
	listers "github.com/chengjoey/pipelines/pkg/client/listers/pipeline/v1alpha1"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/reconciler"
)

// Check that our Reconciler implements Interface
var _ taskrunreconciler.Interface = (*Reconciler)(nil)

// Reconciler implements simpledeploymentreconciler.Interface for
// SimpleDeployment resources.
type Reconciler struct {
	KubeClientSet kubernetes.Interface
	Clock         clock.PassiveClock

	// listers index properties about resources
	podLister     corev1listers.PodLister
	taskRunLister listers.TaskRunLister
}

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, d *v1alpha1.TaskRun) reconciler.Event {
	// impl it

	return nil
}
