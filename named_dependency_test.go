package simple_di

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNamedDependency_BasicsAndRefs(t *testing.T) {
	r := require.New(t)

	// NamedDependency for C: no refs, instance provided
	cDep := NewNamedDependency(DepName("c"), nil, func(ctx context.Context, deps DependenciesStore) (any, error) { return &C{}, nil }, nil)

	cDep.Init(context.Background(), nil)
	r.Equal("c", string(cDep.Name()))
	r.Equal([]DepName(nil), cDep.Refs())
	v := cDep.Get()
	r.NotNil(v)
	if _, ok := v.(*C); !ok {
		t.Fatalf("expected *C, got %T", v)
	}

	// NamedDependency for B: depends on c; provide instance that references c
	cInst := cDep.Get().(*C)
	bDep := NewNamedDependency(DepName("b"), []DepName{"c"}, func(ctx context.Context, _ DependenciesStore) (any, error) {
		return &B{c: cInst}, nil
	}, nil)

	bDep.Init(context.Background(), nil)
	r.Equal("b", string(bDep.Name()))
	r.Equal([]DepName{"c"}, bDep.Refs())
	vb := bDep.Get()
	r.NotNil(vb)
	if _, ok := vb.(*B); !ok {
		t.Fatalf("expected *B, got %T", vb)
	}

	r.NoError(bDep.Close(context.Background()))
	r.NoError(cDep.Close(context.Background()))
}
