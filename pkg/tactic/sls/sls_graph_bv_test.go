package sls

// Source: src/tactic/sls/sls_tracker.h:300-479 and
// src/tactic/sls/sls_evaluator.h:117-493,523-616.

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func slsBVNumeral(t *testing.T, manager *ast.ASTManager, value int64, bits uint32) ast.ASTHandle {
	t.Helper()
	service := manager.NumeralService()
	n, err := service.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := service.CleanupValue(&n); err != nil {
			t.Fatal(err)
		}
	}()
	if err = service.SetInt64(&n, value); err != nil {
		t.Fatal(err)
	}
	h, err := manager.BitVectorNumeral(&n, bits)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func slsBVConst(t *testing.T, manager *ast.ASTManager, sort ast.ASTHandle, name string) ast.ASTHandle {
	t.Helper()
	symbol, err := manager.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	h, err := manager.MkConst(symbol, sort)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSLSInitializedBVGraphEvaluationEndToEnd(t *testing.T) {
	tracker, manager, numbers, powerCache, context, globals, local, budget := slsTrackerFixture(t)
	memory, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	cooperation, err := cooperate.NewCooperation(budget)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cooperation.Close(); err != nil {
			t.Error(err)
		}
		if err := memory.Close(); err != nil {
			t.Error(err)
		}
	})
	services := model.ModelEvaluatorConfig{Context: context, Globals: globals, Params: local, Memory: memory, Cooperation: cooperation}
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort8, err := manager.BitVectorSort(8)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort8, "sls-bv-x")
	y := slsBVConst(t, manager, sort8, "sls-bv-y")
	three := slsBVNumeral(t, manager, 3, 8)
	fifteen := slsBVNumeral(t, manager, 15, 4)
	sum, err := manager.MkBitVectorAdd(x, y)
	if err != nil {
		t.Fatal(err)
	}
	product, err := manager.MkBitVectorMul(sum, three)
	if err != nil {
		t.Fatal(err)
	}
	extract, err := manager.MkBitVectorExtract(5, 2, product)
	if err != nil {
		t.Fatal(err)
	}
	root, err := manager.MkEq(extract, fifteen)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, root)

	if err = tracker.InitializeExpressions([]ast.ASTHandle{root}); err != nil {
		t.Fatal(err)
	}
	constants, err := tracker.Constants()
	if err != nil || len(constants) != 2 {
		t.Fatal("source constant collection", constants, err)
	}
	if links, err := tracker.Uplinks(sum); err != nil || len(links) != 1 || links[0] != product {
		t.Fatal("initialized uplink", links, err)
	}
	if rootDistance, err := tracker.Distance(root); err != nil || rootDistance != 0 {
		t.Fatal("root distance", rootDistance, err)
	}
	if leafDistance, err := tracker.Distance(x); err != nil || leafDistance != 4 {
		t.Fatal("leaf distance", leafDistance, err)
	}
	if got, err := tracker.GetValue(three); err != nil || slsIntText(t, numbers, got) != "3" {
		t.Fatal("interpreted BV numeral initialization", err)
	}

	five, twoFifty := numeral.NewInt(5), numeral.NewInt(250)
	if err = tracker.SetValue(x, &five); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetValue(y, &twoFifty); err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewEvaluator(manager, tracker, numbers, powerCache)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := evaluator.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err = evaluator.EvaluateGraph([]ast.ASTHandle{root}); err != nil {
		t.Fatal(err)
	}
	if got, err := tracker.GetValue(sum); err != nil || slsIntText(t, numbers, got) != "255" {
		t.Fatal("bvadd", err)
	}
	if got, err := tracker.GetValue(product); err != nil || slsIntText(t, numbers, got) != "253" {
		t.Fatal("bvmul", err)
	}
	if got, err := tracker.GetValue(extract); err != nil || slsIntText(t, numbers, got) != "15" {
		t.Fatal("extract", err)
	}
	if got, err := tracker.GetValue(root); err != nil || slsIntText(t, numbers, got) != "1" {
		t.Fatal("root equality", err)
	}
	if sat, err := tracker.IsSAT(); err != nil || !sat {
		t.Fatal("evaluated top expression", sat, err)
	}
	checked := numeral.NewInt(0)
	defer func() {
		if err := numbers.Del(&checked); err != nil {
			t.Fatal(err)
		}
	}()
	if err = evaluator.EvalChecked(product, &checked, services); err != nil || slsIntText(t, numbers, &checked) != "253" {
		t.Fatal("checked evaluator flow", err)
	}
	currentModel, err := tracker.GetModel()
	if err != nil {
		t.Fatal(err)
	}
	if count, err := currentModel.Core().NumConstants(); err != nil || count != 2 {
		t.Fatal("materialized SLS model constants", count, err)
	}
	zero := numeral.NewInt(0)
	if err = tracker.SetValue(x, &zero); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetModel(currentModel); err != nil {
		t.Fatal(err)
	}
	if restored, err := tracker.GetValue(x); err != nil || slsIntText(t, numbers, restored) != "5" {
		t.Fatal("SLS model round trip", err)
	}
	if err = currentModel.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSLSBVGraphArithmeticBitwiseShiftAndComparisonFamilies(t *testing.T) {
	tracker, manager, numbers, powerCache, _, _, _, _ := slsTrackerFixture(t)
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort8, err := manager.BitVectorSort(8)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort8, "sls-bv-family-x")
	y := slsBVConst(t, manager, sort8, "sls-bv-family-y")
	three8 := slsBVNumeral(t, manager, 3, 8)
	build := []struct {
		name string
		want int64
		make func() (ast.ASTHandle, error)
	}{
		{"sub", 254, func() (ast.ASTHandle, error) { return manager.MkBitVectorSub(x, y) }},
		{"neg", 251, func() (ast.ASTHandle, error) { return manager.MkBitVectorNeg(x) }},
		{"and", 5, func() (ast.ASTHandle, error) { return manager.MkBitVectorAnd([]ast.ASTHandle{x, y}) }},
		{"or", 7, func() (ast.ASTHandle, error) { return manager.MkBitVectorOr([]ast.ASTHandle{x, y}) }},
		{"xor", 2, func() (ast.ASTHandle, error) { return manager.MkBitVectorXor([]ast.ASTHandle{x, y}) }},
		{"not", 250, func() (ast.ASTHandle, error) { return manager.MkBitVectorNot(x) }},
		{"shl", 40, func() (ast.ASTHandle, error) { return manager.MkBitVectorSHL(x, three8) }},
		{"lshr", 0, func() (ast.ASTHandle, error) { return manager.MkBitVectorLSHR(x, three8) }},
		{"udiv", 0, func() (ast.ASTHandle, error) { return manager.MkBitVectorUDiv(x, y) }},
		{"urem", 5, func() (ast.ASTHandle, error) { return manager.MkBitVectorURem(x, y) }},
	}
	roots := make([]ast.ASTHandle, 0, len(build)+1)
	values := make([]ast.ASTHandle, 0, len(build))
	for _, tc := range build {
		node, xerr := tc.make()
		if xerr != nil {
			t.Fatal(tc.name, xerr)
		}
		want := slsBVNumeral(t, manager, tc.want, 8)
		root, xerr := manager.MkEq(node, want)
		if xerr != nil {
			t.Fatal(tc.name, xerr)
		}
		values = append(values, node)
		roots = append(roots, root)
	}
	ule, err := manager.MkBitVectorULE(x, y)
	if err != nil {
		t.Fatal(err)
	}
	roots = append(roots, ule)
	slsPin(t, manager, roots...)
	if err = tracker.InitializeExpressions(roots); err != nil {
		t.Fatal(err)
	}
	five, seven := numeral.NewInt(5), numeral.NewInt(7)
	if err = tracker.SetValue(x, &five); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetValue(y, &seven); err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewEvaluator(manager, tracker, numbers, powerCache)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = evaluator.Close() }()
	if err = evaluator.EvaluateGraph(roots); err != nil {
		t.Fatal(err)
	}
	for i, tc := range build {
		got, xerr := tracker.GetValue(values[i])
		if xerr != nil || slsIntText(t, numbers, got) != strconv.FormatInt(tc.want, 10) {
			t.Fatal(tc.name, xerr, slsIntText(t, numbers, got))
		}
	}
	if got, err := tracker.GetValue(ule); err != nil || slsIntText(t, numbers, got) != "1" {
		t.Fatal("ule", err)
	}
}

func TestSLSBVGraphPhysicalFailurePrefixIsCleanupSafe(t *testing.T) {
	tracker, manager, numbers, powerCache, _, _, _, budget := slsTrackerFixture(t)
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(10001)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort, "sls-bv-failure-x")
	y := slsBVConst(t, manager, sort, "sls-bv-failure-y")
	product, err := manager.MkBitVectorMul(x, y)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, product)
	if err = tracker.InitializeExpressions([]ast.ASTHandle{product}); err != nil {
		t.Fatal(err)
	}
	one, huge := numeral.NewInt(1), numeral.NewInt(0)
	if err = numbers.Mul2k(&one, 10000, &huge); err != nil {
		t.Fatal(err)
	}
	defer func() {
		budget.FailAfter(-1)
		if err := numbers.Del(&huge); err != nil {
			t.Fatal(err)
		}
	}()
	if err = tracker.SetValue(x, &huge); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetValue(y, &huge); err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewEvaluator(manager, tracker, numbers, powerCache)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = evaluator.Close() }()
	budget.FailAfter(0)
	if err = evaluator.EvaluateGraph([]ast.ASTHandle{product}); err == nil {
		t.Fatal("large BV multiplication did not reach a physical refusal")
	}
	budget.FailAfter(-1)
	if _, err = tracker.GetValue(product); err != nil && !errors.Is(err, ErrTracker) {
		t.Fatal("failure prefix corrupted tracker ownership", err)
	}
}

func TestSLSBVGraphSignedDivisionRemainderComparisonAndShift(t *testing.T) {
	tracker, manager, numbers, powerCache, _, _, _, _ := slsTrackerFixture(t)
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort8, err := manager.BitVectorSort(8)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort8, "sls-bv-signed-x")
	y := slsBVConst(t, manager, sort8, "sls-bv-signed-y")
	three := slsBVNumeral(t, manager, 3, 8)
	division, err := manager.MkBitVectorSDiv(x, y)
	if err != nil {
		t.Fatal(err)
	}
	remainder, err := manager.MkBitVectorSRem(x, y)
	if err != nil {
		t.Fatal(err)
	}
	modulo, err := manager.MkBitVectorSMod(x, y)
	if err != nil {
		t.Fatal(err)
	}
	shift, err := manager.MkBitVectorASHR(x, three)
	if err != nil {
		t.Fatal(err)
	}
	lessEqual, err := manager.MkBitVectorSLE(x, y)
	if err != nil {
		t.Fatal(err)
	}
	roots := []ast.ASTHandle{division, remainder, modulo, shift, lessEqual}
	slsPin(t, manager, roots...)
	if err = tracker.InitializeExpressions(roots); err != nil {
		t.Fatal(err)
	}
	minusSixBits, positiveThree := numeral.NewInt(250), numeral.NewInt(3)
	if err = tracker.SetValue(x, &minusSixBits); err != nil {
		t.Fatal(err)
	}
	if err = tracker.SetValue(y, &positiveThree); err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewEvaluator(manager, tracker, numbers, powerCache)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = evaluator.Close() }()
	if err = evaluator.EvaluateGraph(roots); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		name string
		node ast.ASTHandle
		want string
	}{{"sdiv", division, "254"}, {"srem", remainder, "0"}, {"smod", modulo, "0"}, {"ashr", shift, "255"}, {"sle", lessEqual, "1"}} {
		got, xerr := tracker.GetValue(check.node)
		if xerr != nil || slsIntText(t, numbers, got) != check.want {
			t.Fatal(check.name, xerr, slsIntText(t, numbers, got))
		}
	}
}

func TestSLSValueASTPhysicalFailurePrefixCleansInvocation(t *testing.T) {
	tracker, manager, numbers, _, _, _, _, budget := slsTrackerFixture(t)
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(10001)
	if err != nil {
		t.Fatal(err)
	}
	one, huge := numeral.NewInt(1), numeral.NewInt(0)
	if err = numbers.Mul2k(&one, 10000, &huge); err != nil {
		t.Fatal(err)
	}
	defer func() {
		budget.FailAfter(-1)
		if err := numbers.Del(&huge); err != nil {
			t.Fatal(err)
		}
	}()
	budget.FailAfter(0)
	if _, err = tracker.ValueAST(sort, &huge); err == nil {
		t.Fatal("value conversion did not reach a physical refusal")
	}
	budget.FailAfter(-1)
	node, err := tracker.ValueAST(sort, &huge)
	if err != nil {
		t.Fatal("value conversion owner did not recover after prefix cleanup", err)
	}
	if text, matched, err := manager.BitVectorNumeralText(node); err != nil || !matched || text == "" {
		t.Fatal("recovered BV value", text, matched, err)
	}
}
