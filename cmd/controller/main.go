package main

import (
	// The set of controllers this controller process runs.
	"github.com/chengjoey/pipelines/pkg/reconciler/taskrun"

	// This defines the shared main for injected controllers.
	"knative.dev/pkg/injection/sharedmain"
)

func main() {
	sharedmain.Main("controller",
		taskrun.NewController,
	)
}
