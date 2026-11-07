package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtUFConstants(t *testing.T, manager *ast.ASTManager, prefix string, count int) (ast.ASTHandle, []ast.ASTHandle) {
	t.Helper()
	sort, err := manager.MkSort(astTestSymbol(t, manager, prefix+"-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	values := make([]ast.ASTHandle, count)
	for index := range values {
		values[index], err = manager.MkConst(astTestSymbol(t, manager, prefix+string(rune('a'+index))), sort)
		astTestMust(t, err)
	}
	return sort, values
}

func TestSMTBooleanClassAssignmentAfterMerge(t *testing.T) {
	for _, truth := range []bool{false, true} {
		for _, assignRoot := range []bool{false, true} {
			m := astTestManager(t)
			_, _, s := smtInternalizerFixture(t, m, nil)
			a := smtRelevancyBool(t, m, "late-class-a")
			b := smtRelevancyBool(t, m, "late-class-b")
			for _, term := range []ast.ASTHandle{a, b} {
				astTestMust(t, s.Internalize(term, true))
				v, _ := s.BoolVar(term)
				astTestMust(t, s.ensureBooleanEnode(term, v, true, false, false))
			}
			left, right := s.context.enodes.get(a), s.context.enodes.get(b)
			astTestMust(t, s.addEquality(left, right, NewSMTEqualityAxiom()))
			source := left
			if (source == source.root) != assignRoot {
				source = right
			}
			variable, _ := s.BoolVar(source.owner)
			for _, value := range []bool{truth, !truth} {
				astTestMust(t, s.PushScope())
				literal := NewSMTLiteral(variable, !value)
				astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
				consistent, err := s.PropagateCore()
				astTestMust(t, err)
				if !consistent {
					t.Fatal("consistent merged Boolean class conflicted")
				}
				for _, term := range []ast.ASTHandle{a, b} {
					v, _ := s.BoolVar(term)
					got, err := s.Assignment(NewSMTLiteral(v, !value))
					astTestMust(t, err)
					if got != lbool.True {
						t.Fatal("assignment did not reach class member", truth, assignRoot, got)
					}
					if s.context.enodes.get(term).mergeTruthValue {
						t.Fatal("test unexpectedly enabled merge_tf")
					}
				}
				astTestMust(t, s.PopScopes(1))
			}
		}
	}
}

func TestSMTCoreEqualityTransitivityAndScopeRollback(t *testing.T) {
	manager, solver := smtSolverFixture(t)
	_, values := smtUFConstants(t, manager, "core-eq-", 3)
	for _, value := range values {
		astTestMust(t, manager.IncRef(value))
	}
	t.Cleanup(func() {
		for _, value := range values {
			astTestMust(t, manager.DecRef(value))
		}
	})
	ab, err := manager.MkEq(values[0], values[1])
	astTestMust(t, err)
	bc, err := manager.MkEq(values[1], values[2])
	astTestMust(t, err)
	ac, err := manager.MkEq(values[0], values[2])
	astTestMust(t, err)
	notAC, err := manager.MkNot(ac)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(ab))
	astTestMust(t, solver.Push())
	astTestMust(t, solver.Assert(bc))
	astTestMust(t, solver.Assert(notAC))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("transitive equality contradiction", status)
	}
	astTestMust(t, solver.Pop(1))
	ac, err = manager.MkEq(values[0], values[2])
	astTestMust(t, err)
	notAC, err = manager.MkNot(ac)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(notAC))
	status, err = solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("popped equality merge was not restored", status)
	}
}

func TestSMTCoreEqualityCongruencePropagation(t *testing.T) {
	manager, solver := smtSolverFixture(t)
	sort, values := smtUFConstants(t, manager, "core-cg-", 2)
	function, err := manager.MkFuncDecl(astTestSymbol(t, manager, "core-cg-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	fa, err := manager.MkApp(function, []ast.ASTHandle{values[0]})
	astTestMust(t, err)
	fb, err := manager.MkApp(function, []ast.ASTHandle{values[1]})
	astTestMust(t, err)
	ab, err := manager.MkEq(values[0], values[1])
	astTestMust(t, err)
	fafb, err := manager.MkEq(fa, fb)
	astTestMust(t, err)
	notFAFB, err := manager.MkNot(fafb)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(ab))
	astTestMust(t, solver.Assert(notFAFB))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("congruence contradiction", status)
	}
}

func TestSMTCoreEqualityPropagatesBooleanCongruence(t *testing.T) {
	manager, solver := smtSolverFixture(t)
	sort, values := smtUFConstants(t, manager, "core-bool-cg-", 2)
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	predicate, err := manager.MkFuncDecl(astTestSymbol(t, manager, "core-bool-cg-p"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	pa, err := manager.MkApp(predicate, []ast.ASTHandle{values[0]})
	astTestMust(t, err)
	pb, err := manager.MkApp(predicate, []ast.ASTHandle{values[1]})
	astTestMust(t, err)
	ab, err := manager.MkEq(values[0], values[1])
	astTestMust(t, err)
	notPB, err := manager.MkNot(pb)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(ab))
	astTestMust(t, solver.Assert(pa))
	astTestMust(t, solver.Assert(notPB))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("Boolean congruence contradiction", status)
	}
}

func TestSMTCoreEqualityPropagatesRelevancyAndRollsBack(t *testing.T) {
	manager := astTestManager(t)
	_, values := smtUFConstants(t, manager, "core-relevant-eq-", 2)
	equality, err := manager.MkEq(values[0], values[1])
	astTestMust(t, err)
	context, relevancy, state := smtInternalizerFixture(t, manager, nil)
	astTestMust(t, state.Internalize(equality, false))
	left, right := context.enodes.get(values[0]), context.enodes.get(values[1])
	if left == nil || right == nil {
		t.Fatal("equality arguments were not internalized")
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, state.markRelevant(values[0]))
	astTestMust(t, relevancy.Propagate())
	variable, found := state.BoolVar(equality)
	if !found {
		t.Fatal("equality Boolean variable missing")
	}
	astTestMust(t, state.AssignLiteral(NewSMTLiteral(variable, false), NewSMTBooleanAxiom()))
	astTestMust(t, state.PropagateTheoryAssignments())
	astTestMust(t, relevancy.Propagate())
	if left.root != right.root || !relevancy.IsRelevant(values[1]) {
		t.Fatal("equality merge did not propagate relevancy")
	}
	astTestMust(t, state.PopScopes(1))
	if left.root == right.root || relevancy.IsRelevant(values[1]) {
		t.Fatal("scope pop did not restore equality class and relevancy")
	}
}
