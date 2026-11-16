package fp

// Source: rel_context.cpp product selection and query ownership;
// dl_context.cpp named-rule replacement and engine lifetime.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func boundIntervalLifecycleDeclaration(t *testing.T, f *boundIntervalPublicFixture, name string, domain []ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	symbol, err := f.m.Symbols.Intern("bound-interval-lifecycle-" + name)
	fragmentMust(t, err)
	declaration := f.hold(f.m.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(t, f.e.RegisterPredicate(declaration))
	return declaration
}

func TestDatalogBoundIntervalPublicNamedUpdateKeepsSeeds(t *testing.T) {
	skipUnreachableInfiniteSortRules(t)
	f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
	p, q, gate := f.declare("update-P", 1), f.declare("update-Q", 1), f.declare("update-Gate", 1)
	f.fact(p, 7)
	x := f.hold(f.m.MkVar(0, f.integer))
	px, qx, gx := f.app(p, x), f.app(q, x), f.app(gate, x)
	fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(px, qx)), "product-step"))
	status, err := f.e.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("initial product copy did not derive output", status)
	}
	answer := f.own(f.e.Answer(q))
	f.checkFormula(answer, []int{7}, true)
	owner, source := f.e.boundIntervalDatalog, f.e.set
	body := f.hold(f.m.MkAnd(px, gx))
	fragmentMust(t, f.e.UpdateRule(f.hold(f.m.MkImplies(body, qx)), "product-step"))
	if f.e.set == source || f.e.boundIntervalDatalog != owner || owner.rules != f.e.set || owner.evaluated != nil {
		t.Fatal("named replacement did not rebind and invalidate product evaluation")
	}
	if _, err := f.e.Answer(q); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("named replacement exposed a stale result", err)
	}
	status, err = f.e.QueryPredicate(q)
	fragmentMust(t, err)
	// C++ projection of the empty positive join drops m_empty.
	if status != lbool.Undef {
		t.Fatal("empty-source projection did not match C++ behavior", status)
	}
	f.checkFormula(f.own(f.e.Answer(q)), []int{7}, true)
	f.checkFormula(answer, []int{7}, true)
	fragmentMust(t, f.e.AddTableFact(gate, []uint32{7}))
	status, err = f.e.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("named replacement discarded original direct seeds", status)
	}
	current := f.own(f.e.Answer(q))
	f.checkFormula(current, []int{7}, true)
	f.checkFormula(current, []int{8}, false)
}
