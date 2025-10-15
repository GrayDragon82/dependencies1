package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBaseDep_BasicsAndRefs(t *testing.T) {
	r := require.New(t)

	// NewBaseDep: onInit cannot be nil,
	r.Panics(func() {
		_ = NewBaseDep[*A, struct{}](nil, nil)
	})

	// BaseDep for C: no refs, onInit returns a *C
	cDep := NewBaseDep(
		func(ctx context.Context, deps *struct{}) (*C, error) {
			return &C{}, nil
		},
		nil,
	)

	// cDep.Init(context.Background(), nil)
	r.Equal("*main.C", cDep.GetName())
	r.Equal([]string(nil), cDep.GetRefs())
	// v := cDep.Get()
	// r.NotNil(v)
	// _, ok := v.(*C)
	// if !ok {
	// 	t.Fatalf("expected *C, got %T", v)
	// }

	// BaseDep for B: refs struct names the dependency key "c"
	bDep := NewBaseDep(
		func(ctx context.Context, dep *struct{ C *C }) (*B, error) { return &B{c: dep.C}, nil },
		nil,
	)
	// bDep.Init(context.Background(), nil)
	r.Equal("*main.B", bDep.GetName())
	r.Equal([]string{"*main.C"}, bDep.GetRefs())
	// v = bDep.Get()
	// r.NotNil(v)
	// if _, ok := v.(*B); !ok {
	// 	t.Fatalf("expected *B, got %T", v)
	// }

	r.NoError(bDep.Close(context.Background()))
	r.NoError(cDep.Close(context.Background()))
}
