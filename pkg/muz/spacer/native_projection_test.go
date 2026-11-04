package spacer

// Source: src/muz/spacer/spacer_util.cpp qe_project native arithmetic branch.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestSpacerNativeProjectionUsesSharedStages(t *testing.T) {
	m, globals := newSpacerTestServices(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(globals.SourceSetName("fixedpoint.spacer.native_mbp", "true"))
	owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	must(err)
	defer func() { must(owner.Close()) }()
	var pins []ast.ASTRef
	defer func() { must(releaseNormalizeRefs(m, pins)) }()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		r, err := m.NewRef(h)
		must(err)
		pins = append(pins, r)
		return h
	}
	boolean := own(m.MkBoolSort())
	symbol, err := m.Symbols.Intern("native-projection-p")
	must(err)
	predicate := own(m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{}))
	pt, err := NewRulePredicateTransformer(owner, predicate, PropSolverConfig{})
	must(err)
	defer func() { must(pt.Close()) }()
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	must(rules.RegisterPredicate(predicate))
	head := own(m.MkApp(predicate, nil))
	rule, err := rules.NewRule(head, nil, nil, "native-projection")
	must(err)
	defer func() { must(rule.Close()) }()
	truth := own(m.MkTrue())
	root, err := pt.NewProofObligation(nil, 2, 1, truth, nil)
	must(err)
	d, err := NewDerivation(root, rule, truth, nil)
	must(err)
	defer func() { must(d.Close()) }()
	md, err := model.NewModel(m)
	must(err)
	defer func() { must(md.Close()) }()
	zero := own(m.ArithmeticIntegerNumeralText("0"))
	one := own(m.ArithmeticIntegerNumeralText("1"))
	integer, err := m.GetSort(zero)
	must(err)
	constant := func(text string, sort ast.ASTHandle, value ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		symbol, err := m.Symbols.Intern(text)
		must(err)
		h := own(m.MkConst(symbol, sort))
		decl, err := m.GetDecl(h)
		must(err)
		must(md.Core().RegisterConstant(decl, value))
		return h
	}
	arraySort := own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), integer))
	baseArray := own(m.MkConstArray(arraySort, zero))
	a := constant("native-projection-a", arraySort, baseArray)
	i := constant("native-projection-i", integer, zero)
	j := constant("native-projection-j", integer, zero)
	arrays, err := m.ArrayFamilyID()
	must(err)
	stored := own(m.MkFamilyApp(arrays, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, one}))
	selected := own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{stored, j}))
	formula := own(m.MkEq(selected, one))
	t.Run("array-reduction-with-no-variables", func(t *testing.T) {
		result, remaining, err := d.project(false, true, md.Core(), nil, formula)
		must(err)
		defer func() { must(m.DelRef(&result)); must(releaseNormalizeRefs(m, remaining)) }()
		h, err := m.RefGet(result)
		must(err)
		occurs, err := m.OccursExpr(stored, h)
		must(err)
		if occurs {
			t.Fatal("native arithmetic mode skipped the shared array reduction stage")
		}
	})
	t.Run("child-skips-empty-projection", func(t *testing.T) {
		// create_next_child guards both projection calls with !vars.empty().
		// The unguarded driver above reduces this select even without variables.
		result, remaining, err := d.projectIfNonempty(false, md.Core(), nil, formula)
		must(err)
		defer func() { must(m.DelRef(&result)); must(releaseNormalizeRefs(m, remaining)) }()
		h, err := m.RefGet(result)
		must(err)
		if h != formula || len(remaining) != 0 {
			t.Fatal("empty child projection changed its formula or variables", h, remaining)
		}
	})
	t.Run("array-reduction-unequal-indices", func(t *testing.T) {
		// array_select_reducer takes the model's unequal-index branch and
		// retains its guard, even when neither array is projected.
		baseOnes := own(m.MkConstArray(arraySort, one))
		otherArray := constant("native-projection-other-array", arraySort, baseOnes)
		otherIndex := constant("native-projection-other-index", integer, one)
		otherStore := own(m.MkFamilyApp(arrays, ast.ASTArrayOpStore, nil, []ast.ASTHandle{otherArray, i, one}))
		otherSelect := own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{otherStore, otherIndex}))
		input := own(m.MkEq(otherSelect, one))
		result, remaining, err := d.project(false, true, md.Core(), nil, input)
		must(err)
		defer func() { must(m.DelRef(&result)); must(releaseNormalizeRefs(m, remaining)) }()
		h, err := m.RefGet(result)
		must(err)
		occurs, err := m.OccursExpr(otherStore, h)
		must(err)
		if occurs || len(remaining) != 0 {
			t.Fatal("array reduction retained store or projection variables", occurs, remaining)
		}
		decl, err := m.GetDecl(otherIndex)
		must(err)
		config := owner.contexts[0].config.ModelEvaluator
		for _, equal := range []bool{false, true} {
			value := one
			if equal {
				value = zero
			}
			must(md.Core().RegisterConstant(decl, value))
			evaluator, err := model.NewModelEvaluator(md.Core(), config)
			must(err)
			evaluated, err := evaluator.EvaluateValue(h)
			must(err)
			actual, err := m.RefGet(evaluated)
			must(err)
			if (!equal && !m.IsTrue(actual)) || (equal && !m.IsFalse(actual)) {
				t.Fatal("unequal-index projection lost its model-selected guard", equal, actual)
			}
			must(m.DelRef(&evaluated))
			must(evaluator.Close())
		}
	})
	t.Run("child-ground-controls-fallback-substitution", func(t *testing.T) {
		// create_next_child passes !ground as qe_project's dont_sub.
		owner.useNativeMBP = false
		defer func() { owner.useNativeMBP = true }()
		two := own(m.ArithmeticIntegerNumeralText("2"))
		x := constant("child-projection-x", integer, two)
		square := own(m.MkArithmeticMul(x, x))
		nonlinear := own(m.MkArithmeticGT(square, zero))
		for _, ground := range []bool{false, true} {
			result, remaining, err := d.projectIfNonempty(ground, md.Core(), []ast.ASTHandle{x}, nonlinear)
			must(err)
			h, err := m.RefGet(result)
			must(err)
			occurs, err := m.OccursExpr(x, h)
			must(err)
			if ground {
				if len(remaining) != 0 || occurs || !m.IsTrue(h) {
					t.Fatal("ground projection did not substitute the model value", remaining, occurs)
				}
			} else if len(remaining) != 1 || !occurs {
				t.Fatal("non-ground projection lost its existential", remaining, occurs)
			} else {
				v, err := m.RefGet(remaining[0])
				must(err)
				if v != x {
					t.Fatal("non-ground projection retained the wrong existential", v)
				}
			}
			must(m.DelRef(&result))
			must(releaseNormalizeRefs(m, remaining))
		}
	})
	t.Run("repeat-lite-for-boolean-array-auxiliary", func(t *testing.T) {
		falsity := own(m.MkFalse())
		boolArraySort := own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), boolean))
		boolBase := own(m.MkConstArray(boolArraySort, falsity))
		boolArray := constant("native-projection-bool-array", boolArraySort, boolBase)
		b := constant("native-projection-bool", boolean, falsity)
		selectBool := own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{boolArray, zero}))
		formula := own(m.MkEq(selectBool, b))
		result, remaining, err := d.project(false, true, md.Core(), []ast.ASTHandle{boolArray}, formula)
		must(err)
		defer func() { must(m.DelRef(&result)); must(releaseNormalizeRefs(m, remaining)) }()
		h, err := m.RefGet(result)
		must(err)
		if len(remaining) != 0 || !m.IsTrue(h) {
			t.Fatal("native mode failed to eliminate the fresh Boolean equality symbolically", len(remaining))
		}
	})
	t.Run("native-arithmetic-forces-elimination", func(t *testing.T) {
		two := own(m.ArithmeticIntegerNumeralText("2"))
		four := own(m.ArithmeticIntegerNumeralText("4"))
		x := constant("native-projection-x", integer, two)
		square := own(m.MkArithmeticMul(x, x))
		formula := own(m.MkEq(square, four))
		result, remaining, err := d.project(false, false, md.Core(), []ast.ASTHandle{x}, formula)
		must(err)
		defer func() { must(m.DelRef(&result)); must(releaseNormalizeRefs(m, remaining)) }()
		if len(remaining) != 0 {
			t.Fatal("native arithmetic must force elimination even with dont_sub enabled", len(remaining))
		}
		h, err := m.RefGet(result)
		must(err)
		occurs, err := m.OccursExpr(x, h)
		must(err)
		if occurs {
			t.Fatal("native arithmetic left the projected variable in the formula")
		}
	})
}
