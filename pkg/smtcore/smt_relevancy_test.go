package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtRelevancyBool(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	value, err := m.MkConst(astTestSymbol(t, m, name), boolean)
	astTestMust(t, err)
	return value
}

func TestSMTRelevancyNegatedArgumentsAndLiteralWatches(t *testing.T) {
	for _, conjunction := range []bool{false, true} {
		for _, assignedFirst := range []bool{false, true} {
			t.Run(fmt.Sprint(conjunction, "/", assignedFirst), func(t *testing.T) {
				m := astTestManager(t)
				a := smtRelevancyBool(t, m, "negated-watch-a")
				b := smtRelevancyBool(t, m, "negated-watch-b")
				notA, err := m.MkNot(a)
				astTestMust(t, err)
				parent, err := m.MkOr(notA, b)
				if conjunction {
					parent, err = m.MkAnd(notA, b)
				}
				astTestMust(t, err)
				context, err := NewSMTCheckerContext(m)
				astTestMust(t, err)
				astTestMust(t, context.SetAssignment(parent, lbool.FromBool(!conjunction)))
				propagator, err := NewSMTRelevancyPropagator(context, true)
				astTestMust(t, err)
				if assignedFirst {
					astTestMust(t, context.SetAssignment(a, lbool.FromBool(conjunction)))
				}
				if conjunction {
					astTestMust(t, propagator.AddAndWatches(parent))
				} else {
					astTestMust(t, propagator.AddOrWatches(parent))
				}
				astTestMust(t, propagator.MarkRelevant(parent))
				astTestMust(t, propagator.Propagate())
				if !assignedFirst {
					if propagator.IsRelevant(a) {
						t.Fatal("undefined branch became relevant")
					}
					astTestMust(t, context.SetAssignment(a, lbool.FromBool(conjunction)))
					astTestMust(t, propagator.AssignmentEvent(a, conjunction))
					astTestMust(t, propagator.Propagate())
				}
				if !propagator.IsRelevant(notA) || !propagator.IsRelevant(a) || propagator.IsRelevant(b) {
					t.Fatal("signed child selection lost the underlying atom")
				}
				falseTerm, err := m.MkFalse()
				astTestMust(t, err)
				if propagator.assignment(falseTerm) != lbool.False {
					t.Fatal("source false constant assignment")
				}
			})
		}
	}
}

func TestSMTRelevancySourceHeadInsertionOrder(t *testing.T) {
	m := astTestManager(t)
	source := smtRelevancyBool(t, m, "head-order-source")
	older := smtRelevancyBool(t, m, "head-order-older")
	newer := smtRelevancyBool(t, m, "head-order-newer")
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	propagator, err := NewSMTRelevancyPropagator(context, true)
	astTestMust(t, err)
	// C++ list<T> constructs each new node at the head, so the most recently
	// registered handler runs first when the source expression is propagated.
	astTestMust(t, propagator.AddDependency(source, older))
	astTestMust(t, propagator.AddDependency(source, newer))
	astTestMust(t, propagator.MarkRelevant(source))
	astTestMust(t, propagator.Propagate())
	got := propagator.RelevantExpressions()
	want := []ast.ASTHandle{source, newer, older}
	if len(got) != len(want) {
		t.Fatalf("source handler order length=%d want=%d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("source handler order[%d]=%v want %v (all %v)", i, got[i], want[i], got)
		}
	}
}

func TestSMTRelevancyBooleanConnectivePropagation(t *testing.T) {
	for _, test := range []struct {
		name        string
		make        func(*ast.ASTManager, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) (ast.ASTHandle, error)
		assignments []lbool.LBool
		want        []int
	}{
		{"or-true", func(m *ast.ASTManager, a, b, _ ast.ASTHandle) (ast.ASTHandle, error) { return m.MkOr(a, b) }, []lbool.LBool{lbool.True, lbool.True, lbool.False}, []int{0}},
		{"and-false", func(m *ast.ASTManager, a, b, _ ast.ASTHandle) (ast.ASTHandle, error) { return m.MkAnd(a, b) }, []lbool.LBool{lbool.False, lbool.False, lbool.True}, []int{0}},
		{"ite-true", func(m *ast.ASTManager, a, b, c ast.ASTHandle) (ast.ASTHandle, error) { return m.MkIte(a, b, c) }, []lbool.LBool{lbool.True, lbool.True, lbool.False}, []int{0, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := astTestManager(t)
			children := []ast.ASTHandle{smtRelevancyBool(t, m, test.name+"-a"), smtRelevancyBool(t, m, test.name+"-b"), smtRelevancyBool(t, m, test.name+"-c")}
			parent, err := test.make(m, children[0], children[1], children[2])
			astTestMust(t, err)
			context, err := NewSMTCheckerContext(m)
			astTestMust(t, err)
			for index, child := range children {
				astTestMust(t, context.SetAssignment(child, test.assignments[index]))
			}
			if test.name == "or-true" {
				astTestMust(t, context.SetAssignment(parent, lbool.True))
			} else if test.name == "and-false" {
				astTestMust(t, context.SetAssignment(parent, lbool.False))
			}
			propagator, err := NewSMTRelevancyPropagator(context, true)
			astTestMust(t, err)
			astTestMust(t, propagator.MarkRelevant(parent))
			astTestMust(t, propagator.Propagate())
			for index, child := range children {
				wanted := false
				for _, selected := range test.want {
					wanted = wanted || selected == index
				}
				if propagator.IsRelevant(child) != wanted {
					t.Fatal("child relevance", index, propagator.IsRelevant(child), wanted)
				}
			}
		})
	}
}

func TestSMTRelevancySharedAssignmentWatches(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "rel-watch-a")
	b := smtRelevancyBool(t, m, "rel-watch-b")
	c := smtRelevancyBool(t, m, "rel-watch-c")

	t.Run("or", func(t *testing.T) {
		parent, err := m.MkOr(a, b)
		astTestMust(t, err)
		context, err := NewSMTCheckerContext(m)
		astTestMust(t, err)
		astTestMust(t, context.SetAssignment(parent, lbool.True))
		propagator, err := NewSMTRelevancyPropagator(context, true)
		astTestMust(t, err)
		astTestMust(t, propagator.AddOrWatches(parent))
		astTestMust(t, propagator.MarkRelevant(parent))
		astTestMust(t, propagator.Propagate())
		if propagator.IsRelevant(a) || propagator.IsRelevant(b) {
			t.Fatal("unassigned OR selected a child")
		}
		astTestMust(t, context.SetAssignment(b, lbool.True))
		astTestMust(t, propagator.AssignmentEvent(b, true))
		astTestMust(t, propagator.Propagate())
		if !propagator.IsRelevant(b) {
			t.Fatal("true OR watch did not select child")
		}
		astTestMust(t, propagator.Check([]ast.ASTHandle{parent, a, b}))
	})

	t.Run("and", func(t *testing.T) {
		parent, err := m.MkAnd(a, b)
		astTestMust(t, err)
		context, err := NewSMTCheckerContext(m)
		astTestMust(t, err)
		astTestMust(t, context.SetAssignment(parent, lbool.False))
		propagator, err := NewSMTRelevancyPropagator(context, true)
		astTestMust(t, err)
		astTestMust(t, propagator.AddAndWatches(parent))
		astTestMust(t, propagator.MarkRelevant(parent))
		astTestMust(t, propagator.Propagate())
		astTestMust(t, context.SetAssignment(a, lbool.False))
		astTestMust(t, propagator.AssignmentEvent(a, false))
		astTestMust(t, propagator.Propagate())
		if !propagator.IsRelevant(a) {
			t.Fatal("false AND watch did not select child")
		}
		astTestMust(t, propagator.Check([]ast.ASTHandle{parent, a, b}))
	})

	t.Run("ite", func(t *testing.T) {
		parent, err := m.MkIte(c, a, b)
		astTestMust(t, err)
		context, err := NewSMTCheckerContext(m)
		astTestMust(t, err)
		propagator, err := NewSMTRelevancyPropagator(context, true)
		astTestMust(t, err)
		astTestMust(t, propagator.AddITEWatches(parent))
		astTestMust(t, propagator.MarkRelevant(parent))
		astTestMust(t, propagator.Propagate())
		if !propagator.IsRelevant(c) || propagator.IsRelevant(a) || propagator.IsRelevant(b) {
			t.Fatal("unassigned ITE relevance")
		}
		astTestMust(t, context.SetAssignment(c, lbool.False))
		astTestMust(t, propagator.AssignmentEvent(c, false))
		astTestMust(t, propagator.Propagate())
		if propagator.IsRelevant(a) || !propagator.IsRelevant(b) {
			t.Fatal("false ITE watch did not select else branch")
		}
		astTestMust(t, propagator.Check([]ast.ASTHandle{parent, a, b, c}))
	})
}

func TestSMTRelevancyDependenciesWatchesScopesAndEqualityClasses(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "rel-dep-a")
	b := smtRelevancyBool(t, m, "rel-dep-b")
	target := smtRelevancyBool(t, m, "rel-dep-target")
	pairTarget := smtRelevancyBool(t, m, "rel-pair-target")
	condition := smtRelevancyBool(t, m, "rel-watch-condition")
	watchTarget := smtRelevancyBool(t, m, "rel-watch-target")
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	propagator, err := NewSMTRelevancyPropagator(context, true)
	astTestMust(t, err)
	astTestMust(t, propagator.PushScope())
	astTestMust(t, propagator.AddDependency(a, target))
	astTestMust(t, propagator.AddPairDependency(a, b, pairTarget))
	astTestMust(t, propagator.AddWatchTarget(condition, true, watchTarget))
	astTestMust(t, propagator.MarkRelevant(a))
	astTestMust(t, propagator.Propagate())
	if !propagator.IsRelevant(target) || propagator.IsRelevant(pairTarget) {
		t.Fatal("simple or premature pair dependency")
	}
	astTestMust(t, propagator.MarkRelevant(b))
	astTestMust(t, propagator.Propagate())
	if !propagator.IsRelevant(pairTarget) {
		t.Fatal("pair dependency")
	}
	astTestMust(t, context.SetAssignment(condition, lbool.True))
	astTestMust(t, propagator.AssignmentEvent(condition, true))
	if !propagator.IsRelevant(watchTarget) {
		t.Fatal("positive assignment watch")
	}
	astTestMust(t, propagator.PopScopes(1))
	for _, expression := range []ast.ASTHandle{a, b, target, pairTarget, watchTarget} {
		if propagator.IsRelevant(expression) {
			t.Fatal("scope did not remove relevance", expression)
		}
	}
	astTestMust(t, propagator.MarkRelevant(a))
	astTestMust(t, propagator.Propagate())
	if propagator.IsRelevant(target) || propagator.IsRelevant(pairTarget) {
		t.Fatal("scope did not remove dependency handlers")
	}

	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	_, first := smtEnodeConstant(t, m, 2130, boolean, 0)
	_, second := smtEnodeConstant(t, m, 2131, boolean, 0)
	first.root, first.next, first.classSize = first, second, 2
	second.root, second.next = first, first
	astTestMust(t, context.RegisterEnode(first, false))
	astTestMust(t, context.RegisterEnode(second, false))
	astTestMust(t, propagator.MarkRelevant(first.owner))
	if !propagator.IsRelevant(first.owner) || !propagator.IsRelevant(second.owner) {
		t.Fatal("equality-class relevance")
	}
}

func TestSMTRelevancyDisabledAndTermITEHandler(t *testing.T) {
	m := astTestManager(t)
	condition := smtRelevancyBool(t, m, "rel-term-condition")
	sort, err := m.MkSort(astTestSymbol(t, m, "RelTermSort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	thenTerm, err := m.MkConst(astTestSymbol(t, m, "rel-then"), sort)
	astTestMust(t, err)
	elseTerm, err := m.MkConst(astTestSymbol(t, m, "rel-else"), sort)
	astTestMust(t, err)
	termITE, err := m.MkIte(condition, thenTerm, elseTerm)
	astTestMust(t, err)
	thenEquality, err := m.MkEq(termITE, thenTerm)
	astTestMust(t, err)
	elseEquality, err := m.MkEq(termITE, elseTerm)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	propagator, err := NewSMTRelevancyPropagator(context, true)
	astTestMust(t, err)
	astTestMust(t, propagator.AddTermITEWatches(termITE, thenEquality, elseEquality))
	astTestMust(t, propagator.MarkRelevant(termITE))
	astTestMust(t, propagator.Propagate())
	if !propagator.IsRelevant(condition) || propagator.IsRelevant(thenEquality) || propagator.IsRelevant(elseEquality) {
		t.Fatal("unassigned term ITE relevance")
	}
	astTestMust(t, context.SetAssignment(condition, lbool.False))
	astTestMust(t, propagator.AssignmentEvent(condition, false))
	astTestMust(t, propagator.Propagate())
	if !propagator.IsRelevant(condition) || propagator.IsRelevant(thenEquality) || !propagator.IsRelevant(elseEquality) {
		t.Fatal("term ITE relevance handler")
	}
	if got := propagator.RelevantExpressions(); len(got) != 4 || got[0] != termITE || got[1] != condition || got[2] != elseEquality || got[3] != elseTerm {
		t.Fatal("relevant expression order", got)
	}
	astTestMust(t, propagator.Check([]ast.ASTHandle{termITE, condition, thenEquality, elseEquality}))
	disabled, err := NewSMTRelevancyPropagator(context, false)
	astTestMust(t, err)
	if !disabled.IsRelevant(ast.ASTHandle{}) {
		t.Fatal("disabled source relevancy treats every expression as relevant")
	}
}
