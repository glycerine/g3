package qe

// Sources: src/muz/spacer/spacer_legacy_mbp.cpp;
// src/muz/spacer/spacer_util.cpp qe_project legacy path.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyProjectionCombinesBooleanArrayAndArithmetic(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	fmMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	b := f.constant("legacy-combined-bool", f.boolean)
	x := f.constant("legacy-combined-int", f.integer)
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("legacy-combined-array-a", arraySort)
	c := f.constant("legacy-combined-array-c", arraySort)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	three := f.own(m.ArithmeticIntegerNumeralText("3"))
	truth := f.own(m.MkTrue())
	base := f.own(m.MkConstArray(arraySort, zero))
	arrayEquality := f.own(m.MkEq(a, c))
	lower := f.own(m.MkArithmeticGE(x, zero))
	upper := f.own(m.MkArithmeticLE(x, three))
	formula := f.own(m.MkAndArray([]ast.ASTHandle{b, arrayEquality, lower, upper}))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{b: truth, x: one, a: base, c: base} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, err := p.Project(md.Core(), []ast.ASTHandle{b, a, x}, formula, false, true)
	fmMust(t, err)
	if mode, err := m.ProofMode(); err != nil || mode != ast.ASTProofEnabled {
		t.Fatal("combined projection changed proof mode", mode, err)
	}
	if _, err := p.projectArithmeticRound(nil, []ast.ASTHandle{x}, formula); err == nil {
		t.Fatal("arithmetic projection accepted a missing model")
	}
	if mode, err := m.ProofMode(); err != nil || mode != ast.ASTProofEnabled {
		t.Fatal("failed arithmetic projection changed proof mode", mode, err)
	}
	defer func() { fmMust(t, result.Close()) }()
	remaining, err := result.Variables()
	fmMust(t, err)
	if len(remaining) != 0 {
		t.Fatal("combined legacy projection retained variables", len(remaining))
	}
	handle, err := result.Formula()
	fmMust(t, err)
	for _, variable := range []ast.ASTHandle{b, a, x} {
		contains, err := ast.NewContainsApp(m, variable)
		fmMust(t, err)
		found, err := contains.Check(handle)
		fmMust(t, err)
		fmMust(t, contains.Close())
		if found {
			t.Fatal("combined legacy projection retains input variable")
		}
	}
}

func TestLegacyProjectionRunsLiteBeforeTheoryDispatch(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	sortName, err := m.Symbols.Intern("legacy-lite-value")
	fmMust(t, err)
	valueSort := f.own(m.MkUninterpretedSort(sortName, nil))
	x := f.constant("legacy-lite-x", valueSort)
	value := f.constant("legacy-lite-constant", valueSort)
	predicate := f.predicate("legacy-lite-predicate", x)
	formula := f.own(m.MkAndArray([]ast.ASTHandle{f.own(m.MkEq(x, value)), predicate}))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	result, err := p.Project(md.Core(), []ast.ASTHandle{x}, formula, false, true)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	remaining, err := result.Variables()
	fmMust(t, err)
	if len(remaining) != 0 {
		t.Fatal("qe_lite retained an equality-defined variable")
	}
	got, err := result.Formula()
	fmMust(t, err)
	want := f.predicate("legacy-lite-predicate", value)
	if got != want {
		t.Fatal("qe_lite preprocessing result", got, want)
	}
}

// Source: spacer_util.cpp qe_project repeats qe_lite on array auxiliaries.
func TestLegacyProjectionEliminatesBooleanArrayAuxiliarySymbolically(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	fmMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.boolean))
	a := f.constant("round-array", sort)
	b := f.constant("round-bool", f.boolean)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	falsity := f.own(m.MkFalse())
	arrays, err := m.ArrayFamilyID()
	fmMust(t, err)
	selectTerm := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, zero}))
	formula := f.own(m.MkEq(selectTerm, b))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	base := f.own(m.MkConstArray(sort, falsity))
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: base, b: falsity} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, err := p.Project(md.Core(), []ast.ASTHandle{a}, formula, false, true)
	fmMust(t, err)
	if mode, err := m.ProofMode(); err != nil || mode != ast.ASTProofEnabled {
		t.Fatal("array projection did not restore proof mode", mode, err)
	}
	if _, err := p.projectArrayRound(nil, nil, formula, false); err == nil {
		t.Fatal("array projection accepted a missing model")
	}
	if mode, err := m.ProofMode(); err != nil || mode != ast.ASTProofEnabled {
		t.Fatal("failed array projection did not restore proof mode", mode, err)
	}
	defer func() { fmMust(t, result.Close()) }()
	root, err := result.Formula()
	fmMust(t, err)
	truth := m.IsTrue(root)
	if !truth {
		t.Fatal("array auxiliary must be eliminated by equality before model substitution", root)
	}
	remaining, err := result.Variables()
	fmMust(t, err)
	if len(remaining) != 0 {
		t.Fatal("array auxiliary remained", remaining)
	}
}

// Source: spacer_util.cpp calls array_project even with no array variables;
// spacer_qe_project.cpp array_select_reducer honors reduce_all_selects.
func TestLegacyProjectionReducesAllSelectsWithoutArrayVariables(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("reduce-all-driver-a", sort)
	i := f.constant("reduce-all-driver-i", f.integer)
	j := f.constant("reduce-all-driver-j", f.integer)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	arrays, err := m.ArrayFamilyID()
	fmMust(t, err)
	stored := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, seven}))
	selected := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{stored, j}))
	formula := f.own(m.MkEq(selected, seven))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	base := f.own(m.MkConstArray(sort, zero))
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: base, i: one, j: one} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	for _, reduceAll := range []bool{false, true} {
		result, err := p.Project(md.Core(), nil, formula, reduceAll, true)
		fmMust(t, err)
		root, err := result.Formula()
		fmMust(t, err)
		occurs, err := m.OccursExpr(stored, root)
		fmMust(t, err)
		if occurs == reduceAll {
			t.Fatal("reduce-all dispatch with no projected variables", reduceAll, occurs, root)
		}
		if reduceAll {
			// Equal model indices select the stored value, but the projected
			// formula must keep the equality guard instead of returning true.
			for _, equalIndices := range []bool{true, false} {
				value := zero
				if equalIndices {
					value = one
				}
				decl, err := m.GetDecl(j)
				fmMust(t, err)
				fmMust(t, md.Core().RegisterConstant(decl, value))
				evaluator, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation, Trace: f.r.config.Trace})
				fmMust(t, err)
				evaluated, err := evaluator.EvaluateValue(root)
				fmMust(t, err)
				handle, err := m.RefGet(evaluated)
				fmMust(t, err)
				if (equalIndices && !m.IsTrue(handle)) || (!equalIndices && !m.IsFalse(handle)) {
					t.Fatal("model-selected store reduction lost its index guard", equalIndices, handle)
				}
				fmMust(t, m.DelRef(&evaluated))
				fmMust(t, evaluator.Close())
			}
		}

		remaining, err := result.Variables()
		fmMust(t, err)
		if len(remaining) != 0 {
			t.Fatal("unexpected projected variables", remaining)
		}
		fmMust(t, result.Close())
	}
}

func TestLegacyProjectionFlattensInputInSourceOrder(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	truth := f.own(m.MkTrue())
	a := f.constant("projection-flat-a", f.boolean)
	b := f.constant("projection-flat-b", f.boolean)
	c := f.constant("projection-flat-c", f.boolean)
	for _, h := range []ast.ASTHandle{a, b, c} {
		decl, err := m.GetDecl(h)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, truth))
	}
	flat := f.own(m.MkAndArray([]ast.ASTHandle{a, b, c}))
	inner := f.own(m.MkAnd(b, c))
	nested := f.own(m.MkAnd(a, inner))
	expected := f.own(m.MkAndArray([]ast.ASTHandle{c, a, b}))
	for _, native := range []bool{false, true} {
		for _, formula := range []ast.ASTHandle{flat, nested} {
			var result *LegacyProjectionResult
			if native {
				result, err = p.ProjectNativeArithmetic(md.Core(), nil, formula, false, true)
			} else {
				result, err = p.Project(md.Core(), nil, formula, false, true)
			}
			fmMust(t, err)
			actual, err := result.Formula()
			fmMust(t, err)
			if actual != expected {
				t.Fatal("initial flatten_and must preserve source swap/reprocessing order", native, formula, actual, expected)
			}
			fmMust(t, result.Close())
		}
	}
}
