package main

import (
	"log"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	"knative.dev/hack/schema/commands"
	"knative.dev/hack/schema/registry"
)

// schema is a tool to dump the schema for Eventing resources.
func main() {
	registry.Register(&v1alpha1.TaskRun{})
	registry.Register(&v1alpha1.PipelineRun{})
	registry.Register(&v1alpha1.Task{})
	registry.Register(&v1alpha1.Pipeline{})

	if err := commands.New("github.com/chengjoey/pipelines").Execute(); err != nil {
		log.Fatal("Error during command execution: ", err)
	}
}
