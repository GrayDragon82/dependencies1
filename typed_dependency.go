package simple_di

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// TypedDependency is a small reusable implementation of Dep that stores an
// instance and delegates initialization/closing to provided callbacks.
type TypedDependency[T any, R any] struct {
	name     string
	instance T
	refsKeys []DepName
	mu       sync.RWMutex
	onInit   func(ctx context.Context, deps *R) (T, error)
	onClose  func(ctx context.Context) error
}

// NewDependency creates a new BaseDep. The name will be derived
// from the instance's of T type (fmt.Sprintf("%T", instance)).
// R represents a struct with dependencies referenced by this dep.
func NewDependency[T any, R any](onInit func(ctx context.Context, deps *R) (T, error), onClose func(ctx context.Context) error) Dependency {
	if onInit == nil {
		panic("TypedDependency.New: onInit cannot be nil")
	}
	var zero T
	name := fmt.Sprintf("%T", zero)
	bd := &TypedDependency[T, R]{name: name, onInit: onInit, onClose: onClose}
	// precompute refs keys from provided refs struct
	var zeroR R
	t := reflect.TypeOf(zeroR)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct {
		keys := make([]DepName, 0, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			keys = append(keys, DepName(t.Field(i).Type.String()))
		}
		bd.refsKeys = keys
	}
	return bd
}

// NewSimpleDependency creates a new BaseDep. The name will be derived
// from the instance's of T type (fmt.Sprintf("%T", instance)).
// This dependency has no own dependencies (refs).
func NewSimpleDependency[T any](onInit func(ctx context.Context) (T, error), onClose func(ctx context.Context) error) Dependency {
	return NewDependency(
		func(ctx context.Context, _ *struct{}) (T, error) {
			return onInit(ctx)
		},
		onClose,
	)
}

func (b *TypedDependency[T, R]) Init(ctx context.Context, deps *Container) error {
	rDeps := MustReduceDependencies[R](deps)
	inst, err := b.onInit(ctx, rDeps)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.instance = inst
	b.mu.Unlock()
	return nil
}

func (b *TypedDependency[T, R]) GetName() DepName { return DepName(b.name) }

// GetRefs returns the field names of the refs struct as dependency keys.
func (b *TypedDependency[T, R]) GetRefs() []DepName {
	if len(b.refsKeys) == 0 {
		return nil
	}
	// return a copy to avoid external mutation
	out := make([]DepName, len(b.refsKeys))
	for i, k := range b.refsKeys {
		out[i] = k
	}
	return out
}

func (b *TypedDependency[T, R]) Get() any {
	b.mu.RLock()
	inst := b.instance
	b.mu.RUnlock()
	return inst
}

func (b *TypedDependency[T, R]) Close(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	var err error
	if b.onClose != nil {
		err = b.onClose(ctx)
	}
	var zero T
	b.instance = zero
	return err
}
