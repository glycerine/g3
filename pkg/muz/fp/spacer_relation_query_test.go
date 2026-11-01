package fp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSpacerRelationQueryReversesMixedSortHead(t *testing.T) {
	m, _ := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	zero, err := m.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	fragmentMust(t, err)
	integer, err := m.GetSort(zero)
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("spacer-mixed-query")
	fragmentMust(t, err)
	relation, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, e.RegisterPredicate(relation))
	query, rule, err := e.makeSpacerRelationQuery(relation, false)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, rule.Close()) }()
	for i, want := range []ast.ASTHandle{integer, boolean} {
		got, err := m.GetDomain(query, uint32(i))
		fragmentMust(t, err)
		if got != want {
			t.Fatalf("query head domain %d has incorrect sort", i)
		}
	}
	head, err := rule.Head()
	fragmentMust(t, err)
	tail, negated, err := rule.Tail(0)
	fragmentMust(t, err)
	if negated {
		t.Fatal("relation query tail is negated")
	}
	for i := uint32(0); i < 2; i++ {
		arg, err := m.GetArg(head, i)
		fragmentMust(t, err)
		index, err := m.GetVarIndex(arg)
		fragmentMust(t, err)
		if index != 1-i {
			t.Fatalf("head variable %d has index %d", i, index)
		}
		arg, err = m.GetArg(tail, i)
		fragmentMust(t, err)
		index, err = m.GetVarIndex(arg)
		fragmentMust(t, err)
		if index != i {
			t.Fatalf("tail variable %d has index %d", i, index)
		}
	}
}

func TestSpacerRelationQueryProofFollowsTraceSetting(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			mode, err := f.manager.ProofMode()
			fragmentMust(t, err)
			_, rule, err := f.engine.makeSpacerRelationQuery(f.declarations["p"], enabled)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, rule.Close()) }()
			proof, found, err := rule.Proof()
			fragmentMust(t, err)
			if found != enabled {
				t.Fatal("query-rule proof presence", found, enabled)
			}
			if enabled {
				formula, err := rule.ToFormula()
				fragmentMust(t, err)
				defer func() { fragmentMust(t, f.manager.DelRef(&formula)) }()
				want, err := f.manager.RefGet(formula)
				fragmentMust(t, err)
				var fact ast.ASTHandle
				asserted, err := f.manager.MatchProofAsserted(proof, &fact)
				fragmentMust(t, err)
				if !asserted || fact != want {
					t.Fatal("query proof does not assert its rule")
				}
			}
			after, err := f.manager.ProofMode()
			fragmentMust(t, err)
			if after != mode {
				t.Fatal("query rule changed the manager proof mode")
			}
		})
	}
}
