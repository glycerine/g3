package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func checkedUDocFixture(t *testing.T) (*UDocRelationFactory, *ast.ASTManager, ast.ASTHandle, []ast.ASTHandle) {
	t.Helper()
	f := newTableRelationFixture(t)
	factory, err := NewFormulaCheckedUDocFactory(f.ast, arena.VectorConfig{}, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	t.Cleanup(factory.Close)
	sort, err := f.ast.BitVectorSort(2)
	astTestMust(t, err)
	astTestPin(t, f.ast, sort)
	values := make([]ast.ASTHandle, 4)
	for i := range values {
		values[i], err = f.ast.BitVectorNumeralUint64(uint64(i), 2)
		astTestMust(t, err)
		astTestPin(t, f.ast, values[i])
	}
	return factory, f.ast, sort, values
}

func checkedUDocOwn(t *testing.T, relation *TypedUDocRelation, err error) *TypedUDocRelation {
	t.Helper()
	astTestMust(t, err)
	if relation == nil || !relation.FormulaChecked() {
		t.Fatal("operation did not publish a formula-checked DOC relation")
	}
	t.Cleanup(func() { astTestMust(t, relation.Close()) })
	return relation
}

func checkedUDocAdd(t *testing.T, relation *TypedUDocRelation, values ...ast.ASTHandle) {
	t.Helper()
	added, err := relation.AddFact(values)
	astTestMust(t, err)
	if !added {
		t.Fatal("new checked DOC fact was not added", values)
	}
}

func checkedUDocContains(t *testing.T, relation *TypedUDocRelation, want bool, values ...ast.ASTHandle) {
	t.Helper()
	got, err := relation.ContainsFact(values)
	astTestMust(t, err)
	if got != want {
		t.Fatal("checked DOC membership", values, got, want)
	}
}

func TestFormulaCheckedUDocFactsCloneResetAndCorruption(t *testing.T) {
	factory, _, sort, values := checkedUDocFixture(t)
	raw, err := factory.NewEmpty([]ast.ASTHandle{sort})
	relation := checkedUDocOwn(t, raw, err)
	if empty, err := relation.Empty(); err != nil || !empty {
		t.Fatal("initial empty DOC", empty, err)
	}
	if empty, err := relation.FastEmpty(); err != nil || !empty {
		t.Fatal("initial fast-empty DOC", empty, err)
	}
	checkedUDocAdd(t, relation, values[1])
	if added, err := relation.AddFact([]ast.ASTHandle{values[1]}); err != nil || added {
		t.Fatal("duplicate changed DOC", added, err)
	}
	checkedUDocContains(t, relation, true, values[1])
	checkedUDocContains(t, relation, false, values[2])
	raw, err = relation.Clone()
	clone := checkedUDocOwn(t, raw, err)
	astTestMust(t, relation.Core().Reset())
	if _, err := relation.ContainsFact([]ast.ASTHandle{values[1]}); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("DOC corruption escaped membership verification", err)
	}
	if _, err := relation.Empty(); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("DOC corruption escaped emptiness verification", err)
	}
	if _, err := relation.FastEmpty(); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("DOC corruption escaped fast-emptiness verification", err)
	}
	checkedUDocContains(t, clone, true, values[1])
	astTestMust(t, relation.Reset())
	if empty, err := relation.Empty(); err != nil || !empty {
		t.Fatal("checked DOC reset", empty, err)
	}
	checkedUDocAdd(t, relation, values[2])
	checkedUDocContains(t, clone, false, values[2])
}

func TestFormulaCheckedUDocCloseAfterFactoryRetirementReleasesOracle(t *testing.T) {
	factory, m, sort, values := checkedUDocFixture(t)
	raw, err := factory.NewEmpty([]ast.ASTHandle{sort})
	relation := checkedUDocOwn(t, raw, err)
	checkedUDocAdd(t, relation, values[1])
	before := m.Stats().LiveReferences
	factory.Close()
	// The DOC storage owner has already retired, matching the existing
	// unchecked factory-lifetime error contract. Its separate AST oracle
	// still owns a root and must release it through its captured manager.
	if err = relation.Close(); err == nil {
		t.Fatal("checked relation accepted an already retired factory")
	}
	if relation.checker != nil || m.Stats().LiveReferences != before-1 {
		t.Fatal("factory-first cleanup lost the checked formula root", before, m.Stats())
	}
	astTestMust(t, relation.Close())
}

func TestFormulaCheckedUDocConstructionNativeRefusalKeepsNestedOwners(t *testing.T) {
	budget := arena.NewBudget(1 << 28)
	var inner *typedUDocFormulaOwner
	var candidate *TypedUDocRelation
	var expected ast.ASTRef
	var innerFailure, failure error
	origin := errors.New("checked DOC construction verification failed")
	active, refusing := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && !refusing && failure == nil {
			refusing = true
			innerFailure = finishTypedUDocFormula(inner, origin)
			failure = finishCheckedUDocConstruction(innerFailure, candidate, expected)
			refusing = false
		}
		return value
	}})
	astTestMust(t, err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	astTestMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	astTestMust(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	astTestMust(t, err)
	_, err = m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	guard, err := numbers.NewValue()
	astTestMust(t, err)
	astTestMust(t, numbers.SetString(&guard, "18446744073709551617/18446744073709551619"))
	_, err = m.ArithmeticNumeral(&guard, false)
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, numbers.Del(&guard))
		astTestMust(t, m.Close())
		astTestMust(t, numbers.Close())
		astTestMust(t, symbols.Close())
		astTestMust(t, runtime.Close())
		if used := budget.Used(); used != 0 {
			t.Fatal("checked DOC cleanup fixture leaked bytes", used)
		}
	})
	factory, err := NewFormulaCheckedUDocFactory(m, arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}, formulaCheckerConfig(t, m))
	astTestMust(t, err)
	t.Cleanup(factory.Close)
	sort, err := m.BitVectorSort(2)
	astTestMust(t, err)
	astTestPin(t, m, sort)
	value, err := m.BitVectorNumeralUint64(3, 2)
	astTestMust(t, err)
	astTestPin(t, m, value)
	raw, err := factory.NewEmpty([]ast.ASTHandle{sort})
	source := checkedUDocOwn(t, raw, err)
	checkedUDocAdd(t, source, value)
	candidate, err = source.Clone()
	astTestMust(t, err)
	expected, err = source.ToFormula()
	astTestMust(t, err)
	innerRef, err := source.ToFormula()
	astTestMust(t, err)
	inner = &typedUDocFormulaOwner{manager: m, roots: []ast.ASTRef{innerRef}}
	var receipt memory.ConstructionFailure
	t.Cleanup(func() {
		if receipt != nil {
			astTestMust(t, receipt.Cleanup())
		} else {
			astTestMust(t, inner.close())
			astTestMust(t, m.DelRef(&expected))
			astTestMust(t, candidate.Close())
		}
	})

	// Reenter only the constructor finalizer while the arithmetic factory owns
	// its native guard: AST release refuses without consuming any live root.
	astTestMust(t, runtime.SetMaxCount(uint64(runtime.State().GlobalCount)+1))
	active = true
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := m.ArithmeticNumeralWithInvocation(&guard, false, &invocation)
	active = false
	astTestMust(t, runtime.SetMaxCount(0))
	astTestMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !errors.Is(failure, ast.ErrASTPending) || !errors.As(failure, &receipt) {
		t.Fatal("native constructor refusal did not return its owner", body, failure)
	}
	var nested memory.ConstructionFailure
	if !errors.As(innerFailure, &nested) {
		t.Fatal("constructor lost nested native cleanup receipt", innerFailure)
	}
	owned, ok := receipt.(*typedUDocFormulaCleanupFailure)
	if !ok || receipt == nested || owned.owner.nested != nested || owned.owner.relation != candidate || len(owned.owner.roots) != 1 || owned.owner.roots[0] != expected {
		t.Fatal("constructor receipt lost exact nested, expected, or relation owner", receipt)
	}
	if receipt.RuntimeOwner() != runtime || !errors.Is(receipt.OriginalCause(), origin) || receipt.CleanupCause() == nil {
		t.Fatal("constructor receipt lost runtime or separate failure causes")
	}
	for _, ref := range []ast.ASTRef{innerRef, expected, candidate.checker.formula} {
		_, err = m.RefGet(ref)
		astTestMust(t, err)
	}
	if candidate.Core() == nil {
		t.Fatal("nested cleanup refusal retired the unpublished relation")
	}
	shown, exposed := receipt.Error(), receipt.Unwrap()
	astTestMust(t, receipt.Cleanup())
	astTestMust(t, receipt.Cleanup())
	if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || owned.owner.nested != nil || owned.owner.relation != nil || len(owned.owner.roots) != 0 || len(inner.roots) != 0 || candidate.Core() != nil || candidate.checker != nil {
		t.Fatal("constructor cleanup retry left an owner or changed the error")
	}
	for _, ref := range []ast.ASTRef{innerRef, expected} {
		if _, err = m.RefGet(ref); err == nil {
			t.Fatal("constructor cleanup left an unpublished root live")
		}
	}
	checkedUDocContains(t, source, true, value)
	raw, err = source.Clone()
	retry := checkedUDocOwn(t, raw, err)
	checkedUDocContains(t, retry, true, value)
}

func TestFormulaCheckedUDocJoinProjectAndSourceIndependence(t *testing.T) {
	factory, _, sort, values := checkedUDocFixture(t)
	raw, err := factory.NewEmpty([]ast.ASTHandle{sort, sort})
	left := checkedUDocOwn(t, raw, err)
	raw, err = factory.NewEmpty([]ast.ASTHandle{sort})
	right := checkedUDocOwn(t, raw, err)
	checkedUDocAdd(t, left, values[0], values[1])
	checkedUDocAdd(t, left, values[2], values[3])
	checkedUDocAdd(t, right, values[1])
	raw, err = left.Join(right, []uint32{1}, []uint32{0})
	joined := checkedUDocOwn(t, raw, err)
	checkedUDocContains(t, joined, true, values[0], values[1], values[1])
	checkedUDocContains(t, joined, false, values[2], values[3], values[1])
	raw, err = joined.Project([]uint32{1, 2})
	projected := checkedUDocOwn(t, raw, err)
	raw, err = left.JoinProject(right, []uint32{1}, []uint32{0}, []uint32{1, 2})
	fused := checkedUDocOwn(t, raw, err)
	for i, value := range values {
		checkedUDocContains(t, projected, i == 0, value)
		checkedUDocContains(t, fused, i == 0, value)
	}
	checkedUDocContains(t, left, true, values[2], values[3])
	astTestMust(t, left.Reset())
	astTestMust(t, right.Reset())
	checkedUDocContains(t, fused, true, values[0])
}

func TestFormulaCheckedUDocFiltersAndRename(t *testing.T) {
	factory, m, sort, values := checkedUDocFixture(t)
	raw, err := factory.NewEmpty([]ast.ASTHandle{sort, sort})
	source := checkedUDocOwn(t, raw, err)
	for _, row := range [][2]int{{0, 2}, {1, 1}, {2, 2}} {
		checkedUDocAdd(t, source, values[row[0]], values[row[1]])
	}
	raw, err = source.Rename([]uint32{0, 1})
	renamed := checkedUDocOwn(t, raw, err)
	checkedUDocContains(t, renamed, true, values[2], values[0])
	checkedUDocContains(t, renamed, false, values[0], values[2])
	raw, err = source.Clone()
	filtered := checkedUDocOwn(t, raw, err)
	astTestMust(t, filtered.FilterIdentical([]uint32{0, 1}))
	checkedUDocContains(t, filtered, false, values[0], values[2])
	checkedUDocContains(t, filtered, true, values[1], values[1])
	astTestMust(t, filtered.FilterEqual(0, values[2]))
	checkedUDocContains(t, filtered, false, values[1], values[1])
	checkedUDocContains(t, filtered, true, values[2], values[2])
	variable, err := m.MkVar(1, sort)
	astTestMust(t, err)
	astTestPin(t, m, variable)
	condition, err := m.MkEq(variable, values[2])
	astTestMust(t, err)
	astTestPin(t, m, condition)
	raw, err = source.FilterInterpretedAndProject(condition, []uint32{1})
	projected := checkedUDocOwn(t, raw, err)
	for i, value := range values {
		checkedUDocContains(t, projected, i == 0 || i == 2, value)
	}
	astTestMust(t, source.FilterInterpreted(condition))
	checkedUDocContains(t, source, false, values[1], values[1])
	checkedUDocContains(t, source, true, values[0], values[2])
}

func TestFormulaCheckedUDocRenameThreeCycleUsesSourceDirection(t *testing.T) {
	factory, m, _, _ := checkedUDocFixture(t)
	sorts := make([]ast.ASTHandle, 3)
	values := make([]ast.ASTHandle, 3)
	for i, value := range []uint64{1, 2, 5} {
		var err error
		sorts[i], err = m.BitVectorSort(uint32(i + 1))
		astTestMust(t, err)
		astTestPin(t, m, sorts[i])
		values[i], err = m.BitVectorNumeralUint64(value, uint32(i+1))
		astTestMust(t, err)
		astTestPin(t, m, values[i])
	}
	raw, err := factory.NewEmpty(sorts)
	source := checkedUDocOwn(t, raw, err)
	checkedUDocAdd(t, source, values...)
	raw, err = source.Rename([]uint32{0, 1, 2})
	renamed := checkedUDocOwn(t, raw, err)
	gotSorts := renamed.Sorts()
	for i, sort := range []ast.ASTHandle{sorts[1], sorts[2], sorts[0]} {
		if gotSorts[i] != sort {
			t.Fatal("checked DOC three-cycle signature", i, gotSorts[i], sort)
		}
	}
	checkedUDocContains(t, renamed, true, values[1], values[2], values[0])
	checkedUDocContains(t, source, true, values...)
	raw, err = renamed.Rename([]uint32{2, 1, 0})
	roundtrip := checkedUDocOwn(t, raw, err)
	checkedUDocContains(t, roundtrip, true, values...)
}

func TestFormulaCheckedUDocUnionKeepsCumulativeDelta(t *testing.T) {
	factory, _, sort, values := checkedUDocFixture(t)
	raw, err := factory.NewEmpty([]ast.ASTHandle{sort})
	target := checkedUDocOwn(t, raw, err)
	raw, err = factory.NewEmpty([]ast.ASTHandle{sort})
	source := checkedUDocOwn(t, raw, err)
	raw, err = factory.NewEmpty([]ast.ASTHandle{sort})
	delta := checkedUDocOwn(t, raw, err)
	checkedUDocAdd(t, target, values[0])
	checkedUDocAdd(t, source, values[0])
	checkedUDocAdd(t, source, values[1])
	checkedUDocAdd(t, delta, values[3])
	astTestMust(t, target.UnionFrom(source, delta))
	for i, value := range values {
		checkedUDocContains(t, target, i == 0 || i == 1, value)
		checkedUDocContains(t, source, i == 0 || i == 1, value)
		checkedUDocContains(t, delta, i == 1 || i == 3, value)
	}
	astTestMust(t, target.UnionFrom(source, delta))
	checkedUDocContains(t, delta, true, values[3])
}

func TestFormulaCheckedUDocNegationAndNullary(t *testing.T) {
	factory, _, sort, values := checkedUDocFixture(t)
	raw, err := factory.NewEmpty([]ast.ASTHandle{sort})
	target := checkedUDocOwn(t, raw, err)
	for _, value := range values {
		checkedUDocAdd(t, target, value)
	}
	raw, err = factory.NewEmpty([]ast.ASTHandle{sort})
	negative := checkedUDocOwn(t, raw, err)
	checkedUDocAdd(t, negative, values[2])
	astTestMust(t, target.FilterByNegation(negative, []uint32{0}, []uint32{0}))
	for i, value := range values {
		checkedUDocContains(t, target, i != 2, value)
	}
	checkedUDocContains(t, negative, true, values[2])
	// The unjoined negative column is existentially quantified; matching the
	// target's second column must not capture either target variable.
	raw, err = factory.NewEmpty([]ast.ASTHandle{sort, sort})
	pairs := checkedUDocOwn(t, raw, err)
	raw, err = factory.NewEmpty([]ast.ASTHandle{sort, sort})
	witness := checkedUDocOwn(t, raw, err)
	rows := [][2]int{{0, 1}, {2, 1}, {1, 2}, {3, 3}}
	for _, row := range rows {
		checkedUDocAdd(t, pairs, values[row[0]], values[row[1]])
	}
	checkedUDocAdd(t, witness, values[1], values[3])
	astTestMust(t, pairs.FilterByNegation(witness, []uint32{1}, []uint32{0}))
	for _, row := range rows {
		checkedUDocContains(t, pairs, row[1] != 1, values[row[0]], values[row[1]])
	}
	checkedUDocContains(t, witness, true, values[1], values[3])
	raw, err = factory.NewFull(nil)
	truth := checkedUDocOwn(t, raw, err)
	raw, err = factory.NewEmpty(nil)
	falsity := checkedUDocOwn(t, raw, err)
	checkedUDocContains(t, truth, true)
	checkedUDocContains(t, falsity, false)
	astTestMust(t, truth.FilterByNegation(falsity, nil, nil))
	checkedUDocContains(t, truth, true)
	checkedUDocAdd(t, falsity)
	astTestMust(t, truth.FilterByNegation(falsity, nil, nil))
	checkedUDocContains(t, truth, false)
	astTestMust(t, falsity.Reset())
	checkedUDocContains(t, falsity, false)
}

func TestFormulaCheckedUDocDetectsSourceIncompleteMembership(t *testing.T) {
	factory, m, sort, values := checkedUDocFixture(t)
	raw, err := factory.NewFull([]ast.ASTHandle{sort})
	target := checkedUDocOwn(t, raw, err)
	raw, err = factory.NewEmpty([]ast.ASTHandle{sort})
	negative := checkedUDocOwn(t, raw, err)
	checkedUDocAdd(t, negative, values[2])
	astTestMust(t, target.FilterByNegation(negative, []uint32{0}, []uint32{0}))
	// Source doc_manager::contains (doc.cpp:631-640) conservatively rejects
	// a ground cube when the containing DOC has a negative cube. The formula
	// checker must expose that false negative, just as check_relation does.
	if found, err := target.Core().ContainsFact([]uint64{0}); err != nil || found {
		t.Fatal("source incomplete DOC membership fixture", found, err)
	}
	formula, err := target.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&formula)) }()
	body, err := m.RefGet(formula)
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, m, variable)
	equal, err := m.MkEq(variable, values[2])
	astTestMust(t, err)
	astTestPin(t, m, equal)
	want, err := m.MkNot(equal)
	astTestMust(t, err)
	astTestPin(t, m, want)
	astTestMust(t, target.checker.verifyGrounded(body, want, false))
	if _, err = target.ContainsFact([]ast.ASTHandle{values[0]}); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("checker accepted source's incomplete membership result", err)
	}
	checkedUDocContains(t, target, false, values[2])
}

func TestFormulaCheckedUDocWideBitVectorsUseSymbolicStorage(t *testing.T) {
	factory, m, _, _ := checkedUDocFixture(t)
	wide, err := m.BitVectorSort(96)
	astTestMust(t, err)
	astTestPin(t, m, wide)
	high, err := m.BitVectorNumeralAllOnes(96)
	astTestMust(t, err)
	astTestPin(t, m, high)
	zero, err := m.BitVectorNumeralUint64(0, 96)
	astTestMust(t, err)
	astTestPin(t, m, zero)
	raw, err := factory.NewFull([]ast.ASTHandle{wide})
	full := checkedUDocOwn(t, raw, err)
	checkedUDocContains(t, full, true, high)
	checkedUDocContains(t, full, true, zero)
	if full.Core().NumBits() != 96 {
		t.Fatal("wide checked relation did not preserve DOC width")
	}
	raw, err = factory.NewEmpty([]ast.ASTHandle{wide})
	singleton := checkedUDocOwn(t, raw, err)
	checkedUDocAdd(t, singleton, high)
	checkedUDocContains(t, singleton, true, high)
	checkedUDocContains(t, singleton, false, zero)
	astTestMust(t, singleton.Core().Reset())
	if _, err := singleton.Empty(); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("wide DOC mutation escaped formula verification", err)
	}
}
