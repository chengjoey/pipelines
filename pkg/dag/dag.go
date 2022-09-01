package dag

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

type Task interface {
	Key() string
	Deps() []string
}

type Tasks interface {
	Items() []Task
	Deps() map[string][]string
}

type Node struct {
	Task
	Prev []*Node
	Next []*Node
}

type Graph struct {
	nodes map[string]*Node
}

func Build(tasks Tasks) (*Graph, error) {
	g := &Graph{
		nodes: make(map[string]*Node),
	}
	for _, pt := range tasks.Items() {
		node := &Node{
			Task: pt,
		}
		if _, ok := g.nodes[pt.Key()]; ok {
			return nil, fmt.Errorf("duplicate task: %s", pt.Key())
		}
		g.nodes[pt.Key()] = node
	}
	for pt, deps := range tasks.Deps() {
		next, ok := g.nodes[pt]
		if !ok {
			return nil, fmt.Errorf("task: %s not existd in items", pt)
		}
		for _, dep := range deps {
			prev, ok := g.nodes[dep]
			if !ok {
				return nil, fmt.Errorf("dep task: %s not existed in items", dep)
			}
			if err := linkNode(next, prev); err != nil {
				return nil, err
			}
		}
	}
	return g, nil
}

func FindSchedulableTasks(g *Graph, doneTasks ...string) ([]string, error) {
	dones := sets.NewString(doneTasks...)
	roots := getRoots(g)
	schedulables := make([]string, 0)
	visited := sets.NewString()

	for _, root := range roots {
		schedulables = append(schedulables, find(root, visited, dones)...)
	}

	for _, done := range doneTasks {
		if !visited.Has(done) {
			return nil, fmt.Errorf("task: %s was indicated completed without ancestors being done", done)
		}
	}
	return schedulables, nil
}

func find(node *Node, visited sets.String, dones sets.String) []string {
	if visited.Has(node.Key()) {
		return nil
	}
	visited.Insert(node.Key())
	if dones.Has(node.Key()) {
		scheduables := make([]string, 0)
		for _, next := range node.Next {
			scheduables = append(scheduables, find(next, visited, dones)...)
		}
		return scheduables
	}
	if isSchedulable(node, dones) {
		return []string{node.Key()}
	}
	return []string{}
}

func isSchedulable(node *Node, dones sets.String) bool {
	collected := make([]string, 0)
	for _, prev := range node.Prev {
		if dones.Has(prev.Key()) {
			collected = append(collected, prev.Key())
		}
	}
	return len(collected) == len(node.Prev)
}

func getRoots(g *Graph) []*Node {
	roots := make([]*Node, 0)
	for _, node := range g.nodes {
		if len(node.Prev) == 0 {
			roots = append(roots, node)
		}
	}
	return roots
}

func linkNode(next, prev *Node) error {
	if next.Key() == prev.Key() {
		return fmt.Errorf("next task: %s can not depends on self", next.Key())
	}
	path := []string{next.Key(), prev.Key()}
	if err := lookNode(next, path, next.Prev); err != nil {
		return err
	}
	next.Prev = append(next.Prev, prev)
	prev.Next = append(prev.Next, next)
	return nil
}

func lookNode(n *Node, path []string, prevs []*Node) error {
	for _, prev := range prevs {
		path = append(path, prev.Key())
		if prev.Key() == n.Key() {
			return fmt.Errorf("unexpected circle tasks: %s", strings.Join(path, "->"))
		}
		if err := lookNode(n, path, prev.Prev); err != nil {
			return err
		}
	}
	return nil
}
