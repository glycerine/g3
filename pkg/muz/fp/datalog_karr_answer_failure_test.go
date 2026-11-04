package fp

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestDatalogKarrAnswerAllocationFailureCanRetry(t *testing.T) {
	failed, completed := false, false
	for stop := int64(0); stop < 160 && !completed; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(e.SelectEngine("datalog"))
			must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
			must(g.SourceSetName("fixedpoint.xform.karr", "true"))
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("karr-answer-alloc")
			must(err)
			sorts := make([]ast.ASTHandle, 12)
			for i := range sorts {
				sorts[i] = integer
			}
			p, err := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(p))
			fact := make([]uint32, len(sorts))
			for i := range fact {
				fact[i] = uint32(17 + i)
			}
			must(e.AddTableFact(p, fact))
			_, err = e.QueryPredicate(p)
			must(err)
			// Retain a complete, distinct answer so every shared arithmetic
			// declaration, variable and zero node already has a stable owner.
			warm, err := e.karrDatalogAnswer(p)
			must(err)
			defer func() { must(m.DelRef(&warm)) }()
			fresh := rel.NewKarrEmpty(uint32(len(sorts)), nil)
			values := make([]*big.Rat, len(sorts))
			for i := range values {
				values[i] = big.NewRat(int64(1000+i), 1)
			}
			must(fresh.AddFact(values))
			e.karrDatalog.results[p] = fresh
			// Numeral declarations are manager-owned interned caches. Root the
			// new constants too, so a failed formula build cannot populate a
			// cache entry between the two ownership measurements.
			constantRoots := make([]ast.ASTRef, 0, len(values))
			defer func() {
				for i := range constantRoots {
					must(m.DelRef(&constantRoots[i]))
				}
			}()
			for _, value := range values {
				constant, err := m.ArithmeticNumeralRat(new(big.Rat).Neg(value), true)
				must(err)
				root, err := m.NewRef(constant)
				must(err)
				constantRoots = append(constantRoots, root)
			}
			before := m.Stats()
			m.Budget().FailAfter(stop)
			answer, cause := e.karrDatalogAnswer(p)
			m.Budget().FailAfter(-1)
			if cause != nil {
				failed = true
				if answer != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal(answer, cause)
				}
				var pending memory.ConstructionFailure
				if errors.As(cause, &pending) {
					must(pending.Cleanup())
				}
			} else {
				completed = true
				must(m.DelRef(&answer))
			}
			after := m.Stats()
			if cause != nil && (after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences) {
				t.Fatal("answer leaked on refusal", stop, before.LiveNodes, after.LiveNodes, before.LiveReferences, after.LiveReferences, cause)
			}
			answer, err = e.karrDatalogAnswer(p)
			must(err)
			must(m.DelRef(&answer))
		})
		if t.Failed() {
			break
		}
	}
	if !failed || !completed {
		t.Fatal("campaign did not cover refusal and success", failed, completed)
	}
}
