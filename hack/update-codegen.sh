#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname $0)/../vendor/knative.dev/hack/codegen-library.sh
export PATH="$GOBIN:$PATH"

function run_yq() {
  run_go_tool github.com/mikefarah/yq/v4@v4.23.1 yq "$@"
}

echo "=== Update Codegen for ${MODULE_NAME}"

group "Kubernetes Codegen"

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/chengjoey/pipelines/pkg/client github.com/chengjoey/pipelines/pkg/apis \
  "pipeline:v1alpha1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

group "Knative Codegen"

# Knative Injection
${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
  github.com/chengjoey/pipelines/pkg/client github.com/chengjoey/pipelines/pkg/apis \
  "pipeline:v1alpha1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

group "Update CRD Schema"

go run $(dirname $0)/../cmd/schema/main.go dump TaskRun \
  | run_yq eval-all --header-preprocess=false --inplace 'select(fileIndex == 0).spec.versions[0].schema.openAPIV3Schema = select(fileIndex == 1) | select(fileIndex == 0)' \
  $(dirname $0)/../config/300-taskrun.yaml -

go run $(dirname $0)/../cmd/schema/main.go dump PipelineRun \
  | run_yq eval-all --header-preprocess=false --inplace 'select(fileIndex == 0).spec.versions[0].schema.openAPIV3Schema = select(fileIndex == 1) | select(fileIndex == 0)' \
  $(dirname $0)/../config/300-pipelinerun.yaml -

go run $(dirname $0)/../cmd/schema/main.go dump Task \
  | run_yq eval-all --header-preprocess=false --inplace 'select(fileIndex == 0).spec.versions[0].schema.openAPIV3Schema = select(fileIndex == 1) | select(fileIndex == 0)' \
  $(dirname $0)/../config/300-task.yaml -

go run $(dirname $0)/../cmd/schema/main.go dump Pipeline \
  | run_yq eval-all --header-preprocess=false --inplace 'select(fileIndex == 0).spec.versions[0].schema.openAPIV3Schema = select(fileIndex == 1) | select(fileIndex == 0)' \
  $(dirname $0)/../config/300-pipeline.yaml -

group "Update deps post-codegen"

# Make sure our dependencies are up-to-date
#${REPO_ROOT_DIR}/hack/update-deps.sh
