package main

import (
	"log"
	"net/http"

	"github.com/chengjoey/pipelines/pkg/reconciler/pipelinerun"
	"github.com/chengjoey/pipelines/pkg/reconciler/taskrun"
	"knative.dev/pkg/injection/sharedmain"
)

func main() {
	// sets up liveness and readiness probes.
	mux := http.NewServeMux()

	mux.HandleFunc("/", handler)
	mux.HandleFunc("/health", handler)
	mux.HandleFunc("/readiness", handler)

	go func() {
		log.Fatal(http.ListenAndServe(":8080", mux))
	}()
	sharedmain.Main("controller",
		taskrun.NewController,
		pipelinerun.NewController,
	)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
