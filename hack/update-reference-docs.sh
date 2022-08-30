#!/usr/bin/env bash

set -o errexit
set -o nounset

echo "Generating API reference docs ..."
go run github.com/ahmetb/gen-crd-api-reference-docs \
    -config "./hack/reference-docs-gen-config.json" \
    -api-dir "github.com/chengjoey/pipelines/pkg/apis" \
    -template-dir "./hack/reference-docs-template" \
    -out-file "./docs/pipeline-api.md"
sed -i".backup" '1s/^/<!--\n---\ntitle: Pipeline API\nlinkTitle: Pipeline API\nweight: 1000\n---\n-->\n\n/' ./docs/pipeline-api.md
