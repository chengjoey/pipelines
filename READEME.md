## Introduction

A simple pipeline operator that supports dag

## Installation

```shell
git clone

kubectl apply -f config/
```

## Examples
`examples/v1alpha1/pipelinerun/pipelinerun-dag.yaml` is an example of a pipeline according to the dag process
```shell
kubectl apply -f examples/v1alpha1/pipelinerun/pipelinerun-dag.yaml
```

```shell
	 b     a
	 |    / \
	 |   |   x
	 |   | / |
	 |   y   |
	  \ /    z
	   w
```