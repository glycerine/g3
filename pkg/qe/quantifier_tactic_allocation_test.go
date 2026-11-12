package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestQuantifierScalarAllocationFrontiers(t *testing.T) {
	for _, name := range []string{"boolean", "arithmetic-equation", "arithmetic-search"} {
		t.Run(name, func(t *testing.T) {
			success, failures := false, 0
			for stop := int64(0); stop < 1024 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					f := newFMFixture(t)
					m := f.r.manager
					fmMust(t, m.RegisterDeclarationPlugins())
					sort := f.integer
					if name == "boolean" {
						sort = f.boolean
					}
					x := f.constant("frontier-x", sort)
					y := f.constant("frontier-y", sort)
					body := f.own(m.MkEq(x, y))
					if name == "arithmetic-search" {
						body = f.own(m.MkArithmeticGT(x, y))
					}
					bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{x}, body)
					fmMust(t, err)
					t.Cleanup(func() { fmMust(t, m.DelRef(&bound)) })
					q, err := m.RefGet(bound)
					fmMust(t, err)
					m.Budget().FailAfter(stop)
					var result ast.ASTRef
					var complete bool
					switch name {
					case "boolean":
						result, complete, err = qeTestEliminate(m, q, f.r.config)
					case "arithmetic-equation":
						result, complete, err = qeTestEliminate(m, q, f.r.config)
					case "arithmetic-search":
						result, complete, err = qeTestEliminate(m, q, f.r.config)
					}
					m.Budget().FailAfter(-1)
					if result != (ast.ASTRef{}) {
						fmMust(t, m.DelRef(&result))
					}
					if err == nil {
						if !complete {
							t.Fatal("scalar quantifier was not eliminated")
						}
						success = true
						return
					}
					var construction memory.ConstructionFailure
					if errors.As(err, &construction) {
						fmMust(t, construction.Cleanup())
					}
					var memout memory.Failure
					if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
						t.Fatal(err)
					}
					failures++
				})
				if t.Failed() {
					t.Fatal("scalar QE allocation frontier", stop)
				}
			}
			if !success || failures == 0 {
				t.Fatal("did not reach both failure and success", failures, success)
			}
		})
	}
}
