package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func newArrayBlastCoveragePlugin(t *testing.T, f *coneFixture) *transforms.ArrayBlastPlugin {
	t.Helper()
	plugin, err := transforms.NewArrayBlastPlugin(f.manager, f.rules, nil, f.globals.Context, f.globals, 35999)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	return plugin
}

func declareArrayBlastPredicate(t *testing.T, f *coneFixture, name string, domain []ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	symbol, err := f.manager.Symbols.Intern(name)
	coneMust(t, err)
	declaration, err := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(declaration))
	return declaration
}

func runArrayBlastCoverageNoChange(t *testing.T, f *coneFixture, head ast.ASTHandle, tails []ast.ASTHandle) {
	t.Helper()
	addTransformRule(t, f, head, tails, nil, "array-blast-refusal")
	goal, err := f.manager.GetDecl(head)
	coneMust(t, err)
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("array-blast refusal fixture is unstratified")
	}
	result, err := newArrayBlastCoveragePlugin(t, f).Transform(f.set)
	coneMust(t, err)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("refused array-blast input produced a transformed rule set")
	}
	if count, getErr := f.set.Len(); getErr != nil || count != 1 {
		t.Fatalf("refusal changed rule count: %d, %v", count, getErr)
	}
}

func TestArrayBlastNoStoreAndCyclicStoreRefusals(t *testing.T) {
	for _, test := range []struct {
		name        string
		cyclicStore bool
	}{
		{name: "no-store-definition"},
		{name: "cyclic-store-definition", cyclicStore: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newConeFixture(t)
			boolean, err := f.manager.MkBoolSort()
			coneMust(t, err)
			arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
			coneMust(t, err)
			predicate := declareArrayBlastPredicate(t, f, "array-blast-"+test.name, []ast.ASTHandle{arraySort})
			array, err := f.manager.MkVar(0, arraySort)
			coneMust(t, err)
			index, err := f.manager.MkVar(1, boolean)
			coneMust(t, err)
			value, err := f.manager.MkVar(2, boolean)
			coneMust(t, err)
			family, err := f.manager.ArrayFamilyID()
			coneMust(t, err)
			selectValue, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, index})
			coneMust(t, err)
			body, err := f.manager.MkEq(selectValue, value)
			coneMust(t, err)
			tails := []ast.ASTHandle{body}
			if test.cyclicStore {
				store, x := f.manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{array, index, value})
				coneMust(t, x)
				definition, x := f.manager.MkEq(array, store)
				coneMust(t, x)
				tails = append(tails, definition)
			}
			head, err := f.manager.MkApp(predicate, []ast.ASTHandle{array})
			coneMust(t, err)
			runArrayBlastCoverageNoChange(t, f, head, tails)
		})
	}
}

func TestArrayBlastStopsBeforeRulesWhenManagerIsCanceled(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	goalDecl := declareArrayBlastPredicate(t, f, "array-blast-cancel", nil)
	goal, err := f.manager.MkApp(goalDecl, nil)
	coneMust(t, err)
	array, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	baseArray, err := f.manager.MkVar(1, arraySort)
	coneMust(t, err)
	index, err := f.manager.MkVar(2, boolean)
	coneMust(t, err)
	value, err := f.manager.MkVar(3, boolean)
	coneMust(t, err)
	family, err := f.manager.ArrayFamilyID()
	coneMust(t, err)
	stored, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{baseArray, index, value})
	coneMust(t, err)
	definition, err := f.manager.MkEq(array, stored)
	coneMust(t, err)
	selected, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{baseArray, index})
	coneMust(t, err)
	observation, err := f.manager.MkEq(selected, value)
	coneMust(t, err)
	addTransformRule(t, f, goal, []ast.ASTHandle{definition, observation}, nil, "array-blast-cancel")
	plugin := newArrayBlastCoveragePlugin(t, f)
	f.manager.Limit.Cancel()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("canceled transform returned a partially built rule set")
	}
}

func TestArrayBlastRefusesNonVariableHeadAfterStoreSubstitution(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	predicate := declareArrayBlastPredicate(t, f, "array-blast-nonvariable-head", []ast.ASTHandle{arraySort})
	left, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	right, err := f.manager.MkVar(1, arraySort)
	coneMust(t, err)
	index, err := f.manager.MkVar(2, boolean)
	coneMust(t, err)
	value, err := f.manager.MkVar(3, boolean)
	coneMust(t, err)
	family, err := f.manager.ArrayFamilyID()
	coneMust(t, err)
	store, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{right, index, value})
	coneMust(t, err)
	definition, err := f.manager.MkEq(left, store)
	coneMust(t, err)
	head, err := f.manager.MkApp(predicate, []ast.ASTHandle{left})
	coneMust(t, err)
	selectValue, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{left, index})
	coneMust(t, err)
	selection, err := f.manager.MkEq(selectValue, value)
	coneMust(t, err)
	runArrayBlastCoverageNoChange(t, f, head, []ast.ASTHandle{definition, selection})
}

func TestArrayBlastAckermannizesEverySelectIndex(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean, boolean}), boolean)
	coneMust(t, err)
	predicate := declareArrayBlastPredicate(t, f, "array-blast-multi-index", nil)
	head, err := f.manager.MkApp(predicate, nil)
	coneMust(t, err)
	baseArray, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	storedArray, err := f.manager.MkVar(1, arraySort)
	coneMust(t, err)
	firstIndex, err := f.manager.MkVar(2, boolean)
	coneMust(t, err)
	secondIndex, err := f.manager.MkVar(3, boolean)
	coneMust(t, err)
	thirdIndex, err := f.manager.MkVar(4, boolean)
	coneMust(t, err)
	fourthIndex, err := f.manager.MkVar(5, boolean)
	coneMust(t, err)
	storedValue, err := f.manager.MkVar(6, boolean)
	coneMust(t, err)
	firstValue, err := f.manager.MkVar(7, boolean)
	coneMust(t, err)
	secondValue, err := f.manager.MkVar(8, boolean)
	coneMust(t, err)
	var indexReferences []ast.ASTRef
	for _, index := range []ast.ASTHandle{secondIndex, thirdIndex, fourthIndex} {
		reference, refErr := f.manager.NewRef(index)
		coneMust(t, refErr)
		indexReferences = append(indexReferences, reference)
	}
	t.Cleanup(func() {
		for i := len(indexReferences) - 1; i >= 0; i-- {
			coneMust(t, f.manager.DelRef(&indexReferences[i]))
		}
	})
	family, err := f.manager.ArrayFamilyID()
	coneMust(t, err)
	store, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{baseArray, firstIndex, secondIndex, storedValue})
	coneMust(t, err)
	definition, err := f.manager.MkEq(storedArray, store)
	coneMust(t, err)
	firstSelect, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{baseArray, firstIndex, secondIndex})
	coneMust(t, err)
	secondSelect, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{baseArray, thirdIndex, fourthIndex})
	coneMust(t, err)
	firstObservation, err := f.manager.MkEq(firstSelect, firstValue)
	coneMust(t, err)
	secondObservation, err := f.manager.MkEq(secondSelect, secondValue)
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{definition, firstObservation, secondObservation}, nil, "array-blast-multi-index")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(predicate))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("multi-index array-blast fixture is unstratified")
	}
	result, err := newArrayBlastCoveragePlugin(t, f).Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("multi-index select rule was not transformed")
	}
	t.Cleanup(func() { coneMust(t, result.Close()) })
	transformed, err := result.Rule(0)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transformed.Close()) })
	tailCount, err := transformed.TailSize()
	coneMust(t, err)
	constraints := 0
	for i := 0; i < tailCount; i++ {
		tail, _, getErr := transformed.Tail(i)
		coneMust(t, getErr)
		count, guards, countErr := arrayBlastBooleanAckermannCounts(f.manager, tail)
		coneMust(t, countErr)
		constraints += count
		if count != 0 && guards != 2 {
			t.Fatalf("multi-index consistency guard has %d index disequalities, want two", guards)
		}
	}
	if constraints != 1 {
		t.Fatalf("multi-index selects generated %d Ackermann constraints, want one", constraints)
	}
}

func TestArrayBlastTransfersRuleRewriteProof(t *testing.T) {
	f := newConeFixture(t)
	coneMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	predicate := declareArrayBlastPredicate(t, f, "array-blast-proof", nil)
	head, err := f.manager.MkApp(predicate, nil)
	coneMust(t, err)
	left, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	right, err := f.manager.MkVar(1, arraySort)
	coneMust(t, err)
	index, err := f.manager.MkVar(2, boolean)
	coneMust(t, err)
	value, err := f.manager.MkVar(3, boolean)
	coneMust(t, err)
	family, err := f.manager.ArrayFamilyID()
	coneMust(t, err)
	store, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{right, index, value})
	coneMust(t, err)
	definition, err := f.manager.MkEq(left, store)
	coneMust(t, err)
	selected, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{right, index})
	coneMust(t, err)
	selection, err := f.manager.MkEq(selected, value)
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{definition, selection}, nil, "array-blast-proof")
	coneMust(t, err)
	formula, err := rule.ToFormula()
	coneMust(t, err)
	fact, err := f.manager.RefGet(formula)
	coneMust(t, err)
	proof, err := f.manager.MkProofAsserted(fact)
	coneMust(t, err)
	coneMust(t, rule.SetProof(proof))
	coneMust(t, f.manager.DelRef(&formula))
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(predicate))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("array-blast proof fixture is unstratified")
	}
	plugin := newArrayBlastCoveragePlugin(t, f)
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("array-blast proof input was not transformed")
	}
	t.Cleanup(func() { coneMust(t, result.Close()) })
	transformed, err := result.Rule(0)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transformed.Close()) })
	transferred, found, err := transformed.Proof()
	if err != nil || !found {
		t.Fatal("array-blast replacement lost its proof", found, err)
	}
	// blast: p = mk_modus_ponens(r.get_proof(), mk_rewrite(fml1, fml2)),
	// where fml2 is the blasted (=> body head) before Horn normalization
	// and tail simplification.
	parents, err := f.manager.GetProofNumParents(transferred)
	coneMust(t, err)
	if parents != 2 {
		t.Fatal("transferred proof is not a modus ponens", parents)
	}
	premise, err := f.manager.GetProofParent(transferred, 0)
	coneMust(t, err)
	if premise != proof {
		t.Fatal("transferred proof does not start from the source rule proof")
	}
	if mode, modeErr := f.manager.ProofMode(); modeErr != nil || mode != ast.ASTProofEnabled {
		t.Fatal("array-blast changed ambient proof mode", mode, modeErr)
	}
}
