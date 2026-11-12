package qe

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestArithmeticCooperBoundedResidueAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 512 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			context, err := NewSolverContext(m, nil)
			fmMust(t, err)
			defer func() { fmMust(t, context.Close()) }()
			plugin, err := NewArithmeticPlugin(context, f.r.config)
			fmMust(t, err)
			fmMust(t, context.AddPlugin(plugin))
			runtime := &ArithmeticCooperRuntime{plugin: plugin}
			m.Budget().FailAfter(stop)
			value, err := runtime.boundedVariable(big.NewInt(8))
			m.Budget().FailAfter(-1)
			var failure memory.Failure
			injected := errors.Is(err, arena.ErrInjectedFailure) || (errors.As(err, &failure) && failure.Kind == memory.FailureMemout)
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				fmMust(t, construction.Cleanup())
			}
			if err != nil {
				if !injected {
					t.Fatal("unexpected construction error", err)
				}
				failures++
			} else {
				success = true
				if len(runtime.fresh) != 1 {
					t.Fatal("fresh bit-vector owner missing")
				}
				bits, e := m.RefGet(runtime.fresh[0])
				fmMust(t, e)
				width, e := m.BitVectorSize(bits)
				fmMust(t, e)
				if width != 4 {
					t.Fatal("power-of-two source width", width)
				}
				arg, e := m.GetArg(value, 0)
				fmMust(t, e)
				if arg != bits {
					t.Fatal("integer conversion does not reference the fresh bit-vector")
				}
				if sort, e := m.GetSort(value); e != nil || sort != f.integer {
					t.Fatal("residue is not an integer", sort, e)
				}
				if kind, e := m.GetKind(value); e != nil || kind != ast.ASTApp {
					t.Fatal("invalid conversion", kind, e)
				}
			}
			fmMust(t, runtime.close())
		})
	}
	if !success || failures == 0 {
		t.Fatal("did not cover failure and success", failures, success)
	}
}
