package taskrun

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline"
	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	"github.com/chengjoey/pipelines/pkg/client/clientset/versioned"
	taskrunreconciler "github.com/chengjoey/pipelines/pkg/client/injection/reconciler/pipeline/v1alpha1/taskrun"
	listers "github.com/chengjoey/pipelines/pkg/client/listers/pipeline/v1alpha1"
	podconvert "github.com/chengjoey/pipelines/pkg/pod"
	"github.com/chengjoey/pipelines/pkg/reconciler/taskrun/resources"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

// Check that our Reconciler implements Interface
var _ taskrunreconciler.Interface = (*Reconciler)(nil)

var (
	groupVersionKind = schema.GroupVersionKind{
		Group:   v1alpha1.SchemeGroupVersion.Group,
		Version: v1alpha1.SchemeGroupVersion.Version,
		Kind:    "TaskRun",
	}
)

// Reconciler implements simpledeploymentreconciler.Interface for
// SimpleDeployment resources.
type Reconciler struct {
	KubeClientSet     kubernetes.Interface
	PipelineClientSet versioned.Interface
	Clock             clock.PassiveClock

	// listers index properties about resources
	podLister     corev1listers.PodLister
	taskRunLister listers.TaskRunLister
}

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, tr *v1alpha1.TaskRun) reconciler.Event {
	logger := logging.FromContext(ctx)

	if !tr.HasStarted() {
		tr.Status.InitializeConditions()
		if tr.Status.StartTime.Sub(tr.CreationTimestamp.Time) < 0 {
			logger.Warnf("TaskRun %s createTimestamp %s is after the taskRun started %s", tr.GetNamespacedName().String(), tr.CreationTimestamp, tr.Status.StartTime)
			tr.Status.StartTime = &tr.CreationTimestamp
		}
	}

	if tr.IsDone() {
		logger.Infof("taskrun done: %s \n", tr.Name)
		tr.SetDefaults(ctx)

		return nil
	}

	if tr.IsCancelled() {
		message := fmt.Sprintf("TaskRun %q was cancelled.", tr.Name)
		err := r.failTaskRun(ctx, tr, v1alpha1.TaskRunSpecStatusCancelled, message)
		if err != nil {
			return err
		}
		return nil
	}

	if failed, reason, message := r.checkPodFailed(tr); failed {
		err := r.failTaskRun(ctx, tr, reason, message)
		return err
	}

	ts, err := r.prepare(ctx, tr)
	if err != nil {
		logger.Errorf("TaskRun prepare err: %v", err)
		return err
	}

	if err = r.reconcile(ctx, tr, ts); err != nil {
		logger.Errorf("Reconcile err: %v", err)
	}

	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, tr *v1alpha1.TaskRun, ts *v1alpha1.TaskSpec) error {
	logger := logging.FromContext(ctx)

	var pod *corev1.Pod
	var err error

	if tr.Status.PodName != "" {
		pod, err = r.podLister.Pods(tr.Namespace).Get(tr.Status.PodName)
		if k8serrors.IsNotFound(err) {

		} else if err != nil {
			logger.Errorf("eror gettig pod: %s, err: %v", tr.Status.PodName, err)
			return err
		}
	} else {
		labelSelecotr := labels.Set{pipeline.TaskRunLabelKey: tr.Name}
		pos, err := r.podLister.Pods(tr.Namespace).List(labelSelecotr.AsSelector())
		if err != nil {
			logger.Errorf("Error listing pods: %v", err)
			return err
		}
		for idx := range pos {
			po := pos[idx]
			if metav1.IsControlledBy(po, tr) && !podconvert.DidTaskRunFail(po) {
				pod = po
			}
		}
	}

	if pod == nil {
		pod, err = r.createPod(ctx, tr, ts)
		if err != nil {
			newErr := r.handlePodCreationError(tr, err)
			logger.Errorf("Failed to create task run pod for taskRun %q: %v", tr.Name, newErr)
			return newErr
		}
	}
	tr.Status, err = podconvert.MakeTaskRunStatus(logger, *tr, pod)
	if err != nil {
		return err
	}

	logger.Infof("Successfully reconciled taskrun %s/%s with status: %v", tr.Name, tr.Namespace, tr.Status.GetCondition(apis.ConditionSucceeded))
	return nil
}

func (r *Reconciler) createPod(ctx context.Context, tr *v1alpha1.TaskRun, ts *v1alpha1.TaskSpec) (*corev1.Pod, error) {
	var err error
	stepContainer := corev1.Container{
		Name:    podconvert.StepName(ts.Name),
		Image:   ts.Image,
		Command: ts.Command,
		Args:    ts.Args,
	}
	podAnnotations := kmeta.CopyMap(tr.Annotations)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tr.Namespace,
			Name:      kmeta.ChildName(tr.Name, "-pod"),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(tr, groupVersionKind),
			},
			Annotations: podAnnotations,
			Labels:      makeLabels(tr),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{stepContainer},
		},
	}

	podName := pod.Name
	pod, err = r.KubeClientSet.CoreV1().Pods(tr.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil && k8serrors.IsAlreadyExists(err) {
		if p, getErr := r.podLister.Pods(tr.Namespace).Get(podName); getErr == nil {
			return p, nil
		}
	}
	return pod, err
}

// makeLabels constructs the labels we will propagate from TaskRuns to Pods.
func makeLabels(s *v1alpha1.TaskRun) map[string]string {
	labels := make(map[string]string, len(s.ObjectMeta.Labels)+1)
	// NB: Set this *before* passing through TaskRun labels. If the TaskRun
	// has a managed-by label, it should override this default.

	// Copy through the TaskRun's labels to the underlying Pod's.
	for k, v := range s.ObjectMeta.Labels {
		labels[k] = v
	}

	// NB: Set this *after* passing through TaskRun Labels. If the TaskRun
	// specifies this label, it should be overridden by this value.
	labels[pipeline.TaskRunLabelKey] = s.Name
	return labels
}

func (r *Reconciler) prepare(ctx context.Context, tr *v1alpha1.TaskRun) (*v1alpha1.TaskSpec, error) {
	logger := logging.FromContext(ctx)
	tr.SetDefaults(ctx)

	task, err := resources.GetTaskFuncFromTaskRun(ctx, r.PipelineClientSet, tr)
	if err != nil {
		logger.Errorf("Failed to fetch task reference: %s: %v", tr.Spec.TaskRef.Name, err)
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedResolution, err)
		return nil, err
	}

	if tr.Status.TaskSpec == nil {
		tr.Status.TaskSpec = &task.Spec
		if tr.ObjectMeta.Annotations == nil {
			tr.ObjectMeta.Annotations = make(map[string]string, len(task.Annotations))
		}
		for key, value := range task.Annotations {
			if _, ok := tr.ObjectMeta.Annotations[key]; !ok {
				tr.ObjectMeta.Annotations[key] = value
			}
		}
		if tr.ObjectMeta.Labels == nil {
			tr.ObjectMeta.Labels = make(map[string]string, len(task.Labels)+1)
		}
		for key, value := range task.Labels {
			if _, ok := tr.ObjectMeta.Labels[key]; !ok {
				tr.ObjectMeta.Labels[key] = value
			}
		}
		if tr.Spec.TaskRef != nil {
			tr.ObjectMeta.Labels[pipeline.TaskLabelKey] = task.Name
		}
		return &task.Spec, nil
	}
	return tr.Spec.TaskSpec, nil
}

func (r *Reconciler) checkPodFailed(tr *v1alpha1.TaskRun) (bool, v1alpha1.TaskRunReason, string) {
	step := tr.Status.Steps
	if step == nil {
		return false, "", ""
	}

	if step.Waiting != nil && step.Waiting.Reason == "ImagePullBackOff" {
		image := step.ImageID
		message := fmt.Sprintf(`The step %q in TaskRun %q failed to pull the image %q. The pod errored with the message: "%s."`, step.Name, tr.Name, image, step.Waiting.Message)
		return true, v1alpha1.TaskRunReasonImagePullFailed, message
	}

	return false, "", ""
}

func (r *Reconciler) failTaskRun(ctx context.Context, tr *v1alpha1.TaskRun, reason v1alpha1.TaskRunReason, message string) error {
	logger := logging.FromContext(ctx)

	logger.Warnf("stopping task run: %s because of %s", tr.Name, reason)
	tr.Status.MarkResourceFailed(reason, errors.New(message))

	completionTime := metav1.Time{Time: r.Clock.Now()}
	tr.Status.CompletionTime = &completionTime
	if tr.Status.PodName == "" {
		logger.Warnf("task run %s has no pod running yet", tr.Name)
		return nil
	}

	err := r.KubeClientSet.CoreV1().Pods(tr.Namespace).Delete(ctx, tr.Status.PodName, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.Infof("Failed to terminate pod: %v", err)
		return err
	}

	step := tr.Status.Steps
	if step == nil {
		return nil
	}
	if step.Running != nil {
		step.Terminated = &corev1.ContainerStateTerminated{
			ExitCode:   1,
			StartedAt:  step.Running.StartedAt,
			FinishedAt: completionTime,
			Reason:     reason.String(),
		}
		step.Running = nil
		tr.Status.Steps = step
	}

	if step.Waiting != nil {
		step.Terminated = &corev1.ContainerStateTerminated{
			ExitCode:   1,
			FinishedAt: completionTime,
			Reason:     reason.String(),
		}
		step.Waiting = nil
		tr.Status.Steps = step
	}

	return nil
}

func isExceededResourceQuotaError(err error) bool {
	return err != nil && k8serrors.IsForbidden(err) && strings.Contains(err.Error(), "exceeded quota")
}

func isTaskRunValidationFailed(err error) bool {
	return err != nil && strings.Contains(err.Error(), "TaskRun validation failed")
}

func (c *Reconciler) handlePodCreationError(tr *v1alpha1.TaskRun, err error) error {
	switch {
	case isTaskRunValidationFailed(err):
		tr.Status.MarkResourceFailed(podconvert.ReasonFailedValidation, err)
	case k8serrors.IsAlreadyExists(err):
		tr.Status.MarkResourceOngoing(podconvert.ReasonPending, fmt.Sprint("tried to create pod, but it already exists"))
	default:
		// The pod creation failed with unknown reason. The most likely
		// reason is that something is wrong with the spec of the Task, that we could
		// not check with validation before - i.e. pod template fields
		msg := fmt.Sprintf("failed to create task run pod %q: %v. Maybe ", tr.Name, err)
		if tr.Spec.TaskRef != nil {
			msg += fmt.Sprintf("missing or invalid Task %s/%s", tr.Namespace, tr.Spec.TaskRef.Name)
		} else {
			msg += "invalid TaskSpec"
		}
		err = controller.NewPermanentError(errors.New(msg))
		tr.Status.MarkResourceFailed(podconvert.ReasonCouldntGetTask, err)
	}
	return err
}
