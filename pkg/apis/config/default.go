package config

import "time"

const (
	// DefaultTimeoutMinutes is used when no timeout is specified.
	DefaultTimeoutMinutes = 60
	// NoTimeoutDuration is used when a pipeline or task should never time out.
	NoTimeoutDuration = 0 * time.Minute
	// DefaultServiceAccountValue is the SA used when one is not specified.
	DefaultServiceAccountValue = "default"
	// DefaultManagedByLabelValue is the value for the managed-by label that is used by default.
	DefaultManagedByLabelValue = "zchengjoey-pipelines"
	// DefaultCloudEventSinkValue is the default value for cloud event sinks.
	DefaultCloudEventSinkValue = ""
	// DefaultMaxMatrixCombinationsCount is used when no max matrix combinations count is specified.
	DefaultMaxMatrixCombinationsCount = 256

	defaultTimeoutMinutesKey      = "default-timeout-minutes"
	defaultServiceAccountKey      = "default-service-account"
	defaultManagedByLabelValueKey = "default-managed-by-label-value"
	defaultPodTemplateKey         = "default-pod-template"
	defaultAAPodTemplateKey       = "default-affinity-assistant-pod-template"
)
