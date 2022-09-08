package pipelinerun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline"
	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	"github.com/chengjoey/pipelines/pkg/client/clientset/versioned"
	pipelinerunreconciler "github.com/chengjoey/pipelines/pkg/client/injection/reconciler/pipeline/v1alpha1/pipelinerun"
	listers "github.com/chengjoey/pipelines/pkg/client/listers/pipeline/v1alpha1"
	"github.com/chengjoey/pipelines/pkg/dag"
	"github.com/chengjoey/pipelines/pkg/reconciler/pipelinerun/resource"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

const (
	// ReasonCouldntGetPipeline indicates that the reason for the failure status is that the
	// associated Pipeline couldn't be retrieved
	ReasonCouldntGetPipeline = "CouldntGetPipeline"
	// ReasonCouldntGetTask indicates that the reason for the failure status is that the
	// associated Pipeline's Tasks couldn't all be retrieved
	ReasonCouldntGetTask = "CouldntGetTask"
	// ReasonFailedValidation indicates that the reason for failure status is
	// that pipelinerun failed runtime validation
	ReasonFailedValidation = "PipelineValidationFailed"
	// ReasonInvalidGraph indicates that the reason for the failure status is that the
	// associated Pipeline is an invalid graph (a.k.a wrong order, cycle, …)
	ReasonInvalidGraph = "PipelineInvalidGraph"
	// ReasonCancelled indicates that a PipelineRun was cancelled.
	ReasonCancelled = "Cancelled"
	// ReasonPending indicates that a PipelineRun is pending.
	ReasonPending = "PipelineRunPending"
	// ReasonCouldntCancel indicates that a PipelineRun was cancelled but attempting to update
	// all of the running TaskRuns as cancelled failed.
	ReasonCouldntCancel = "PipelineRunCouldntCancel"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	Clock             clock.PassiveClock
	kubeClientSet     kubernetes.Interface
	pipelineClientSet versioned.Interface

	// listers index properties about pipeline resource
	pipelineRunLister listers.PipelineRunLister
	taskRunLister     listers.TaskRunLister
}

var (
	_ pipelinerunreconciler.Interface = (*Reconciler)(nil)
)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Pipeline Run
// resource with the current status of the resource.
func (r *Reconciler) ReconcileKind(ctx context.Context, pr *v1alpha1.PipelineRun) reconciler.Event {
	logger := logging.FromContext(ctx)

	if !pr.HasStarted() && !pr.IsPending() {
		pr.Status.InitializeConditions(r.Clock)
		if pr.Status.StartTime.Sub(pr.CreationTimestamp.Time) < 0 {
			logger.Warnf("PipelineRun %s createTimestamp %s is after the pipelineRun started %s", pr.GetNamespacedName().String(), pr.CreationTimestamp, pr.Status.StartTime)
			pr.Status.StartTime = &pr.CreationTimestamp
		}
	}

	if pr.IsDone() {
		pr.SetDefaults(ctx)
		if err := r.updateTaskRunsStatusDirectly(pr); err != nil {
			return err
		}
		return nil
	}

	if err := propagatePipelineNameLabelToPipelineRun(pr); err != nil {
		return err
	}

	if pr.IsCancelled() {
		err := cancelPipelineRun(ctx, pr, r.pipelineClientSet)
		if err != nil {
			return err
		}
		return nil
	}

	err := r.updatePipelineRunStatusFromInformer(ctx, pr)
	if err != nil {
		logger.Errorf("Error while syncing the pipelinerun status: %v", err.Error())
		return err
	}

	pipelineSpec, err := resource.GetPipelineFromPipelineRun(ctx, r.pipelineClientSet, pr)
	if err != nil {
		return err
	}
	if err := r.reconcile(ctx, pr, pipelineSpec); err != nil {
		logger.Errorf("Reconcile err: %v", err)
		return err
	}
	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, pr *v1alpha1.PipelineRun, p *v1alpha1.Pipeline) error {
	logger := logging.FromContext(ctx)
	pr.SetDefaults(ctx)

	if pr.IsPending() {
		pr.Status.MarkRunning(ReasonPending, fmt.Sprintf("PipelineRun: %s is pending", pr.Name))
		return nil
	}

	graph, err := dag.Build(v1alpha1.PipelineTaskList(p.Spec.Tasks))
	if err != nil {
		pr.Status.MarkFailed(ReasonInvalidGraph, "PipelineRun %s/%s's Pipeline DAG is invalid: %s", pr.Namespace, pr.Name, err)
		return controller.NewPermanentError(err)
	}

	pr.Status.PipelineSpec = &p.Spec
	tasks := p.Spec.Tasks
	pipelineRunState, err := r.resolvePipelineState(ctx, tasks, &pr.ObjectMeta, pr)
	if err != nil {
		return err
	}
	pipelineRunFacts := &resource.PipelineRunFacts{
		State:      pipelineRunState,
		SpecStatus: pr.Spec.Status,
		TasksGraph: graph,
	}

	if pr.IsGracefullyCancelled() && pipelineRunFacts.IsRunning() {
		// TODO add cancel
	}
	if err := r.runNextScheduleableTasks(ctx, pr, pipelineRunFacts); err != nil {
		return err
	}

	after := pipelineRunFacts.GetPipelineConditionStatus(ctx, pr)
	switch after.Status {
	case corev1.ConditionTrue:
		pr.Status.MarkSucceeded(after.Reason, after.Message)
	case corev1.ConditionFalse:
		pr.Status.MarkFailed(after.Reason, after.Message)
	case corev1.ConditionUnknown:
		pr.Status.MarkRunning(after.Reason, after.Message)
	}
	after = pr.Status.GetCondition(apis.ConditionSucceeded)
	pr.Status.StartTime = pipelineRunFacts.State.AdjustStartTime(pr.Status.StartTime)

	logger.Infof("PipelineRun %s status is being set to %s", pr.Name, after)
	return nil
}

func (r *Reconciler) runNextScheduleableTasks(ctx context.Context, pr *v1alpha1.PipelineRun, pipelineRunFacts *resource.PipelineRunFacts) error {
	logger := logging.FromContext(ctx)

	nextRpts, err := pipelineRunFacts.DAGExecutionQueue()
	if err != nil {
		logger.Errorf("Error getting potential next tasks for valid pipelinerun %s: %v", pr.Name, err)
		return controller.NewPermanentError(err)
	}
	for _, rpt := range nextRpts {
		if rpt == nil {
			continue
		}
		rpt.TaskRun, err = r.createTaskRun(ctx, rpt.TaskRunName, rpt, pr)
		if err != nil {
			return fmt.Errorf("error creating TaskRun called %s for PipelineTask %s from PipelineRun %s: %w", rpt.TaskRunName, rpt.PipelineTask.Name, pr.Name, err)
		}
	}
	return nil
}

func (r *Reconciler) createTaskRun(ctx context.Context, taskRunName string, rpt *resource.ResolvedPipelineTask, pr *v1alpha1.PipelineRun) (*v1alpha1.TaskRun, error) {
	logger := logging.FromContext(ctx)

	tr, _ := r.taskRunLister.TaskRuns(pr.Namespace).Get(taskRunName)
	if tr != nil {
		if !tr.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
			return tr, nil
		}
	}
	tr = &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:            taskRunName,
			Namespace:       pr.Namespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(pr)},
			Labels:          combineTaskRunAndTaskSpecLabels(pr, rpt.PipelineTask),
			Annotations:     combineTaskRunAndTaskSpecAnnotations(pr, rpt.PipelineTask),
		},
		Spec: v1alpha1.TaskRunSpec{},
	}
	if rpt.ResolvedTaskResources.TaskName != "" {
		tr.Spec.TaskRef = rpt.PipelineTask.TaskRef
	} else if rpt.ResolvedTaskResources.TaskSpec != nil {
		tr.Spec.TaskSpec = rpt.ResolvedTaskResources.TaskSpec
	}
	logger.Infof("Creating a new TaskRun object %s for pipeline task %s", taskRunName, rpt.PipelineTask.Name)
	return r.pipelineClientSet.ZchengjoeyV1alpha1().TaskRuns(pr.Namespace).Create(ctx, tr, metav1.CreateOptions{})
}

func combineTaskRunAndTaskSpecLabels(pr *v1alpha1.PipelineRun, pipelineTask *v1alpha1.PipelineTask) map[string]string {
	labels := make(map[string]string)
	addMetadataByPrecedence(labels, getTaskrunLabels(pr, pipelineTask.Name, true))

	if pipelineTask.TaskSpec != nil {
		addMetadataByPrecedence(labels, map[string]string{})
	}

	return labels
}

func combineTaskRunAndTaskSpecAnnotations(pr *v1alpha1.PipelineRun, pipelineTask *v1alpha1.PipelineTask) map[string]string {
	annotations := make(map[string]string)

	addMetadataByPrecedence(annotations, getTaskrunAnnotations(pr))

	if pipelineTask.TaskSpec != nil {
		addMetadataByPrecedence(annotations, map[string]string{})
	}

	return annotations
}

func getTaskrunAnnotations(pr *v1alpha1.PipelineRun) map[string]string {
	// Propagate annotations from PipelineRun to TaskRun.
	annotations := make(map[string]string, len(pr.ObjectMeta.Annotations)+1)
	for key, val := range pr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	return annotations
}

// addMetadataByPrecedence() adds the elements in addedMetadata to metadata. If the same key is present in both maps, the value from metadata will be used.
func addMetadataByPrecedence(metadata map[string]string, addedMetadata map[string]string) {
	for key, value := range addedMetadata {
		// add new annotations if the key not exists in current ones
		if _, ok := metadata[key]; !ok {
			metadata[key] = value
		}
	}
}

func clearStatus(tr *v1alpha1.TaskRun) {
	tr.Status.StartTime = nil
	tr.Status.CompletionTime = nil
	tr.Status.PodName = ""
}

func (r *Reconciler) resolvePipelineState(
	ctx context.Context,
	tasks []v1alpha1.PipelineTask,
	pipelineMeta *metav1.ObjectMeta,
	pr *v1alpha1.PipelineRun,
) (resource.PipelineRunState, error) {
	pst := resource.PipelineRunState{}
	for _, task := range tasks {
		resolvedTask, err := resource.ResolvePipelineTask(
			ctx,
			*pr,
			func(ctx context.Context, name string) (*v1alpha1.Task, error) {
				return r.pipelineClientSet.ZchengjoeyV1alpha1().Tasks(pr.Namespace).Get(ctx, name, metav1.GetOptions{})
			},
			func(name string) (*v1alpha1.TaskRun, error) {
				return r.taskRunLister.TaskRuns(pr.Namespace).Get(name)
			},
			task,
		)
		if err != nil {
			pr.Status.MarkFailed(ReasonCouldntGetTask,
				"PipelineRun %s/%s can't be Run; couldn't resolve all references: %s",
				pipelineMeta.Namespace, pr.Name, err)
			return nil, controller.NewPermanentError(err)
		}
		pst = append(pst, resolvedTask)
	}
	return pst, nil
}

func (c *Reconciler) updatePipelineRunStatusFromInformer(ctx context.Context, pr *v1alpha1.PipelineRun) error {
	logger := logging.FromContext(ctx)

	// Get the pipelineRun label that is set on each TaskRun.  Do not include the propagated labels from the
	// Pipeline and PipelineRun.  The user could change them during the lifetime of the PipelineRun so the
	// current labels may not be set on the previously created TaskRuns.
	pipelineRunLabels := getTaskrunLabels(pr, "", false)
	taskRuns, err := c.taskRunLister.TaskRuns(pr.Namespace).List(k8slabels.SelectorFromSet(pipelineRunLabels))
	if err != nil {
		logger.Errorf("could not list TaskRuns %#v", err)
		return err
	}

	updatePipelineRunStatusFromTaskRuns(logger, pr, taskRuns)
	return nil
}

// updatePipelineRunStatusFromTaskRuns takes a PipelineRun and a list of TaskRuns within that PipelineRun, and updates
// the PipelineRun's .Status.TaskRuns.
func updatePipelineRunStatusFromTaskRuns(logger *zap.SugaredLogger, pr *v1alpha1.PipelineRun, trs []*v1alpha1.TaskRun) {
	// If no TaskRun was found, nothing to be done. We never remove taskruns from the status
	if len(trs) == 0 {
		return
	}

	if pr.Status.TaskRuns == nil {
		pr.Status.TaskRuns = make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
	}

	taskRuns := filterTaskRunsForPipelineRun(logger, pr, trs)

	// Loop over all the TaskRuns associated to Tasks
	for _, taskrun := range taskRuns {
		lbls := taskrun.GetLabels()
		pipelineTaskName := lbls[pipeline.PipelineTaskLabelKey]

		if _, ok := pr.Status.TaskRuns[taskrun.Name]; !ok {
			// This taskrun was missing from the status.
			// Add it without conditions, which are handled in the next loop
			logger.Infof("Found a TaskRun %s that was missing from the PipelineRun status", taskrun.Name)
			pr.Status.TaskRuns[taskrun.Name] = &v1alpha1.PipelineRunTaskRunStatus{
				PipelineTaskName: pipelineTaskName,
				Status:           &taskrun.Status,
			}
		}
	}
}

// filterTaskRunsForPipelineRun returns TaskRuns owned by the PipelineRun.
func filterTaskRunsForPipelineRun(logger *zap.SugaredLogger, pr *v1alpha1.PipelineRun, trs []*v1alpha1.TaskRun) []*v1alpha1.TaskRun {
	var ownedTaskRuns []*v1alpha1.TaskRun

	for _, tr := range trs {
		// Only process TaskRuns that are owned by this PipelineRun.
		// This skips TaskRuns that are indirectly created by the PipelineRun (e.g. by custom tasks).
		if len(tr.OwnerReferences) < 1 || tr.OwnerReferences[0].UID != pr.ObjectMeta.UID {
			logger.Debugf("Found a TaskRun %s that is not owned by this PipelineRun", tr.Name)
			continue
		}
		ownedTaskRuns = append(ownedTaskRuns, tr)
	}

	return ownedTaskRuns
}

func getTaskrunLabels(pr *v1alpha1.PipelineRun, pipelineTaskName string, includePipelineLabels bool) map[string]string {
	// Propagate labels from PipelineRun to TaskRun.
	labels := make(map[string]string, len(pr.ObjectMeta.Labels)+1)
	if includePipelineLabels {
		for key, val := range pr.ObjectMeta.Labels {
			labels[key] = val
		}
	}
	labels[pipeline.PipelineRunLabelKey] = pr.Name
	if pipelineTaskName != "" {
		labels[pipeline.PipelineTaskLabelKey] = pipelineTaskName
	}
	if pr.Status.PipelineSpec != nil {
		// check if a task is part of the "tasks" section, add a label to identify it during the runtime
		for _, f := range pr.Status.PipelineSpec.Tasks {
			if pipelineTaskName == f.Name {
				labels[pipeline.MemberOfLabelKey] = v1alpha1.PipelineTasks
				break
			}
		}
	}
	return labels
}

func (r *Reconciler) updateTaskRunsStatusDirectly(pr *v1alpha1.PipelineRun) error {
	for taskRunName := range pr.Status.TaskRuns {
		prtrs := pr.Status.TaskRuns[taskRunName]
		tr, err := r.taskRunLister.TaskRuns(pr.Namespace).Get(taskRunName)
		if err != nil {
			// If the TaskRun isn't found, it just means it won't be run
			if !k8serrors.IsNotFound(err) {
				return fmt.Errorf("error retrieving TaskRun %s: %w", taskRunName, err)
			}
		} else {
			prtrs.Status = &tr.Status
		}
	}
	return nil
}

func propagatePipelineNameLabelToPipelineRun(pr *v1alpha1.PipelineRun) error {
	if pr.ObjectMeta.Labels == nil {
		pr.ObjectMeta.Labels = make(map[string]string)
	}
	switch {
	case pr.Spec.PipelineRef != nil && pr.Spec.PipelineRef.Name != "":
		pr.ObjectMeta.Labels[pipeline.PipelineLabelKey] = pr.Spec.PipelineRef.Name
	case pr.Spec.PipelineSpec != nil:
		pr.ObjectMeta.Labels[pipeline.PipelineLabelKey] = pr.Name
	default:
		return fmt.Errorf("pipelineRun %s not providing PipelineRef or PipelineSpec", pr.Name)
	}
	return nil
}

// cancelPipelineRun marks the PipelineRun as cancelled and any resolved TaskRun(s) too.
func cancelPipelineRun(ctx context.Context, pr *v1alpha1.PipelineRun, clientSet versioned.Interface) error {
	errs := cancelPipelineTaskRunsForTaskNames(ctx, pr, clientSet, sets.NewString())

	// If we successfully cancelled all the TaskRuns and Runs, we can consider the PipelineRun cancelled.
	if len(errs) == 0 {
		reason := ReasonCancelled

		pr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  reason,
			Message: fmt.Sprintf("PipelineRun %q was cancelled", pr.Name),
		})
		// update pr completed time
		pr.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	} else {
		e := strings.Join(errs, "\n")
		// Indicate that we failed to cancel the PipelineRun
		pr.Status.SetCondition(&apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  ReasonCouldntCancel,
			Message: fmt.Sprintf("PipelineRun %q was cancelled but had errors trying to cancel TaskRuns and/or Runs: %s", pr.Name, e),
		})
		return fmt.Errorf("error(s) from cancelling TaskRun(s) from PipelineRun %s: %s", pr.Name, e)
	}
	return nil
}

func cancelPipelineTaskRunsForTaskNames(ctx context.Context, pr *v1alpha1.PipelineRun, clientSet versioned.Interface, taskNames sets.String) []string {
	errs := make([]string, 0)
	trNames := getChildObjectsFromPRStatusForTaskNames(ctx, pr.Status, taskNames)
	for _, taskRunName := range trNames {
		if err := cancelTaskRun(ctx, taskRunName, pr.Namespace, clientSet); err != nil {
			errs = append(errs, fmt.Errorf("Failed to patch TaskRun `%s` with cancellation: %s", taskRunName, err).Error())
		}
	}
	return errs
}

var cancelTaskRunPatchBytes, cancelRunPatchBytes []byte

func cancelTaskRun(ctx context.Context, taskRunName string, namespace string, clientSet versioned.Interface) error {
	_, err := clientSet.ZchengjoeyV1alpha1().TaskRuns(namespace).Patch(ctx, taskRunName, types.JSONPatchType, cancelTaskRunPatchBytes, metav1.PatchOptions{}, "")
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func getChildObjectsFromPRStatusForTaskNames(ctx context.Context, prs v1alpha1.PipelineRunStatus, taskNames sets.String) []string {
	var trNames []string

	for trName, trs := range prs.TaskRuns {
		if taskNames.Len() == 0 || taskNames.Has(trs.PipelineTaskName) {
			trNames = append(trNames, trName)
		}
	}

	return trNames
}
