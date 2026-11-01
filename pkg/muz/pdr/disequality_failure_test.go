package pdr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestPDRDisequalityReductionColdFailureOwnershipAndRetry(t *testing.T) {
	// Fresh arenas expose partial-constructor failures that warmed pools hide.
	// Check every allocation checkpoint through the first successful reduction.
	for _, termCount := range []int{8, 16, 24, 32} {
		t.Run(fmt.Sprint(termCount), func(t *testing.T) {
			stops := make([]int64, 1024)
			for i := range stops {
				stops[i] = int64(i)
			}
			for stop := int64(1024); stop <= 16384; stop *= 2 {
				stops = append(stops, stop)
			}
			succeeded, failures := false, 0
			for _, stop := range stops {
				if succeeded {
					break
				}
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					m, oldOwner, transformer, _, closeBase := closureFixture(t)
					config := oldOwner.contexts.config
					closureMust(t, transformer.Close())
					closureMust(t, oldOwner.Close())
					c, err := NewContext(m, nil, 2, ReachableNoCache, config)
					closureMust(t, err)
					defer func() { closureMust(t, c.Close()); closeBase() }()
					var pins []ast.ASTRef
					defer func() { closureMust(t, closeASTRefs(m, pins)) }()
					own := func(h ast.ASTHandle, err error) ast.ASTHandle {
						t.Helper()
						closureMust(t, err)
						ref, err := m.NewRef(h)
						closureMust(t, err)
						pins = append(pins, ref)
						return h
					}
					seven := own(m.ArithmeticRealNumeralText("7"))
					sort, err := m.GetSort(seven)
					closureMust(t, err)
					numbers := []ast.ASTHandle{own(m.ArithmeticRealNumeralText("0")), own(m.ArithmeticRealNumeralText("1")), own(m.ArithmeticRealNumeralText("2"))}
					md, err := model.NewModel(m)
					closureMust(t, err)
					defer func() { closureMust(t, md.Close()) }()
					variables := make([]ast.ASTHandle, termCount)
					var conjuncts []ast.ASTHandle
					for i := range variables {
						variables[i] = own(m.MkFreshConst("repeated-disequality", sort))
						decl, err := m.GetDecl(variables[i])
						closureMust(t, err)
						closureMust(t, md.Core().RegisterConstant(decl, seven))
						for _, number := range numbers {
							conjuncts = append(conjuncts, own(m.MkNot(own(m.MkEq(variables[i], number)))))
						}
					}
					formula := own(m.MkAndArray(conjuncts))
					counts := make([]uint32, len(variables))
					for i, x := range variables {
						counts[i], err = m.GetRefCount(x)
						closureMust(t, err)
					}
					before := m.Stats().LiveReferences
					m.Budget().FailAfter(stop)
					result, err := c.reduceDisequalities(md, 3, formula)
					m.Budget().FailAfter(-1)
					if result != (ast.ASTRef{}) {
						closureMust(t, m.DelRef(&result))
					}
					if err != nil {
						var cleanup memory.ConstructionFailure
						if errors.As(err, &cleanup) {
							closureMust(t, cleanup.Cleanup())
						}
						var failure memory.Failure
						if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
							t.Fatal("unexpected reduction failure", err)
						}
						failures++
					} else {
						succeeded = true
					}
					if got := m.Stats().LiveReferences; got != before {
						t.Fatal("reduction leaked reference slots", got, before)
					}
					for i, x := range variables {
						count, err := m.GetRefCount(x)
						closureMust(t, err)
						if count != counts[i] {
							t.Fatal("unpublished definition leaked a variable reference", i, count, counts[i])
						}
					}
					retry, err := c.reduceDisequalities(md, 3, formula)
					closureMust(t, err)
					root, err := m.RefGet(retry)
					closureMust(t, err)
					size, err := m.GetNumArgs(root)
					closureMust(t, err)
					if int(size) != len(variables) {
						t.Fatal("retry lost defining equalities", size, len(variables))
					}
					closureMust(t, m.DelRef(&retry))
				})
				if t.Failed() {
					break
				}
			}
			if !succeeded || failures == 0 {
				t.Fatal("missing disequality failure/success frontier", failures, succeeded)
			}
		})
	}
}
