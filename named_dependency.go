package simple_di

import (
	"context"
	"sync"
)

// NamedDependency is a non-generic implementation of Dep that uses a provided
// name and a slice of DepName for references.
type NamedDependency struct {
	name     DepName
	instance any
	refs     []DepName
	mu       sync.RWMutex
	onInit   func(ctx context.Context, deps DependenciesStore) (any, error)
	onClose  func(ctx context.Context) error
}

// NewNamedDependency constructs a NamedDependency. 'name' must be a unique key used by the container.
// refs is a slice of dependency keys this dependency depends on.
func NewNamedDependency(
	name DepName,
	refs []DepName,
	onInit func(ctx context.Context, deps DependenciesStore) (any, error),
	onClose func(ctx context.Context) error,
) Dependency {
	return &NamedDependency{name: name, refs: refs, onInit: onInit, onClose: onClose}
}

func (n *NamedDependency) Init(ctx context.Context, deps DependenciesStore) error {
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

func (n *NamedDependency) Name() DepName { return n.name }

func (n *NamedDependency) Refs() []DepName {
	if n.refs == nil {
		return nil
	}
	out := make([]DepName, 0, len(n.refs))
	out = append(out, n.refs...)
	return out
}

func (n *NamedDependency) Get() any {
	n.mu.RLock()
	inst := n.instance
	n.mu.RUnlock()
	return inst
}

func (n *NamedDependency) Close(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	var err error
	if n.onClose != nil {
		err = n.onClose(ctx)
	}
	n.instance = nil
	return err
}
