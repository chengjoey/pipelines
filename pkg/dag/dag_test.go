package dag_test

import (
	"testing"

	"github.com/chengjoey/pipelines/pkg/apis/pipeline/v1alpha1"
	"github.com/chengjoey/pipelines/pkg/dag"
)

func TestFindSchedulableTasks1(t *testing.T) {
	g := testGraph1(t)
	tcs := []struct {
		name          string
		finished      []string
		expectedTasks []string
	}{{
		name:          "nothing-done",
		finished:      []string{},
		expectedTasks: []string{"a", "b"},
	}, {
		name:          "a-done",
		finished:      []string{"a"},
		expectedTasks: []string{"b", "x"},
	}, {
		name:          "b-done",
		finished:      []string{"b"},
		expectedTasks: []string{"a"},
	}, {
		name:          "a-and-b-done",
		finished:      []string{"a", "b"},
		expectedTasks: []string{"x"},
	}, {
		name:          "a-x-done",
		finished:      []string{"a", "x"},
		expectedTasks: []string{"b", "y", "z"},
	}, {
		name:          "a-x-b-done",
		finished:      []string{"a", "x", "b"},
		expectedTasks: []string{"y", "z"},
	}, {
		name:          "a-x-y-done",
		finished:      []string{"a", "x", "y"},
		expectedTasks: []string{"b", "z"},
	}, {
		name:          "a-x-y-done",
		finished:      []string{"a", "x", "y"},
		expectedTasks: []string{"b", "z"},
	}, {
		name:          "a-x-y-b-done",
		finished:      []string{"a", "x", "y", "b"},
		expectedTasks: []string{"w", "z"},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tasks, err := dag.FindSchedulableTasks(g, tc.finished...)
			if err != nil {
				t.Fatalf("Didn't expect error when getting next tasks for %v but got %v", tc.finished, err)
			}
			if len(tasks) != len(tc.expectedTasks) {
				t.Fatalf("expected tasks: %v not equal got tasks: %v", tc.expectedTasks, tasks)
			}
			t.Logf("expectd tasks: %v, got tasks: %v", tc.expectedTasks, tasks)
		})
	}
}

func TestFindSchedulableTasks2(t *testing.T) {
	g := testGraph2(t)
	tcs := []struct {
		name          string
		finished      []string
		expectedTasks []string
	}{{
		name:          "nothing-done",
		finished:      []string{},
		expectedTasks: []string{"a"},
	}, {
		name:          "a-done",
		finished:      []string{"a"},
		expectedTasks: []string{"d", "e"},
	}, {
		name:          "d-done",
		finished:      []string{"a", "d"},
		expectedTasks: []string{"e"},
	}, {
		name:          "d-and-e-done",
		finished:      []string{"a", "d", "e"},
		expectedTasks: []string{"f"},
	}, {
		name:          "f-done",
		finished:      []string{"a", "d", "e", "f"},
		expectedTasks: []string{"g"},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tasks, err := dag.FindSchedulableTasks(g, tc.finished...)
			if err != nil {
				t.Fatalf("Didn't expect error when getting next tasks for %v but got %v", tc.finished, err)
			}
			if len(tasks) != len(tc.expectedTasks) {
				t.Fatalf("expected tasks: %v not equal got tasks: %v", tc.expectedTasks, tasks)
			}
			t.Logf("expectd tasks: %v, got tasks: %v", tc.expectedTasks, tasks)
		})
	}
}

func testGraph1(t *testing.T) *dag.Graph {
	//  b     a
	//  |    / \
	//  |   |   x
	//  |   | / |
	//  |   y   |
	//   \ /    z
	//    w
	tasks := []v1alpha1.PipelineTask{{
		Name: "a",
	}, {
		Name: "b",
	}, {
		Name:     "w",
		RunAfter: []string{"b", "y"},
	}, {
		Name:     "x",
		RunAfter: []string{"a"},
	}, {
		Name:     "y",
		RunAfter: []string{"a", "x"},
	}, {
		Name:     "z",
		RunAfter: []string{"x"},
	}}
	g, err := dag.Build(v1alpha1.PipelineTaskList(tasks))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func testGraph2(t *testing.T) *dag.Graph {
	//   a
	//  / \
	// d   e
	//  \ /
	//   f
	//   |
	//   g
	tasks := []v1alpha1.PipelineTask{{
		Name: "a",
	}, {
		Name:     "d",
		RunAfter: []string{"a"},
	}, {
		Name:     "e",
		RunAfter: []string{"a"},
	}, {
		Name:     "f",
		RunAfter: []string{"d", "e"},
	}, {
		Name:     "g",
		RunAfter: []string{"f"},
	}}
	g, err := dag.Build(v1alpha1.PipelineTaskList(tasks))
	if err != nil {
		t.Fatal(err)
	}
	return g
}
