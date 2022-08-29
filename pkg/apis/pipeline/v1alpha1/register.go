package v1alpha1

import (
	"github.com/chengjoey/pipelines/pkg/apis/pipeline"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: pipeline.GroupName, Version: "v1alpha1"}
