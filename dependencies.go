// Package main provides dependency management functionality.
//
// The package implements a simple dependency container that can store and retrieve
// objects implementing the Nameable interface. Dependencies can be accessed either
// by their name or by their type.
//
// Example usage:
//
//	type MyDep struct{ name string }
//	func (m *MyDep) GetName() string { return m.name }
//
//	deps := NewDependencies()
//	deps.Add(&MyDep{name: "example"})
//
//	// Get by name
//	if dep, exists := deps.Get("example"); exists {
//	    // use dep
//	}
//
//	// Get by type
//	if dep, exists := GetByType[*MyDep](deps); exists {
//	    // use dep
//	}
//
// The package provides two main ways to retrieve dependencies:
// 1. By name using the Get method
// 2. By type using either GetByType method or GetByType generic function
//
// Thread Safety: The implementation is not concurrency-safe and should be
// protected by appropriate synchronization mechanisms if used in concurrent
// environments.
package main

import (
	"context"
	"fmt"
	"reflect"
)

// Dependencies holds a map of dependency objects implementing Dep
type Dependencies struct {
	deps map[string]Dep
	// lastOrder holds the last successful initialization order so Close can
	// reuse it without recomputing the topological order.
	lastOrder []string
}

// NewDependencies creates a new Dependencies instance
func NewDependencies() *Dependencies {
	return &Dependencies{
		deps: make(map[string]Dep),
	}
}

// Add stores one or more dependencies. Each dependency is stored by its
// GetName() and by its concrete type key. Each value must be a pointer;
// passing a non-pointer will cause a panic.
func (d *Dependencies) Add(values ...Dep) {
	for _, value := range values {
		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Ptr {
			panic("Dependencies.Add: value must be a pointer")
		}
		d.deps[value.GetName()] = value
	}
}

// GetByType retrieves a dependency by its type T
func GetByType[T any](d *Dependencies) (T, bool) {
	var zero T
	typeKey := fmt.Sprintf("%T", zero)
	if value, exists := d.Get(typeKey); exists {
		return value.(T), true
	}
	return zero, false
}

// Get retrieves a dependency by key and returns the prepared instance (Dep.Get()).
func (d *Dependencies) Get(key string) (any, bool) {
	dep, exists := d.deps[key]
	if !exists {
		return nil, false
	}
	return dep.Get(), true
}

func ReduceDependencies[R any](deps *Dependencies) *R {
	rDeps := new(R)
	t := reflect.TypeOf(rDeps)
	v := reflect.ValueOf(rDeps)
	for i := 0; i < t.Elem().NumField(); i++ {
		key := DepName(t.Elem().Field(i).Type.String())
		val, found := deps.Get(string(key))
		if found {
			v.Elem().Field(i).Set(reflect.ValueOf(val))
		}
	}
	return rDeps
}

func MustReduceDependencies[R any](deps *Dependencies) *R {
	rDeps := new(R)
	t := reflect.TypeOf(rDeps)
	v := reflect.ValueOf(rDeps)
	for i := 0; i < t.Elem().NumField(); i++ {
		key := DepName(t.Elem().Field(i).Type.String())
		val, found := deps.Get(string(key))
		if !found {
			panic(fmt.Sprintf("MustReduceDependencies: missing dependency %s", key))
		}
		v.Elem().Field(i).Set(reflect.ValueOf(val))
	}
	return rDeps
}

// Init initializes all registered dependencies in an order that satisfies
// their GetRefs() requirements (dependencies are initialized before
// dependents). If initialization of any dependency fails, already
// initialized dependencies are closed in reverse order.
func (d *Dependencies) Init(ctx context.Context) error {
	order, err := d.topoOrder()
	if err != nil {
		return err
	}

	initialized := make([]string, 0, len(order))
	for _, name := range order {
		dep := d.deps[name]
		if err := dep.Init(ctx, d); err != nil {
			// attempt to close already-initialized in reverse order
			for i := len(initialized) - 1; i >= 0; i-- {
				_ = d.deps[initialized[i]].Close(ctx)
			}
			return err
		}
		initialized = append(initialized, name)
	}
	// record successful initialization order for faster Close
	d.lastOrder = order
	return nil
}

// Close closes all dependencies in reverse initialization order.
func (d *Dependencies) Close(ctx context.Context) error {
	var order []string
	if len(d.lastOrder) > 0 {
		order = d.lastOrder
	} else {
		var err error
		order, err = d.topoOrder()
		if err != nil {
			return err
		}
	}
	var firstErr error
	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		if err := d.deps[name].Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	// clear cached order after close
	d.lastOrder = nil
	return firstErr
}

// topoOrder performs a topological sort of dependencies based on GetRefs.
// Returns an order where dependencies appear before dependents.
func (d *Dependencies) topoOrder() ([]string, error) {
	// build adjacency using deps map
	// states: 0=unvisited,1=visiting,2=done
	state := make(map[string]int)
	var order []string

	var visit func(string) error
	visit = func(n string) error {
		if s, ok := state[n]; ok {
			if s == 1 {
				return fmt.Errorf("cycle detected at %s", n)
			}
			if s == 2 {
				return nil
			}
		}
		state[n] = 1
		dep, exists := d.deps[n]
		if !exists {
			return fmt.Errorf("unknown dependency: %s", n)
		}
		for _, ref := range dep.GetRefs() {
			if _, ok := d.deps[ref]; !ok {
				return fmt.Errorf("missing dependency %s required by %s", ref, n)
			}
			if err := visit(ref); err != nil {
				return err
			}
		}
		state[n] = 2
		order = append(order, n)
		return nil
	}

	for name := range d.deps {
		if state[name] == 0 {
			if err := visit(name); err != nil {
				return nil, err
			}
		}
	}
	return order, nil
}
