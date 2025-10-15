package main

import (
	"context"
	"sync"
)

// DepName is a simple alias for dependency name keys.
type DepName string

// NamedDep is a non-generic implementation of Dep that uses a provided
// name and a slice of DepName for references.
type NamedDep struct {
	name     DepName
	instance any
	refs     []DepName
	mu       sync.RWMutex
	onInit   func(ctx context.Context, deps *Dependencies) (any, error)
	onClose  func(ctx context.Context) error
}

// NewNamedDep constructs a NamedDep. 'name' must be a unique key used by the container.
// refs is a slice of dependency keys this dep depends on.
func NewNamedDep(name DepName, refs []DepName, onInit func(ctx context.Context, deps *Dependencies) (any, error), onClose func(ctx context.Context) error) Dep {
	return &NamedDep{name: name, refs: refs, onInit: onInit, onClose: onClose}
}

func (n *NamedDep) Init(ctx context.Context, deps *Dependencies) error {
	if n.onInit != nil {
		inst, err := n.onInit(ctx, deps)
		if err != nil {
			return err
		}
		n.mu.Lock()
		n.instance = inst
		n.mu.Unlock()
	}
	return nil
}

func (n *NamedDep) GetName() string { return string(n.name) }

func (n *NamedDep) GetRefs() []string {
	if n.refs == nil {
		return nil
	}
	out := make([]string, 0, len(n.refs))
	for _, r := range n.refs {
		out = append(out, string(r))
	}
	return out
}

func (n *NamedDep) Get() any {
	n.mu.RLock()
	inst := n.instance
	n.mu.RUnlock()
	return inst
}

func (n *NamedDep) Close(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	var err error
	if n.onClose != nil {
		err = n.onClose(ctx)
	}
	n.instance = nil
	return err
}
