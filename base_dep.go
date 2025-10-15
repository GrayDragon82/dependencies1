package main

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// BaseDep is a small reusable implementation of Dep that stores an
// instance and delegates initialization/closing to provided callbacks.
type BaseDep[T any, R any] struct {
	name     string
	instance T
	refsKeys []DepName
	mu       sync.RWMutex
	onInit   func(ctx context.Context, deps *R) (T, error)
	onClose  func(ctx context.Context) error
}

// NewBaseDep creates a new BaseDep. The name will be derived
// from the instance's concrete type (fmt.Sprintf("%T", instance)).
// refs should be a struct whose fields are other dependencies referenced by this dep.
func NewBaseDep[T any, R any](onInit func(ctx context.Context, deps *R) (T, error), onClose func(ctx context.Context) error) Dep {
	if onInit == nil {
		panic("BaseDep.New: onInit cannot be nil")
	}
	var zero T
	name := fmt.Sprintf("%T", zero)
	bd := &BaseDep[T, R]{name: name, onInit: onInit, onClose: onClose}
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

func NewSimpleBaseDep[T any](onInit func(ctx context.Context) (T, error), onClose func(ctx context.Context) error) Dep {
	return NewBaseDep(
		func(ctx context.Context, _ *struct{}) (T, error) {
			return onInit(ctx)
		},
		onClose,
	)
}

func (b *BaseDep[T, R]) Init(ctx context.Context, deps *Dependencies) error {
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

func (b *BaseDep[T, R]) GetName() string { return b.name }

// GetRefs returns the field names of the refs struct as dependency keys.
func (b *BaseDep[T, R]) GetRefs() []string {
	if len(b.refsKeys) == 0 {
		return nil
	}
	// return a copy to avoid external mutation
	out := make([]string, len(b.refsKeys))
	for i, k := range b.refsKeys {
		out[i] = string(k)
	}
	return out
}

func (b *BaseDep[T, R]) Get() any {
	b.mu.RLock()
	inst := b.instance
	b.mu.RUnlock()
	return inst
}

func (b *BaseDep[T, R]) Close(ctx context.Context) error {
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
