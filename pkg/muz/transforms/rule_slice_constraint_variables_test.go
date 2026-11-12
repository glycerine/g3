package transforms_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRuleSliceDerivedSubstitutionOfConstraintOnlyVariable(t *testing.T) {
	for _, mixedSparse := range []bool{false, true} {
		name := "dense-integer"
		if mixedSparse {
			name = "sparse-mixed"
		}
		for _, reverse := range []bool{false, true} {
			for _, swapped := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/reverse=%t/swapped=%t", name, reverse, swapped), func(t *testing.T) { testRuleSliceConstraintVariables(t, mixedSparse, reverse, swapped) })
			}
		}
	}
}

func testRuleSliceConstraintVariables(t *testing.T, mixedSparse, reverse, swapped bool) {
	f := newConeFixture(t)
	m := f.manager
	zero, err := m.ArithmeticIntegerNumeralText("0")
	coneMust(t, err)
	integer, err := m.GetSort(zero)
	coneMust(t, err)
	x, err := m.MkVar(0, integer)
	coneMust(t, err)
	y, err := m.MkVar(1, integer)
	coneMust(t, err)
	guardZero := zero
	if swapped {
		guardZero, err = m.ArithmeticIntegerNumeralText("2")
		coneMust(t, err)
	}
	guardSort := integer
	if mixedSparse {
		value := "0"
		if swapped {
			value = "2"
		}
		guardZero, err = m.ArithmeticRealNumeralText(value)
		coneMust(t, err)
		guardSort, err = m.GetSort(guardZero)
		coneMust(t, err)
	}
	z, err := m.MkVar(2, guardSort)
	coneMust(t, err)
	p := declareTransformPredicate(t, f, "constraint-P", []ast.ASTHandle{integer, integer}, []ast.ASTHandle{x, y})
	q := declareTransformPredicate(t, f, "constraint-Q", []ast.ASTHandle{integer, integer}, []ast.ASTHandle{x, y})
	goal := declareTransformPredicate(t, f, "constraint-Goal", []ast.ASTHandle{integer}, []ast.ASTHandle{y})
	outputGuard, err := m.MkEq(y, zero)
	coneMust(t, err)
	guard, err := m.MkEq(z, guardZero)
	coneMust(t, err)
	sources := make(map[ast.ASTHandle]bool)
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
	}{
		{p.application, []ast.ASTHandle{guard}}, {q.application, []ast.ASTHandle{p.application}}, {goal.application, []ast.ASTHandle{q.application, outputGuard}},
	} {
		r, e := f.rules.NewRule(spec.head, spec.tails, nil, "")
		coneMust(t, e)
		coneMust(t, f.rules.MkRuleAssertedProof(r))
		original, found, e := r.Proof()
		coneMust(t, e)
		if !found {
			t.Fatal("missing original proof")
		}
		sources[original] = true
		coneMust(t, f.set.Add(r))
		coneMust(t, r.Close())
	}
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("invalid source")
	}
	plugin := transforms.NewRuleSlicePlugin(m, f.rules, 1)
	plugin.SetGenerateProofTrace(true)
	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(plugin))
	changed, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !changed {
		t.Fatal("no slicing")
	}
	sp, ok := plugin.DerivedPredicate(p.declaration)
	if !ok {
		t.Fatal("P not sliced")
	}
	sq, ok := plugin.DerivedPredicate(q.declaration)
	if !ok {
		t.Fatal("Q not sliced")
	}
	converter := plugin.TakeProofConverter()
	coneMust(t, transformer.Close())
	defer func() { coneMust(t, converter.Close()) }()
	var parents []ast.ASTHandle
	for _, d := range []ast.ASTHandle{sq, sp} {
		rs, e := f.set.PredicateRules(d)
		coneMust(t, e)
		if len(rs) != 1 {
			t.Fatal("rule count", len(rs))
		}
		proof, found, e := rs[0].Proof()
		coneMust(t, e)
		if !found {
			t.Fatal("missing proof")
		}
		parents = append(parents, proof)
		coneMust(t, rs[0].Close())
	}
	// The resolved clause has a variable appearing only in its arithmetic guard.
	selected := x
	constraintVariable, err := m.MkVar(1, guardSort)
	coneMust(t, err)
	sorts := []ast.ASTHandle{guardSort, integer}
	substitution := []ast.ASTHandle{zero, guardZero}
	if mixedSparse {
		boolean, e := m.MkBoolSort()
		coneMust(t, e)
		truth, e := m.MkTrue()
		coneMust(t, e)
		selected, err = m.MkVar(1, integer)
		coneMust(t, err)
		constraintVariable, err = m.MkVar(3, guardSort)
		coneMust(t, err)
		sorts = []ast.ASTHandle{guardSort, boolean, integer, boolean}
		substitution = []ast.ASTHandle{truth, zero, truth, guardZero}
	}
	selectedIndex, err := m.GetVarIndex(selected)
	coneMust(t, err)
	constraintIndex, err := m.GetVarIndex(constraintVariable)
	coneMust(t, err)
	if swapped {
		selectedIndex, constraintIndex = constraintIndex, selectedIndex
		selected, err = m.MkVar(selectedIndex, integer)
		coneMust(t, err)
		constraintVariable, err = m.MkVar(constraintIndex, guardSort)
		coneMust(t, err)
		sorts[len(sorts)-1-int(selectedIndex)] = integer
		sorts[len(sorts)-1-int(constraintIndex)] = guardSort
		substitution[selectedIndex] = zero
		substitution[constraintIndex] = guardZero
	}
	qx, err := m.MkApp(sq, []ast.ASTHandle{selected})
	coneMust(t, err)
	yz, err := m.MkEq(constraintVariable, guardZero)
	coneMust(t, err)
	body, err := m.MkImplies(yz, qx)
	coneMust(t, err)
	name, err := m.Symbols.Intern("constraint-variable")
	coneMust(t, err)
	names := make([]symbol.Symbol, len(sorts))
	for i := range names {
		names[i] = name
	}
	formula, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body})
	coneMust(t, err)
	scope, err := ast.NewScopedProof(m)
	coneMust(t, err)
	position := ast.ASTProofPosition{First: 1}
	premiseSubstitution := [][]ast.ASTHandle{{selected}, {selected, constraintVariable}}
	if reverse {
		// The bmc linear (0, 1) layout keeps the consumer first.
		position = ast.ASTProofPosition{Second: 1}
	}
	inner, err := m.MkProofHyperResolve(parents, formula, []ast.ASTProofPosition{position}, premiseSubstitution)
	coneMust(t, err)
	innerRef, err := m.NewRef(inner)
	coneMust(t, err)
	defer func() { coneMust(t, m.DelRef(&innerRef)) }()
	// Pass through another derived unary step before grounding. The sparse
	// bank contains unused Boolean binders between its integer and real slots.
	identity := append([]ast.ASTHandle(nil), substitution...)
	identity[selectedIndex] = selected
	identity[constraintIndex] = constraintVariable
	middle, err := m.MkProofHyperResolve([]ast.ASTHandle{inner}, formula, nil, [][]ast.ASTHandle{identity})
	coneMust(t, err)
	middleRef, err := m.NewRef(middle)
	coneMust(t, err)
	defer func() { coneMust(t, m.DelRef(&middleRef)) }()
	qzero, err := m.MkApp(sq, []ast.ASTHandle{zero})
	coneMust(t, err)
	zeroEq, err := m.MkEq(guardZero, guardZero)
	coneMust(t, err)
	conclusion, err := m.MkImplies(zeroEq, qzero)
	coneMust(t, err)
	outer, err := m.MkProofHyperResolve([]ast.ASTHandle{middle}, conclusion, nil, [][]ast.ASTHandle{substitution})
	coneMust(t, err)
	outerRef, err := m.NewRef(outer)
	coneMust(t, err)
	defer func() { coneMust(t, m.DelRef(&outerRef)) }()
	coneMust(t, scope.Close())
	converted, err := converter.Convert(outer)
	coneMust(t, err)
	defer func() { coneMust(t, m.DelRef(&converted)) }()
	handle, err := m.RefGet(converted)
	coneMust(t, err)
	fact, err := m.GetProofFact(handle)
	coneMust(t, err)
	body, err = m.GetQuantifierBody(fact)
	coneMust(t, err)
	expectedHead, err := m.MkApp(q.declaration, []ast.ASTHandle{x, zero})
	coneMust(t, err)
	expectedBody, err := m.MkImplies(zeroEq, expectedHead)
	coneMust(t, err)
	if body != expectedBody {
		t.Fatal("reconstructed conclusion lost original predicate or constraint substitution")
	}
	todo := []ast.ASTHandle{handle}
	seen := make(map[ast.ASTHandle]bool)
	leaves := 0
	for len(todo) > 0 {
		node := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[node] {
			continue
		}
		seen[node] = true
		var leaf ast.ASTHandle
		asserted, e := m.MatchProofAsserted(node, &leaf)
		coneMust(t, e)
		if asserted {
			leaves++
			if !sources[node] {
				t.Fatal("nonoriginal assertion")
			}
		}
		n, e := m.GetProofNumParents(node)
		coneMust(t, e)
		for i := uint32(0); i < n; i++ {
			parent, e := m.GetProofParent(node, i)
			coneMust(t, e)
			todo = append(todo, parent)
		}
	}
	if leaves != 2 {
		t.Fatal("original asserted leaf count", leaves)
	}
	checker, err := proofs.NewChecker(m)
	coneMust(t, err)
	checkedProofs := []ast.ASTHandle{outer, handle}
	if reverse {
		// The source (0, 1) input layout is not checker-valid; only the
		// reconstructed proof is.
		checkedProofs = checkedProofs[1:]
	}
	for _, checked := range checkedProofs {
		valid, conditions, err := checker.Check(checked)
		coneMust(t, err)
		defer func() {
			for _, c := range conditions {
				coneMust(t, m.DecRef(c))
			}
		}()
		if !valid {
			t.Fatal("invalid reconstructed proof")
		}
		solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: f.globals.Context, Globals: f.globals})
		coneMust(t, err)
		defer func() { coneMust(t, solver.Close()) }()
		for _, c := range conditions {
			coneMust(t, solver.Push())
			neg, e := m.MkNot(c)
			coneMust(t, e)
			coneMust(t, solver.AssertFormula(neg))
			status, e := solver.CheckSat(nil)
			coneMust(t, e)
			coneMust(t, solver.Pop(1))
			if status != lbool.False {
				t.Fatal("invalid proof condition", status)
			}
		}
	}
}
