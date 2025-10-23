// Package simple_di provides dependency management functionality.
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
package simple_di

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Container stores all registered dependencies.
// It is safe for concurrent reads (Get, Has, List), but writes (Add, Init, Close)
// must not be concurrent and must happen before initialization is complete.
type Container struct {
	lazyInit bool // allow on-demand initialization

	mu          sync.RWMutex
	deps        map[DepName]Dependency
	instances   map[DepName]any // cached initialized dependency instances
	initOrder   []DepName
	initialized bool
	closed      bool
}

// New creates a new Container instance.
func New(lazyInit bool) *Container {
	return &Container{
		deps:      make(map[DepName]Dependency),
		instances: make(map[DepName]any),
		lazyInit:  lazyInit,
	}
}

// Add stores one or more dependencies in the container.
// It is forbidden to add dependencies after initialization.
func (c *Container) Add(deps ...Dependency) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return fmt.Errorf("cannot add dependencies after initialization")
	}
	if c.closed {
		return fmt.Errorf("cannot add dependencies after close")
	}

	for _, dep := range deps {
		name := dep.Name()
		if name == "" {
			return fmt.Errorf("dependency name cannot be empty")
		}
		if _, exists := c.deps[name]; exists {
			return fmt.Errorf("dependency %q already exists", name)
		}
		c.deps[name] = dep
	}
	return nil
}

// Get retrieves a dependency by name.
// Thread-safe, but cannot be called after full initialization is complete.
func (c *Container) Get(name DepName) (any, error) {
	c.mu.RLock()
	instance, ok := c.instances[name]
	c.mu.RUnlock()
	if ok {
		return instance, nil
	}

	if !c.lazyInit {
		return nil, fmt.Errorf("dependency %q not initialized (lazyInit is disabled)", name)
	}

	return c.lazyInitAndGet(name)
}

func (c *Container) lazyInitAndGet(name DepName) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double check again
	if instance, ok := c.instances[name]; ok {
		return instance, nil
	}

	dep, exists := c.deps[name]
	if !exists {
		return nil, fmt.Errorf("dependency %q not found", name)
	}

	// Call Init
	if err := dep.Init(context.Background(), c); err != nil {
		return nil, fmt.Errorf("failed to init dependency %q: %w", name, err)
	}
	instance := dep.Get()
	c.instances[name] = instance
	c.initOrder = append(c.initOrder, name)
	return instance, nil
}

// Has checks if dependency exists (thread-safe).
func (c *Container) Has(key DepName) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.deps[key]
	return exists
}

// Init initializes all dependencies in topological order.
// Marks container as initialized after success.
func (c *Container) Init(ctx context.Context) error {
	if c.lazyInit {
		return fmt.Errorf("cannot call Init when lazyInit is enabled")
	}

	c.mu.Lock()
	if c.initialized || c.closed {
		c.mu.Unlock()
		return fmt.Errorf("container already initialized or closed")
	}
	c.initialized = true
	c.mu.Unlock()

	order, err := c.resolveOrder()
	if err != nil {
		return err
	}

	initialized := make([]DepName, 0, len(order))
	for _, name := range order {
		if _, ok := c.instances[name]; ok {
			// already initialized
			continue
		}

		dep := c.deps[name]
		if err := dep.Init(ctx, c); err != nil {
			// Rollback already initialized
			for i := len(initialized) - 1; i >= 0; i-- {
				_ = c.deps[initialized[i]].Close(ctx)
			}
			c.closed = true // prevent further use
			return fmt.Errorf("init failed for dependency %q: %w", name, err)
		}
		initialized = append(initialized, name)
		c.instances[name] = dep.Get()
	}

	c.initOrder = order
	c.initialized = true
	return nil
}

// Close shuts down dependencies in reverse order.
func (c *Container) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return fmt.Errorf("container not initialized")
	}

	var firstErr error
	for i := len(c.initOrder) - 1; i >= 0; i-- {
		name := c.initOrder[i]
		if err := c.deps[name].Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	c.initialized = false
	c.closed = true
	c.initOrder = nil
	return firstErr
}

// resolveOrder computes the initialization order (topological sort).
func (c *Container) resolveOrder() ([]DepName, error) {
	state := make(map[DepName]int)
	var order []DepName

	var visit func(DepName) error
	visit = func(n DepName) error {
		if s, ok := state[n]; ok {
			if s == 1 {
				return fmt.Errorf("cycle detected at %q", n)
			}
			if s == 2 {
				return nil
			}
		}
		state[n] = 1

		dep, exists := c.deps[n]
		if !exists {
			return fmt.Errorf("unknown dependency %q", n)
		}
		for _, ref := range dep.Refs() {
			if _, ok := c.deps[ref]; !ok {
				return fmt.Errorf("missing dependency %q required by %q", ref, n)
			}
			if err := visit(ref); err != nil {
				return err
			}
		}
		state[n] = 2
		order = append(order, n)
		return nil
	}

	for name := range c.deps {
		if state[name] == 0 {
			if err := visit(name); err != nil {
				return nil, err
			}
		}
	}
	return order, nil
}

// GetByType retrieves a dependency by its type T
func GetByType[T any](d DependenciesStore) (T, error) {
	var zero T
	typeKey := DepName(fmt.Sprintf("%T", zero))
	value, err := d.Get(typeKey)
	if err != nil {
		return zero, err
	}
	return value.(T), nil
}

// ReduceDependencies populates the fields of a struct R with dependency instances
// from the given container.
//
// Each exported field of R is treated as a dependency:
//   - If the field has a `dep:"<name>"` tag, that name is used to look up the dependency.
//   - Otherwise, the dependency key is derived from the field's type string.
//
// Example:
//
//	type MyDeps struct {
//	    DB    *sql.DB        `dep:"main_db"`
//	    Cache *redis.Client  // lookup by type
//	}
//
//	deps, err := ReduceDependencies[MyDeps](container)
//
// Returns (*R, error). Missing or duplicate dependencies are reported as errors.
func reduceDependencies[R any](c *Container, skipErrors bool) (*R, error) {
	var zero R
	t := reflect.TypeOf(zero)
	if t.Kind() != reflect.Struct {
		return &zero, fmt.Errorf("ReduceDependencies: type %T is not a struct", zero)
	}

	v := reflect.New(t).Elem()
	result := v

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Determine dependency key
		depName := DepName(field.Tag.Get("dep"))
		if depName == "" {
			depName = DepName(field.Type.String())
		}

		instance, ok := c.instances[depName]
		if !ok {
			// Try lazy Get
			var err error
			instance, err = c.Get(depName)
			if err != nil {
				if skipErrors {
					continue
				}
				return nil, fmt.Errorf(
					"ReduceDependencies: missing dependency for field %q (%s): %w", field.Name, depName, err,
				)
			}
		}

		// Fill the field
		fv := result.Field(i)
		if !fv.CanSet() {
			if skipErrors {
				continue
			}
			return nil, fmt.Errorf("ReduceDependencies: field %q cannot be set", field.Name)
		}
		fv.Set(reflect.ValueOf(instance))
	}

	final := result.Addr().Interface().(*R)
	return final, nil
}

// ReduceDependencies populates the fields of a struct R with dependency instances
// from the given container.
//
// Each exported field of R is treated as a dependency:
//   - If the field has a `dep:"<name>"` tag, that name is used to look up the dependency.
//   - Otherwise, the dependency key is derived from the field's type string.
//
// Example:
//
//		type MyDeps struct {
//		    DB    *sql.DB        `dep:"main_db"`
//		    Cache *redis.Client  // lookup by type
//		}
//
//	 	func MyFunc(deps *MyDeps) {...}
//
//		MyFunc(ReduceDependencies[MyDeps](container))
//
// Missing dependencies and private fields are ignored.
func ReduceDependencies[R any](deps DependenciesStore) *R {
	rDeps, _ := reduceDependencies[R](deps.(*Container), true)
	return rDeps
}

// MustReduceDependencies populates the fields of a struct R with dependency instances
// from the given container.
//
// Each exported field of R is treated as a dependency:
//   - If the field has a `dep:"<name>"` tag, that name is used to look up the dependency.
//   - Otherwise, the dependency key is derived from the field's type string.
//
// Example:
//
//		type MyDeps struct {
//		    DB    *sql.DB        `dep:"main_db"`
//		    Cache *redis.Client  // lookup by type
//		}
//
//	 	func MyFunc(deps *MyDeps) {...}
//
//		MyFunc(MustReduceDependencies[MyDeps](container))
//
// Missing dependencies and private fields leads to panic.
func MustReduceDependencies[R any](deps DependenciesStore) *R {
	rDeps, err := reduceDependencies[R](deps.(*Container), false)
	if err != nil {
		panic(err)
	}
	return rDeps
}
