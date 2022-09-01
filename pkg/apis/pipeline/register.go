package pipeline

const (
	GroupName = "zchengjoey.dev"
)

const (
	// TaskLabelKey is used as the label identifier for a Task
	TaskLabelKey = GroupName + "/task"

	// TaskRunLabelKey is used as the label identifier for a TaskRun
	TaskRunLabelKey = GroupName + "/taskRun"

	// PipelineLabelKey is used as the label identifier for a Pipeline
	PipelineLabelKey = GroupName + "/pipeline"

	// PipelineRunLabelKey is used as the label identifier for a PipelineRun
	PipelineRunLabelKey = GroupName + "/pipelineRun"

	// PipelineTaskLabelKey is used as the label identifier for a PipelineTask
	PipelineTaskLabelKey = GroupName + "/pipelineTask"
)
