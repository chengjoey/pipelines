package pod

import (
	"fmt"
	"strings"
	"time"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const (
	// ReasonCouldntGetTask indicates that the reason for the failure status is that the
	// Task couldn't be found
	ReasonCouldntGetTask = "CouldntGetTask"

	// ReasonFailedResolution indicated that the reason for failure status is
	// that references within the TaskRun could not be resolved
	ReasonFailedResolution = "TaskRunResolutionFailed"

	// ReasonFailedValidation indicated that the reason for failure status is
	// that taskrun failed runtime validation
	ReasonFailedValidation = "TaskRunValidationFailed"

	// ReasonPullImageFailed indicates that the TaskRun's pod failed to pull image
	ReasonPullImageFailed = "PullImageFailed"

	// ReasonCreateContainerConfigError indicates that the TaskRun failed to create a pod due to
	// config error of container
	ReasonCreateContainerConfigError = "CreateContainerConfigError"

	// ReasonPodCreationFailed indicates that the reason for the current condition
	// is that the creation of the pod backing the TaskRun failed
	ReasonPodCreationFailed = "PodCreationFailed"

	// ReasonPending indicates that the pod is in corev1.Pending, and the reason is not
	// ReasonExceededNodeResources or isPodHitConfigError
	ReasonPending = "Pending"

	// timeFormat is RFC3339 with millisecond
	timeFormat = "2006-01-02T15:04:05.000Z07:00"
)

const (
	readyAnnotation      = "zchengjoey.dev/ready"
	readyAnnotationValue = "READY"
	stepPrefix           = "step-"
)

const oomKilled = "OOMKilled"

func isOOMKilled(s corev1.ContainerStatus) bool {
	return s.State.Terminated.Reason == oomKilled
}

// IsContainerStep returns true if the container name indicates that it
// represents a step.
func IsContainerStep(name string) bool { return strings.HasPrefix(name, stepPrefix) }

func DidTaskRunFail(pod *corev1.Pod) bool {
	f := pod.Status.Phase == corev1.PodFailed
	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated != nil {
				f = f || s.State.Terminated.ExitCode != 0 || isOOMKilled(s)
			}
		}
	}
	return f
}

func StepName(name string) string {
	if name != "" {
		return fmt.Sprintf("%s%s", stepPrefix, name)
	}
	return fmt.Sprintf("%sunamed-1", stepPrefix)
}

// trimStepPrefix returns the container name, stripped of its step prefix.
func trimStepPrefix(name string) string { return strings.TrimPrefix(name, stepPrefix) }

// markStatusRunning sets taskrun status to running
func markStatusRunning(trs *v1alpha1.TaskRunStatus, reason, message string) {
	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  reason,
		Message: message,
	})
}

// MakeTaskRunStatus returns a TaskRunStatus based on the Pod's status.
func MakeTaskRunStatus(logger *zap.SugaredLogger, tr v1alpha1.TaskRun, pod *corev1.Pod) (v1alpha1.TaskRunStatus, error) {
	trs := &tr.Status
	if trs.GetCondition(apis.ConditionSucceeded) == nil || trs.GetCondition(apis.ConditionSucceeded).Status == corev1.ConditionUnknown {
		// If the taskRunStatus doesn't exist yet, it's because we just started running
		markStatusRunning(trs, v1alpha1.TaskRunReasonRunning.String(), "Not all Steps in the Task have finished executing")
	}

	sortPodContainerStatuses(pod.Status.ContainerStatuses, pod.Spec.Containers)

	complete := areStepsComplete(pod) || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed

	if complete {
		updateCompletedTaskRunStatus(logger, trs, pod)
	} else {
		updateIncompleteTaskRunStatus(trs, pod)
	}

	trs.PodName = pod.Name
	trs.Steps = &v1alpha1.StepState{}

	var stepStatuses []corev1.ContainerStatus
	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			stepStatuses = append(stepStatuses, s)
			trs.Steps = &v1alpha1.StepState{
				ContainerState: *s.State.DeepCopy(),
				Name:           trimStepPrefix(s.Name),
				ImageID:        s.ImageID,
			}
		}
	}
	return *trs, nil
}

func updateCompletedTaskRunStatus(logger *zap.SugaredLogger, trs *v1alpha1.TaskRunStatus, pod *corev1.Pod) {
	if DidTaskRunFail(pod) {
		msg := getFailureMessage(logger, pod)
		markStatusFailure(trs, v1alpha1.TaskRunReasonFailed.String(), msg)
	} else {
		markStatusSuccess(trs)
	}

	// update tr completed time
	trs.CompletionTime = &metav1.Time{Time: time.Now()}
}

// markStatusFailure sets taskrun status to failure with specified reason
func markStatusFailure(trs *v1alpha1.TaskRunStatus, reason string, message string) {
	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
}

// markStatusSuccess sets taskrun status to success
func markStatusSuccess(trs *v1alpha1.TaskRunStatus) {
	trs.SetCondition(&apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  v1alpha1.TaskRunReasonSuccessful.String(),
		Message: "All Steps have completed executing",
	})
}

func updateIncompleteTaskRunStatus(trs *v1alpha1.TaskRunStatus, pod *corev1.Pod) {
	switch pod.Status.Phase {
	case corev1.PodRunning:
		markStatusRunning(trs, v1alpha1.TaskRunReasonRunning.String(), "Not all Steps in the Task have finished executing")
	case corev1.PodPending:
		switch {
		case isPullImageError(pod):
			markStatusRunning(trs, ReasonPullImageFailed, getWaitingMessage(pod))
		default:
			markStatusRunning(trs, ReasonPending, getWaitingMessage(pod))
		}
	}
}

func getWaitingMessage(pod *corev1.Pod) string {
	// First, try to surface reason for pending/unknown about the actual build step.
	for _, status := range pod.Status.ContainerStatuses {
		wait := status.State.Waiting
		if wait != nil && wait.Message != "" {
			return fmt.Sprintf("build step %q is pending with reason %q",
				status.Name, wait.Message)
		}
	}
	// Try to surface underlying reason by inspecting pod's recent status if condition is not true
	for i, podStatus := range pod.Status.Conditions {
		if podStatus.Status != corev1.ConditionTrue {
			return fmt.Sprintf("pod status %q:%q; message: %q",
				pod.Status.Conditions[i].Type,
				pod.Status.Conditions[i].Status,
				pod.Status.Conditions[i].Message)
		}
	}
	// Next, return the Pod's status message if it has one.
	if pod.Status.Message != "" {
		return pod.Status.Message
	}

	// Lastly fall back on a generic pending message.
	return "Pending"
}

// isPullImageError returns true if the Pod's status indicates there are any error when pulling image
func isPullImageError(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && isImageErrorReason(containerStatus.State.Waiting.Reason) {
			return true
		}
	}
	return false
}

func isImageErrorReason(reason string) bool {
	// Reference from https://github.com/kubernetes/kubernetes/blob/a1c8e9386af844757333733714fa1757489735b3/pkg/kubelet/images/types.go#L26
	imageErrorReasons := []string{
		"ImagePullBackOff",
		"ImageInspectError",
		"ErrImagePull",
		"ErrImageNeverPull",
		"RegistryUnavailable",
		"InvalidImageName",
	}
	for _, imageReason := range imageErrorReasons {
		if imageReason == reason {
			return true
		}
	}
	return false
}

func getFailureMessage(logger *zap.SugaredLogger, pod *corev1.Pod) string {
	// First, try to surface an error about the actual build step that failed.
	for _, status := range pod.Status.ContainerStatuses {
		term := status.State.Terminated
		if term != nil {
			if term.ExitCode != 0 {
				// Newline required at end to prevent yaml parser from breaking the log help text at 80 chars
				return fmt.Sprintf("%q exited with code %d (image: %q); for logs run: kubectl -n %s logs %s -c %s\n",
					status.Name, term.ExitCode, status.ImageID,
					pod.Namespace, pod.Name, status.Name)
			}
		}
	}
	// Next, return the Pod's status message if it has one.
	if pod.Status.Message != "" {
		return pod.Status.Message
	}

	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated != nil {
				if isOOMKilled(s) {
					return oomKilled
				}
			}
		}
	}

	// Lastly fall back on a generic error message.
	return "build failed for unspecified reasons."
}

func areStepsComplete(pod *corev1.Pod) bool {
	stepsComplete := len(pod.Status.ContainerStatuses) > 0 && pod.Status.Phase == corev1.PodRunning
	for _, s := range pod.Status.ContainerStatuses {
		if IsContainerStep(s.Name) {
			if s.State.Terminated == nil {
				stepsComplete = false
			}
		}
	}
	return stepsComplete
}

// sortPodContainerStatuses reorders a pod's container statuses so that
// they're in the same order as the step containers from the TaskSpec.
func sortPodContainerStatuses(podContainerStatuses []corev1.ContainerStatus, podSpecContainers []corev1.Container) {
	statuses := map[string]corev1.ContainerStatus{}
	for _, status := range podContainerStatuses {
		statuses[status.Name] = status
	}
	for i, c := range podSpecContainers {
		// prevent out-of-bounds panic on incorrectly formed lists
		if i < len(podContainerStatuses) {
			podContainerStatuses[i] = statuses[c.Name]
		}
	}
}
