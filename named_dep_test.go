package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNamedDep_BasicsAndRefs(t *testing.T) {
	r := require.New(t)

	// NamedDep for C: no refs, instance provided
	cDep := NewNamedDep(DepName("c"), nil, func(ctx context.Context, deps *Dependencies) (any, error) { return &C{}, nil }, nil)

	cDep.Init(context.Background(), nil)
	r.Equal("c", cDep.GetName())
	r.Equal([]string(nil), cDep.GetRefs())
	v := cDep.Get()
	r.NotNil(v)
	if _, ok := v.(*C); !ok {
		t.Fatalf("expected *C, got %T", v)
	}

	// NamedDep for B: depends on c; provide instance that references c
	cInst := cDep.Get().(*C)
	bDep := NewNamedDep(DepName("b"), []DepName{"c"}, func(ctx context.Context, _ *Dependencies) (any, error) {
		return &B{c: cInst}, nil
	}, nil)

	bDep.Init(context.Background(), nil)
	r.Equal("b", bDep.GetName())
	r.Equal([]string{"c"}, bDep.GetRefs())
	vb := bDep.Get()
	r.NotNil(vb)
	if _, ok := vb.(*B); !ok {
		t.Fatalf("expected *B, got %T", vb)
	}

	r.NoError(bDep.Close(context.Background()))
	r.NoError(cDep.Close(context.Background()))
}
