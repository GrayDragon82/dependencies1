package simple_di

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTypedDependency_BasicsAndRefs(t *testing.T) {
	r := require.New(t)

	// NewBaseDep: onInit cannot be nil,
	r.Panics(func() {
		_ = NewDependency[*A, struct{}](nil, nil)
	})

	// BaseDep for C: no refs, onInit returns a *C
	cDep := NewDependency(
		func(ctx context.Context, deps *struct{}) (*C, error) {
			return &C{}, nil
		},
		nil,
	)

	r.Equal("*simple_di.C", string(cDep.Name()))
	r.Equal([]DepName(nil), cDep.Refs())

	// BaseDep for B: refs struct names the dependency key "c"
	bDep := NewDependency(
		func(ctx context.Context, dep *struct{ C *C }) (*B, error) { return &B{c: dep.C}, nil },
		nil,
	)
	r.Equal("*simple_di.B", string(bDep.Name()))
	r.Equal([]DepName{"*simple_di.C"}, bDep.Refs())

	r.NoError(bDep.Close(context.Background()))
	r.NoError(cDep.Close(context.Background()))
}
