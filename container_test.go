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

func (t *testDepA) Init(ctx context.Context, deps *Container) error {
	t.instance = &A{}
	return nil
}
func (t *testDepA) GetName() DepName                { return "a" }
func (t *testDepA) GetRefs() []DepName              { return nil }
func (t *testDepA) Get() any                        { return t.instance }
func (t *testDepA) Close(ctx context.Context) error { t.instance = nil; return nil }

type testDepB struct {
	instance any
}

func (t *testDepB) Init(ctx context.Context, deps *Container) error {
	// use referenced dependency "c" to build own instance
	if cInst, ok := deps.Get("c"); ok && cInst != nil {
		if cPtr, ok := cInst.(*C); ok {
			t.instance = &B{c: cPtr}
			return nil
		}
	}
	t.instance = &B{c: nil}
	return nil
}
func (t *testDepB) GetName() DepName                { return "b" }
func (t *testDepB) GetRefs() []DepName              { return []DepName{"c"} }
func (t *testDepB) Get() any                        { return t.instance }
func (t *testDepB) Close(ctx context.Context) error { t.instance = nil; return nil }

type testDepC struct{ instance any }

func (t *testDepC) Init(ctx context.Context, deps *Container) error {
	t.instance = &C{}
	return nil
}
func (t *testDepC) GetName() DepName                { return "c" }
func (t *testDepC) GetRefs() []DepName              { return nil }
func (t *testDepC) Get() any                        { return t.instance }
func (t *testDepC) Close(ctx context.Context) error { t.instance = nil; return nil }

// package-scope helper types for error tests
type missingDep struct{}

func (m *missingDep) Init(ctx context.Context, deps *Container) error { return nil }
func (m *missingDep) GetName() DepName                                { return "m" }
func (m *missingDep) GetRefs() []DepName                              { return []DepName{"nope"} }
func (m *missingDep) Get() any                                        { return nil }
func (m *missingDep) Close(ctx context.Context) error                 { return nil }

type depX struct{}

func (x *depX) Init(ctx context.Context, deps *Container) error { return nil }
func (x *depX) GetName() DepName                                { return "x" }
func (x *depX) GetRefs() []DepName                              { return []DepName{"y"} }
func (x *depX) Get() any                                        { return nil }
func (x *depX) Close(ctx context.Context) error                 { return nil }

type depY struct{}

func (y *depY) Init(ctx context.Context, deps *Container) error { return nil }
func (y *depY) GetName() DepName                                { return "y" }
func (y *depY) GetRefs() []DepName                              { return []DepName{"x"} }
func (y *depY) Get() any                                        { return nil }
func (y *depY) Close(ctx context.Context) error                 { return nil }

func TestDependencies_Add_Get_Require(t *testing.T) {
	r := require.New(t)
	d := New()

	// Add pointer values only (Add requires pointer Dep)
	d.Add(&testDepA{}, &testDepB{}, &testDepC{})

	// initialize dependencies first
	r.NoError(d.Init(context.Background()), "Init failed")

	// Get by key (returns prepared instance)
	v, ok := d.Get("a")
	r.True(ok, "expected to find key 'a'")

	a, ok := v.(*A)
	r.Truef(ok, "expected *A for key a, got: %T", v)
	r.Equal("A", a.A(), "unexpected A.A() result")

	v, ok = d.Get("b")
	r.True(ok, "expected to find key 'b'")

	// check B.B() result to verify dependency on C was injected
	b, ok := v.(*B)
	r.Truef(ok, "expected *B for key b, got: %T", v)
	r.Equal("B + C", b.B(), "unexpected B.B() result")

	// missing key
	_, ok = d.Get("missing")
	r.False(ok, "expected no value for 'missing'")
}

func TestDependencies_WithNamedDep(t *testing.T) {
	r := require.New(t)
	d := New()

	// Add pointer values only (Add requires pointer Dep)
	d.Add(
		NewNamedDependency("a", nil, func(ctx context.Context, deps *Container) (any, error) { return &A{}, nil }, nil),
		NewNamedDependency(
			"b",
			[]DepName{"c"},
			func(ctx context.Context, deps *Container) (any, error) {
				v, ok := deps.Get("c")
				r.True(ok, "expected to find key 'c'")
				c, ok := v.(*C)
				r.Truef(ok, "expected *C for key c, got: %T", v)
				return &B{c: c}, nil
			},
			nil,
		),
		NewNamedDependency("c", nil, func(ctx context.Context, deps *Container) (any, error) { return &C{}, nil }, nil),
	)

	// initialize dependencies first
	r.NoError(d.Init(context.Background()), "Init failed")

	// Get by key (returns prepared instance)
	v, ok := d.Get("a")
	r.True(ok, "expected to find key 'a'")
	a, ok := v.(*A)
	r.Truef(ok, "expected *A for key a, got: %T", v)
	r.Equal("A", a.A(), "unexpected A.A() result")

	v, ok = d.Get("b")
	r.True(ok, "expected to find key 'b'")
	b, ok := v.(*B)
	r.Truef(ok, "expected *B for key b, got: %T", v)
	r.Equal("B + C", b.B(), "unexpected B.B() result")
}

func TestDependencies_WithBaseDep(t *testing.T) {
	r := require.New(t)
	d := New()

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
	a, ok := GetByType[*A](d)
	r.True(ok, "expected to find key 'a'")
	r.Equal("A", a.A(), "unexpected A.A() result")

	b, ok := GetByType[*B](d)
	r.True(ok, "expected to find key 'b'")
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
	d := New()
	// use package-scope helper type 'missingDep'
	m := &missingDep{}
	d.Add(m)

	err := d.Init(context.Background())
	r.Error(err)
	r.Contains(err.Error(), "missing dependency nope required by m")
}

func TestInitCycleDetection(t *testing.T) {
	r := require.New(t)
	d := New()
	// use package-scope helper types 'depX' and 'depY'
	dx := &depX{}
	dy := &depY{}
	d.Add(dx, dy)

	err := d.Init(context.Background())
	r.Error(err)
	r.Contains(err.Error(), "cycle detected")
}

func TestGetUnknownKey(t *testing.T) {
	d := New()
	if v, ok := d.Get("unknown"); ok || v != nil {
		t.Fatalf("expected Get to return (nil,false) for unknown key, got: (%v,%v)", v, ok)
	}
}
