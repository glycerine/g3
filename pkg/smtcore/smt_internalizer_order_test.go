package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTInternalizationOrderBooleanGateContextsAndNotElision(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "order-a")
	b := smtRelevancyBool(t, m, "order-b")
	c := smtRelevancyBool(t, m, "order-c")
	and, err := m.MkAnd(a, b)
	astTestMust(t, err)
	not, err := m.MkNot(c)
	astTestMust(t, err)
	root, err := m.MkOr(and, not)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	order, err := NewSMTInternalizationOrder(m, context)
	astTestMust(t, err)
	got, err := order.Sort(root)
	astTestMust(t, err)
	want := []ast.ASTHandle{a, b, and, c}
	if len(got) != len(want) {
		t.Fatal("order length", got)
	}
	for index := range want {
		if got[index].Expression != want[index] || !got[index].GateContext {
			t.Fatal("Boolean topological order", index, got[index], want[index])
		}
	}
}

func TestSMTInternalizationOrderTermITEContextsAndInternalizedCutoff(t *testing.T) {
	m := astTestManager(t)
	condition := smtRelevancyBool(t, m, "order-condition")
	sort, err := m.MkSort(astTestSymbol(t, m, "OrderSort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	thenTerm, thenNode := smtEnodeConstant(t, m, 2220, sort, 0)
	elseTerm, _ := smtEnodeConstant(t, m, 2221, sort, 0)
	root, err := m.MkIte(condition, thenTerm, elseTerm)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	astTestMust(t, context.RegisterEnode(thenNode, false))
	order, err := NewSMTInternalizationOrder(m, context)
	astTestMust(t, err)
	got, err := order.Sort(root)
	astTestMust(t, err)
	if len(got) != 3 || got[0].Expression != elseTerm || got[0].GateContext || got[1].Expression != thenTerm || got[1].GateContext || got[2].Expression != condition || !got[2].GateContext {
		t.Fatal("term ITE source order and contexts", got)
	}
}

// smtcore_03-F2: m_manager.is_term_ite(n) requires the basic family, so a
// theory operator whose kind equals OP_ITE (bvadd is kind 4) is visited as
// an ordinary application.
func TestSMTInternalizationOrderTheoryOperatorIsNotTermITE(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	family, err := m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	sort, err := m.BitVectorSort(8)
	astTestMust(t, err)
	var consts []ast.ASTHandle
	for _, name := range []string{"order-bv-x", "order-bv-y", "order-bv-z"} {
		c, err := m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		consts = append(consts, c)
	}
	x, y, z := consts[0], consts[1], consts[2]
	add, err := m.MkFamilyApp(family, 4, nil, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	if kind, err := m.GetDeclKind(add); err != nil || kind != ast.ASTOpIte {
		t.Fatal("bvadd kind no longer collides with OP_ITE", kind, err)
	}
	root, err := m.MkEq(add, z)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	order, err := NewSMTInternalizationOrder(m, context)
	astTestMust(t, err)
	got, err := order.Sort(root)
	astTestMust(t, err)
	want := []ast.ASTHandle{x, y, add, z}
	if len(got) != len(want) {
		t.Fatal("bvadd top-sort order", got)
	}
	for i := range want {
		if got[i].Expression != want[i] || got[i].GateContext {
			t.Fatal("bvadd top-sort order and contexts", got)
		}
	}
}

func TestSMTInternalizationOrderScopedBooleanCutoff(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "a")
	b := smtRelevancyBool(t, m, "b")
	shared, err := m.MkOr(a, b)
	astTestMust(t, err)
	root, err := m.MkIff(shared, a)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(root))
	t.Cleanup(func() { astTestMust(t, m.DecRef(root)) })
	ctx, _, state := smtInternalizerFixture(t, m, nil)
	live, err := NewSMTInternalizationOrder(m, ctx)
	astTestMust(t, err)
	live.variables = state.boolVar

	check := func(wantChild bool) {
		t.Helper()
		items, err := live.Sort(root)
		astTestMust(t, err)
		for _, item := range items {
			if item.Expression == b {
				if !wantChild {
					t.Fatal("visited child of an internalized Boolean")
				}
				return
			}
		}
		if wantChild {
			t.Fatal("skipped child of a Boolean without a live variable")
		}
	}
	check(true)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(shared, true))
	check(false)
	astTestMust(t, state.PopScopes(1))
	// A different expression can now acquire the same variable number.
	astTestMust(t, state.Internalize(smtRelevancyBool(t, m, "other"), true))
	check(true)
}
