package main

import (
	"github.com/chengjoey/pipelines/pkg/reconciler/pipelinerun"
	"github.com/chengjoey/pipelines/pkg/reconciler/taskrun"
	"knative.dev/pkg/injection/sharedmain"
)

func main() {
	sharedmain.Main("controller",
		taskrun.NewController,
		pipelinerun.NewController,
	)
}
