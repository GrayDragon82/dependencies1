/*
Package simple_di provides testing for the dependency injection container implementation.

Test Types:
  - serviceA: A struct implementing Nameable interface with value receiver
  - serviceB: A struct implementing Nameable interface with pointer receiver
  - serviceC: A struct implementing Nameable interface used for negative testing

Test Functions:
  - TestDependencies_Add_Get: Tests basic Add and Get operations using keys
  - TestDependencies_GetByType_NonGeneric: Tests type-based retrieval using string type keys
  - TestGetByType_Generic: Tests generic type-based dependency retrieval
  - TestDependencies_Add_Get_Require: Same as TestDependencies_Add_Get but using testify/require
  - TestDependencies_GetByType_NonGeneric_Require: Same as TestDependencies_GetByType_NonGeneric but using testify/require
  - TestGetByType_Generic_Require: Same as TestGetByType_Generic but using testify/require

Each test validates the proper storage and retrieval of dependencies using both key-based
and type-based approaches, including support for both value and pointer types. The tests
also verify proper handling of missing dependencies and invalid type requests.
*/
package simple_di

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type A struct {
}

func (a A) A() string { return "A" }

type B struct {
	c *C
}

func (b *B) B() string { return "B + " + b.c.C() }

type C struct {
}

func (c *C) C() string { return "C" }

// test types implementing Dep
type testDepA struct {
	instance any
}

func (t *testDepA) Init(ctx context.Context, deps DependenciesStore) error {
	t.instance = &A{}
	return nil
}
func (t *testDepA) Name() DepName                   { return "a" }
func (t *testDepA) Refs() []DepName                 { return nil }
func (t *testDepA) Get() any                        { return t.instance }
func (t *testDepA) Close(ctx context.Context) error { t.instance = nil; return nil }

type testDepB struct {
	instance any
}

func (t *testDepB) Init(ctx context.Context, deps DependenciesStore) error {
	// use referenced dependency "c" to build own instance
	if cInst, err := deps.Get("c"); err == nil && cInst != nil {
		if cPtr, ok := cInst.(*C); ok {
			t.instance = &B{c: cPtr}
			return nil
		}
	}
	t.instance = &B{c: nil}
	return nil
}
func (t *testDepB) Name() DepName                   { return "b" }
func (t *testDepB) Refs() []DepName                 { return []DepName{"c"} }
func (t *testDepB) Get() any                        { return t.instance }
func (t *testDepB) Close(ctx context.Context) error { t.instance = nil; return nil }

type testDepC struct{ instance any }

func (t *testDepC) Init(ctx context.Context, deps DependenciesStore) error {
	t.instance = &C{}
	return nil
}
func (t *testDepC) Name() DepName                   { return "c" }
func (t *testDepC) Refs() []DepName                 { return nil }
func (t *testDepC) Get() any                        { return t.instance }
func (t *testDepC) Close(ctx context.Context) error { t.instance = nil; return nil }

// package-scope helper types for error tests
type missingDep struct{}

func (m *missingDep) Init(ctx context.Context, deps DependenciesStore) error { return nil }
func (m *missingDep) Name() DepName                                          { return "m" }
func (m *missingDep) Refs() []DepName                                        { return []DepName{"nope"} }
func (m *missingDep) Get() any                                               { return nil }
func (m *missingDep) Close(ctx context.Context) error                        { return nil }

type depX struct{}

func (x *depX) Init(ctx context.Context, deps DependenciesStore) error { return nil }
func (x *depX) Name() DepName                                          { return "x" }
func (x *depX) Refs() []DepName                                        { return []DepName{"y"} }
func (x *depX) Get() any                                               { return nil }
func (x *depX) Close(ctx context.Context) error                        { return nil }

type depY struct{}

func (y *depY) Init(ctx context.Context, deps DependenciesStore) error { return nil }
func (y *depY) Name() DepName                                          { return "y" }
func (y *depY) Refs() []DepName                                        { return []DepName{"x"} }
func (y *depY) Get() any                                               { return nil }
func (y *depY) Close(ctx context.Context) error                        { return nil }

func TestDependencies_Add_Get_Require(t *testing.T) {
	r := require.New(t)
	d := New(false)

	// Add pointer values only (Add requires pointer Dep)
	d.Add(&testDepA{}, &testDepB{}, &testDepC{})

	// initialize dependencies first
	r.NoError(d.Init(context.Background()), "Init failed")

	// Get by key (returns prepared instance)
	v, err := d.Get("a")
	r.NoError(err, "expected to find key 'a'")

	a, ok := v.(*A)
	r.Truef(ok, "expected *A for key a, got: %T", v)
	r.Equal("A", a.A(), "unexpected A.A() result")

	v, err = d.Get("b")
	r.NoError(err, "expected to find key 'b'")

	// check B.B() result to verify dependency on C was injected
	b, ok := v.(*B)
	r.Truef(ok, "expected *B for key b, got: %T", v)
	r.Equal("B + C", b.B(), "unexpected B.B() result")

	// missing key
	m, err := d.Get("missing")
	r.EqualError(err, "dependency \"missing\" not initialized (lazyInit is disabled)", "expected error for 'missing'")
	r.Nil(m, "expected nil for missing key")
}

func TestDependencies_WithNamedDep(t *testing.T) {
	r := require.New(t)
	d := New(false)

	// Add pointer values only (Add requires pointer Dep)
	r.NoError(d.Add(
		NewNamedDependency("a", nil, func(ctx context.Context, deps DependenciesStore) (any, error) { return &A{}, nil }, nil),
		NewNamedDependency(
			"b",
			[]DepName{"c"},
			func(ctx context.Context, deps DependenciesStore) (any, error) {
				v, err := deps.Get("c")
				r.NoError(err, "expected to find key 'c'")
				c, ok := v.(*C)
				r.Truef(ok, "expected *C for key c, got: %T", v)
				return &B{c: c}, nil
			},
			nil,
		),
		NewNamedDependency("c", nil, func(ctx context.Context, deps DependenciesStore) (any, error) { return &C{}, nil }, nil),
	))

	// Verify dependencies were registered using Has()
	r.True(d.Has("a"), "Has() should return true for 'a'")
	r.True(d.Has("b"), "Has() should return true for 'b'")
	r.True(d.Has("c"), "Has() should return true for 'c'")
	r.False(d.Has("nonexistent"), "Has() should return false for 'nonexistent'")

	// initialize dependencies first
	r.NoError(d.Init(context.Background()), "Init failed")

	// Get by key (returns prepared instance)
	v, err := d.Get("a")
	r.NoError(err, "expected to find key 'a'")
	a, ok := v.(*A)
	r.Truef(ok, "expected *A for key a, got: %T", v)
	r.Equal("A", a.A(), "unexpected A.A() result")

	v, err = d.Get("b")
	r.NoError(err, "expected to find key 'b'")
	b, ok := v.(*B)
	r.Truef(ok, "expected *B for key b, got: %T", v)
	r.Equal("B + C", b.B(), "unexpected B.B() result")
}

func TestDependencies_WithBaseDep(t *testing.T) {
	r := require.New(t)
	d := New(false)

	// Add pointer values only (Add requires pointer Dep)
	// NewBaseDep receives onInit and onClose funcs
	d.Add(
		NewSimpleDependency(func(ctx context.Context) (*A, error) { return &A{}, nil }, nil),
		NewDependency(
			func(ctx context.Context, deps *struct{ C *C }) (*B, error) {
				return &B{c: deps.C}, nil
			},
			nil,
		),
		NewSimpleDependency(func(ctx context.Context) (*C, error) { return &C{}, nil }, nil),
	)

	// initialize dependencies first
	r.NoError(d.Init(context.Background()), "Init failed")

	// Get by type (returns prepared instance)
	a, err := GetByType[*A](d)
	r.NoError(err, "expected to find key 'a'")
	r.Equal("A", a.A(), "unexpected A.A() result")

	b, err := GetByType[*B](d)
	r.NoError(err, "expected to find key 'b'")
	// check B.B() result to verify dependency on C was injected
	r.Equal("B + C", b.B(), "unexpected B.B() result")

	type testDeps struct {
		A *A
		B *B
	}
	testInjectionFunc := func(deps *testDeps) {
		r.Equal("A", deps.A.A(), "unexpected A.A() result in injected deps")
		r.Equal("B + C", deps.B.B(), "unexpected B.B() result in injected deps")
	}
	testInjectionFunc(MustReduceDependencies[testDeps](d))

	// close dependencies
	r.NoError(d.Close(context.Background()), "Close failed")
}

func TestInitMissingDependency(t *testing.T) {
	r := require.New(t)
	d := New(false)
	// use package-scope helper type 'missingDep'
	m := &missingDep{}
	r.NoError(d.Add(m))

	// Verify the dependency is registered
	r.True(d.Has("m"), "Has() should return true for registered dependency 'm'")
	// But its required dependency doesn't exist
	r.False(d.Has("nope"), "Has() should return false for missing dependency 'nope'")

	err := d.Init(context.Background())
	r.Error(err)
	r.Contains(err.Error(), "missing dependency \"nope\" required by \"m\"")
}

func TestInitCycleDetection(t *testing.T) {
	r := require.New(t)
	d := New(false)
	// use package-scope helper types 'depX' and 'depY'
	dx := &depX{}
	dy := &depY{}
	d.Add(dx, dy)

	err := d.Init(context.Background())
	r.Error(err)
	r.Contains(err.Error(), "cycle detected")
}

// TestReduceDependencies_ComprehensiveScenario tests all ReduceDependencies features:
// - Custom dep tags
// - Duplicate types with different names
// - Private fields (should be skipped)
// - Type-based and tag-based lookup
func TestReduceDependencies(t *testing.T) {
	r := require.New(t)
	d := New(false)

	// Add various dependencies with both custom names and type-based names
	r.NoError(d.Add(
		// Type-based dependencies
		NewSimpleDependency(func(ctx context.Context) (*A, error) { return &A{}, nil }, nil),

		// Named dependencies - multiple instances of same type
		NewNamedDependency("c", nil, func(ctx context.Context, deps DependenciesStore) (any, error) {
			return &C{}, nil
		}, nil),
	))

	r.NoError(d.Init(context.Background()))

	// Test struct covering all scenarios:
	// 1. Custom tags
	// 2. Duplicate types with tags
	// 3. Type-based implicit lookup
	// 4. Missing dependency (will be nil with ReduceDependencies, panic with MustReduceDependencies)
	// 5. Private fields (skipped)
	type ComprehensiveDeps struct {
		// Type-based lookup (implicit)
		A1 *A
		A2 *A

		// Name-based with custom tag mapping
		C1 *C `dep:"c"`
		C2 *C `dep:"c"`

		// Custom named dependency
		SpecialB *B `dep:"b"`

		// Missing dependency
		A3 *A `dep:"a"`
		B  *B
		C3 *C

		// Private field (should be skipped)
		privateA *A
		privateB *C `dep:"c"`
	}

	// Test ReduceDependencies (soft-fail mode)
	deps := ReduceDependencies[ComprehensiveDeps](d)
	r.NotNil(deps)

	// Type-based lookup should work
	r.NotNil(deps.A1, "A1 should be populated via type-based lookup")
	r.NotNil(deps.A2, "A2 should be populated via type-based lookup")
	r.Equal("A", deps.A1.A())
	r.Equal("A", deps.A2.A())

	// Custom tags should work
	r.NotNil(deps.C1, "C1 should be populated via custom tag")
	r.NotNil(deps.C2, "C2 should be populated via custom tag")
	r.Equal("C", deps.C1.C())
	r.Equal("C", deps.C2.C())

	// Missing dependency should be nil (skipped in soft-fail mode)
	r.Nil(deps.A3, "A3 should be nil (missing dependency)")
	r.Nil(deps.B, "B should be nil (missing dependency)")
	r.Nil(deps.C3, "C3 should be nil (missing dependency)")

	// Private field should remain nil (not accessible)
	r.Nil(deps.privateA, "privateA should remain nil (private field)")
	r.Nil(deps.privateB, "privateB should remain nil (private field)")
}

// TestReduceDependencies_MissingDependencies tests behavior with missing dependencies
func TestReduceDependencies_MissingDependencies(t *testing.T) {
	r := require.New(t)
	d := New(false)

	// Add only some dependencies
	r.NoError(d.Add(
		NewSimpleDependency(func(ctx context.Context) (*A, error) { return &A{}, nil }, nil),
		NewNamedDependency("c", nil, func(ctx context.Context, deps DependenciesStore) (any, error) {
			return &C{}, nil
		}, nil),
	))

	// Verify which dependencies are registered using Has()
	r.True(d.Has("*simple_di.A"), "Has() should return true for registered '*simple_di.A'")
	r.True(d.Has("c"), "Has() should return true for registered 'c'")
	r.False(d.Has("a"), "Has() should return false for unregistered 'a'")
	r.False(d.Has("b"), "Has() should return false for unregistered 'b'")
	r.False(d.Has("*simple_di.B"), "Has() should return false for unregistered '*simple_di.B'")
	r.False(d.Has("*simple_di.C"), "Has() should return false for unregistered '*simple_di.C'")

	r.NoError(d.Init(context.Background()))

	// Test struct with missing dependencies
	type DepsWithMissing struct {
		A1       *A
		C1       *C `dep:"c"`
		A3       *A `dep:"a"` // missing
		B        *B // missing
		C3       *C // missing (type-based)
		SpecialB *B `dep:"b"` // missing
	}

	// ReduceDependencies should skip missing dependencies (soft-fail mode)
	deps := ReduceDependencies[DepsWithMissing](d)
	r.NotNil(deps)

	// Present dependencies should be populated
	r.NotNil(deps.A1, "A1 should be populated")
	r.NotNil(deps.C1, "C1 should be populated")
	r.Equal("A", deps.A1.A())
	r.Equal("C", deps.C1.C())

	// Missing dependencies should be nil (skipped in soft-fail mode)
	r.Nil(deps.A3, "A3 should be nil (missing dependency)")
	r.Nil(deps.B, "B should be nil (missing dependency)")
	r.Nil(deps.C3, "C3 should be nil (missing dependency)")
	r.Nil(deps.SpecialB, "SpecialB should be nil (missing dependency)")
}

// TestMustReduceDependencies tests hard-fail behavior with missing dependencies
func TestMustReduceDependencies(t *testing.T) {
	r := require.New(t)
	d := New(false)

	r.NoError(d.Add(
		NewSimpleDependency(func(ctx context.Context) (*A, error) { return &A{}, nil }, nil),
		NewNamedDependency("c", nil, func(ctx context.Context, deps DependenciesStore) (any, error) {
			return &C{}, nil
		}, nil),
	))

	r.NoError(d.Init(context.Background()))

	// Test with struct that has missing dependencies - should panic
	type DepsWithMissing struct {
		A1 *A
		C1 *C `dep:"c"`
		B  *B // missing
	}

	r.Panics(func() {
		MustReduceDependencies[DepsWithMissing](d)
	}, "MustReduceDependencies should panic on missing dependency")

	// Test with struct that has all dependencies present - should not panic
	type AllPresentDeps struct {
		A1 *A
		C1 *C `dep:"c"`
	}

	r.NotPanics(func() {
		allDeps := MustReduceDependencies[AllPresentDeps](d)
		r.NotNil(allDeps)
		r.NotNil(allDeps.A1)
		r.NotNil(allDeps.C1)
	}, "MustReduceDependencies should not panic when all dependencies are present")
}

// TestReduceDependencies_EmptyStruct tests behavior with empty struct
func TestReduceDependencies_EmptyStruct(t *testing.T) {
	r := require.New(t)
	d := New(false)

	r.NoError(d.Add(
		NewSimpleDependency(func(ctx context.Context) (*A, error) { return &A{}, nil }, nil),
	))

	r.NoError(d.Init(context.Background()))

	// Test struct with no dependencies
	type EmptyStruct struct{}

	// ReduceDependencies should return an instance of the struct
	emptyDeps := MustReduceDependencies[EmptyStruct](d)
	r.NotNil(emptyDeps)

	// The instance should be empty (no dependencies injected)
	r.IsType(&EmptyStruct{}, emptyDeps)
}

// TestLazyInit_BasicGet tests lazy initialization with basic Get operations
func TestLazyInit_BasicGet(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	// Add dependencies without calling Init()
	r.NoError(d.Add(
		&testDepA{},
		&testDepB{}, // depends on "c"
		&testDepC{},
	))

	// Verify dependencies are registered but not initialized
	r.True(d.Has("a"), "Dependency 'a' should be registered")
	r.True(d.Has("b"), "Dependency 'b' should be registered")
	r.True(d.Has("c"), "Dependency 'c' should be registered")

	// Get should trigger lazy initialization
	vA, err := d.Get("a")
	r.NoError(err, "Lazy init should succeed for 'a'")
	r.NotNil(vA, "Instance should not be nil")

	a, ok := vA.(*A)
	r.True(ok, "Should be *A type")
	r.Equal("A", a.A(), "A should work correctly")

	// Get second dependency
	vB, err := d.Get("b")
	r.NoError(err, "Lazy init should succeed for 'c'")
	r.NotNil(vB, "Instance should not be nil")

	b, ok := vB.(*B)
	r.True(ok, "Should be *B type")
	r.Equal("B + C", b.B(), "B should work correctly")

	// Getting again should return cached instance
	vA2, err := d.Get("a")
	r.NoError(err, "Second Get should succeed")
	r.Same(vA, vA2, "Should return same cached instance")
}

// TestLazyInit_WithDependencies tests lazy initialization with dependencies
func TestLazyInit_WithDependencies(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	// Add dependencies with references
	r.NoError(d.Add(
		&testDepB{}, // depends on "c"
		&testDepC{},
	))

	// Get B should trigger lazy init of both B and C
	v, err := d.Get("b")
	r.NoError(err, "Lazy init should succeed for 'b'")
	r.NotNil(v, "Instance should not be nil")

	b, ok := v.(*B)
	r.True(ok, "Should be *B type")
	r.Equal("B + C", b.B(), "B should have C injected")

	// C should also be initialized now
	v, err = d.Get("c")
	r.NoError(err, "C should be initialized")
	r.NotNil(v, "C instance should not be nil")
}

// TestLazyInit_CannotCallInit tests that Init() fails when lazyInit is enabled
func TestLazyInit_CannotCallInit(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	r.NoError(d.Add(&testDepA{}))

	// Calling Init() should fail
	err := d.Init(context.Background())
	r.Error(err, "Init() should fail when lazyInit is enabled")
	r.Contains(err.Error(), "cannot call Init when lazyInit is enabled")
}

// TestLazyInit_MissingDependency tests error handling for missing dependencies
func TestLazyInit_MissingDependency(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	// Add dependency that requires non-existent dependency
	r.NoError(d.Add(&missingDep{}))

	// Get should fail because required dependency is missing
	_, err := d.Get("m")
	r.Error(err, "Get should fail for missing required dependency")
	r.EqualError(err, "failed to init dependency \"nope\" required by \"m\": dependency \"nope\" not found")
}

// TestLazyInit_ConcurrentGet tests concurrent access with lazy initialization
func TestLazyInit_ConcurrentGet(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	r.NoError(d.Add(
		&testDepA{},
		&testDepC{},
	))

	// Simulate concurrent access
	var wg sync.WaitGroup
	results := make([]any, 10)
	errors := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v, err := d.Get("a")
			results[idx] = v
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// All should succeed
	for i := 0; i < 10; i++ {
		r.NoError(errors[i], "Concurrent Get should not error")
		r.NotNil(results[i], "Result should not be nil")
	}

	// All should return the same instance (cached)
	firstInstance := results[0]
	for i := 1; i < 10; i++ {
		r.Same(firstInstance, results[i], "All should return same cached instance")
	}
}

// TestLazyInit_NonExistentDependency tests getting a non-existent dependency
func TestLazyInit_NonExistentDependency(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	r.NoError(d.Add(&testDepA{}))

	// Try to get non-existent dependency
	_, err := d.Get("nonexistent")
	r.Error(err, "Get should fail for non-existent dependency")
	r.Contains(err.Error(), "not found")
}

// TestLazyInit_WithNamedDependencies tests lazy init with named dependencies
func TestLazyInit_WithNamedDependencies(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	// Add named dependencies
	r.NoError(d.Add(
		NewNamedDependency("service_a", nil, func(ctx context.Context, deps DependenciesStore) (any, error) {
			return &A{}, nil
		}, nil),
		NewNamedDependency("service_b", []DepName{"service_c"}, func(ctx context.Context, deps DependenciesStore) (any, error) {
			c, err := deps.Get("service_c")
			if err != nil {
				return nil, err
			}
			return &B{c: c.(*C)}, nil
		}, nil),
		NewNamedDependency("service_c", nil, func(ctx context.Context, deps DependenciesStore) (any, error) {
			return &C{}, nil
		}, nil),
	))

	// Verify all are registered
	r.True(d.Has("service_a"))
	r.True(d.Has("service_b"))
	r.True(d.Has("service_c"))

	// Get service_b should lazily initialize both b and c
	v, err := d.Get("service_b")
	r.NoError(err)
	b := v.(*B)
	r.Equal("B + C", b.B())

	// service_c should now be cached
	v, err = d.Get("service_c")
	r.NoError(err)
	r.NotNil(v)
}

// TestLazyInit_WithTypedDependencies tests lazy init with typed dependencies
func TestLazyInit_WithTypedDependencies(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	r.NoError(d.Add(
		NewSimpleDependency(func(ctx context.Context) (*A, error) { return &A{}, nil }, nil),
		NewDependency(
			func(ctx context.Context, deps *struct{ C *C }) (*B, error) {
				return &B{c: deps.C}, nil
			},
			nil,
		),
		NewSimpleDependency(func(ctx context.Context) (*C, error) { return &C{}, nil }, nil),
	))

	// Get by type using GetByType
	a, err := GetByType[*A](d)
	r.NoError(err)
	r.Equal("A", a.A())

	b, err := GetByType[*B](d)
	r.NoError(err)
	r.Equal("B + C", b.B())
}

// TestLazyInit_ReduceDependencies tests ReduceDependencies with lazy init
func TestLazyInit_ReduceDependencies(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	r.NoError(d.Add(
		NewSimpleDependency(func(ctx context.Context) (*A, error) { return &A{}, nil }, nil),
		NewNamedDependency("db", nil, func(ctx context.Context, deps DependenciesStore) (any, error) {
			return &C{}, nil
		}, nil),
	))

	// Use ReduceDependencies - should trigger lazy init
	type LazyDeps struct {
		ServiceA *A
		Database *C `dep:"db"`
	}

	deps := ReduceDependencies[LazyDeps](d)
	r.NotNil(deps)
	r.NotNil(deps.ServiceA)
	r.NotNil(deps.Database)
	r.Equal("A", deps.ServiceA.A())
	r.Equal("C", deps.Database.C())
}

// TestLazyInit_AddAfterGet tests that Add fails after lazy initialization starts
func TestLazyInit_AddAfterGet(t *testing.T) {
	r := require.New(t)
	d := New(true) // lazyInit enabled

	r.NoError(d.Add(&testDepA{}))

	// Trigger lazy init by getting a dependency
	_, err := d.Get("a")
	r.NoError(err)

	// Now try to add another dependency - should fail
	err = d.Add(&testDepC{})
	r.Error(err, "Add should fail after initialization has started")
	r.Contains(err.Error(), "cannot add dependencies after initialization")
}
