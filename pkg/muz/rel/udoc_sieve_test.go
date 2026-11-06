package rel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func ownUDocSieve(t *testing.T, sieve *UDocSieve, err error) *UDocSieve {
	t.Helper()
	astTestMust(t, err)
	if sieve == nil {
		t.Fatal("DOC sieve constructor returned no owner")
	}
	t.Cleanup(func() { astTestMust(t, sieve.Close()) })
	return sieve
}

func assertUDocSieveContains(t *testing.T, sieve *UDocSieve, want bool, values ...ast.ASTHandle) {
	t.Helper()
	found, err := sieve.ContainsFact(values)
	astTestMust(t, err)
	if found != want {
		t.Fatal("DOC sieve membership", found, want)
	}
}

func assertUDocSieveMask(t *testing.T, sieve *UDocSieve, want ...bool) {
	t.Helper()
	mask := sieve.InnerColumns()
	if len(mask) != len(want) {
		t.Fatal("DOC sieve mask arity", mask, want)
	}
	for i := range want {
		if mask[i] != want[i] {
			t.Fatal("DOC sieve inner-column mask", mask, want)
		}
	}
}

func assertUDocSievePointFormula(t *testing.T, manager *ast.ASTManager, sieve *UDocSieve, columns []uint32, values []ast.ASTHandle) {
	t.Helper()
	ref, err := sieve.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, manager.DelRef(&ref)) }()
	actual, err := manager.RefGet(ref)
	astTestMust(t, err)
	sorts := sieve.Sorts()
	terms := make([]ast.ASTHandle, len(columns))
	for i, column := range columns {
		variable, err := manager.MkVar(column, sorts[column])
		astTestMust(t, err)
		astTestPin(t, manager, variable)
		terms[i], err = manager.MkEq(variable, values[i])
		astTestMust(t, err)
		astTestPin(t, manager, terms[i])
	}
	expected, err := manager.MkAndArray(terms)
	astTestMust(t, err)
	astTestPin(t, manager, expected)
	oracle := &relationFormulaChecker{manager: manager, sorts: sorts, config: formulaCheckerConfig(t, manager)}
	astTestMust(t, oracle.verifyGrounded(expected, actual, false))
}

func TestUDocSieveCapabilityPreservesWideAndFiniteSorts(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(factory.Close)
	wide, err := f.ast.BitVectorSort(96)
	astTestMust(t, err)
	astTestPin(t, f.ast, wide)
	name, err := f.ast.Symbols.Intern("doc-sieve-finite-three")
	astTestMust(t, err)
	finite, err := f.ast.MkDatalogFiniteSort(name, 3)
	astTestMust(t, err)
	astTestPin(t, f.ast, finite)
	five, err := f.ast.BitVectorNumeralUint64(5, 96)
	astTestMust(t, err)
	astTestPin(t, f.ast, five)
	six, err := f.ast.BitVectorNumeralUint64(6, 96)
	astTestMust(t, err)
	astTestPin(t, f.ast, six)
	two, err := f.ast.MkDatalogNumeral(2, finite)
	astTestMust(t, err)
	astTestPin(t, f.ast, two)
	sorts := []ast.ASTHandle{f.boolean, wide, finite}
	raw, err := NewEmptyUDocSieve(factory, sorts)
	sieve := ownUDocSieve(t, raw, err)
	assertUDocSieveMask(t, sieve, false, true, true)
	innerSorts := sieve.Inner().Sorts()
	if len(innerSorts) != 2 || innerSorts[0] != wide || innerSorts[1] != finite || sieve.Inner().Core().NumBits() != 98 {
		t.Fatal("DOC sieve changed original inner sorts or symbolic width", innerSorts)
	}
	if empty, err := sieve.Empty(); err != nil || !empty {
		t.Fatal("new DOC sieve is not empty", empty, err)
	}
	if added, err := sieve.AddFact([]ast.ASTHandle{f.truth, five, two}); err != nil || !added {
		t.Fatal("DOC sieve insertion", added, err)
	}
	assertUDocSieveContains(t, sieve, true, f.falsehood, five, two)
	assertUDocSieveContains(t, sieve, false, f.truth, six, two)
	assertUDocSievePointFormula(t, f.ast, sieve, []uint32{1, 2}, []ast.ASTHandle{five, two})
	raw, err = NewFullUDocSieve(factory, sorts)
	full := ownUDocSieve(t, raw, err)
	assertUDocSieveMask(t, full, false, true, true)
	assertUDocSieveContains(t, full, true, f.falsehood, six, two)
	assertUDocSievePointFormula(t, f.ast, full, nil, nil)
}

func TestUDocSieveAllIgnoredAndNullaryTruth(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(factory.Close)
	for _, sorts := range [][]ast.ASTHandle{{f.boolean, f.boolean}, nil} {
		raw, err := NewEmptyUDocSieve(factory, sorts)
		empty := ownUDocSieve(t, raw, err)
		if len(empty.Inner().Sorts()) != 0 || empty.Inner().Core().NumBits() != 0 {
			t.Fatal("ignored columns did not use actual zero-column DOC storage")
		}
		if len(sorts) != 0 {
			assertUDocSieveMask(t, empty, false, false)
		}
		if found, err := empty.FastEmpty(); err != nil || !found {
			t.Fatal("empty zero-column DOC sieve", found, err)
		}
		var values, alternate []ast.ASTHandle
		if len(sorts) != 0 {
			values, alternate = []ast.ASTHandle{f.truth, f.falsehood}, []ast.ASTHandle{f.falsehood, f.truth}
		}
		if added, err := empty.AddFact(values); err != nil || !added {
			t.Fatal("zero-column DOC sieve insertion", added, err)
		}
		assertUDocSieveContains(t, empty, true, alternate...)
		assertUDocSievePointFormula(t, f.ast, empty, nil, nil)
		raw, err = NewFullUDocSieve(factory, sorts)
		full := ownUDocSieve(t, raw, err)
		assertUDocSieveContains(t, full, true, alternate...)
		if added, err := full.AddFact(values); err != nil || added {
			t.Fatal("full zero-column DOC sieve added duplicate truth", added, err)
		}
	}
}

func TestUDocSieveProjectionRenumbersOriginalVariablesAndOwnsClones(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(factory.Close)
	bv3, err := f.ast.BitVectorSort(3)
	astTestMust(t, err)
	astTestPin(t, f.ast, bv3)
	one := f.bv(1)
	astTestPin(t, f.ast, one)
	five, err := f.ast.BitVectorNumeralUint64(5, 3)
	astTestMust(t, err)
	astTestPin(t, f.ast, five)
	raw, err := NewEmptyUDocSieve(factory, []ast.ASTHandle{f.boolean, f.bits, f.boolean, bv3})
	source := ownUDocSieve(t, raw, err)
	_, err = source.AddFact([]ast.ASTHandle{f.truth, one, f.falsehood, five})
	astTestMust(t, err)
	for _, test := range []struct {
		name    string
		removed []uint32
		mask    []bool
		columns []uint32
		values  []ast.ASTHandle
	}{
		{"identity", nil, []bool{false, true, false, true}, []uint32{1, 3}, []ast.ASTHandle{one, five}},
		{"leading-ignored", []uint32{0}, []bool{true, false, true}, []uint32{0, 2}, []ast.ASTHandle{one, five}},
		{"all-ignored", []uint32{0, 2}, []bool{true, true}, []uint32{0, 1}, []ast.ASTHandle{one, five}},
		{"first-inner", []uint32{1}, []bool{false, false, true}, []uint32{2}, []ast.ASTHandle{five}},
		{"all-inner", []uint32{1, 3}, []bool{false, false}, nil, nil},
		{"all-columns", []uint32{0, 1, 2, 3}, nil, nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := source.Project(test.removed)
			projected := ownUDocSieve(t, raw, err)
			assertUDocSieveMask(t, projected, test.mask...)
			assertUDocSievePointFormula(t, f.ast, projected, test.columns, test.values)
			astTestMust(t, projected.Inner().Reset())
			assertUDocSieveContains(t, source, true, f.falsehood, one, f.truth, five)
		})
	}
	raw, err = source.Clone()
	clone := ownUDocSieve(t, raw, err)
	answer, err := clone.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&answer)) }()
	astTestMust(t, source.Close())
	assertUDocSieveContains(t, clone, true, f.falsehood, one, f.truth, five)
	astTestMust(t, clone.Close())
	if _, err := f.ast.RefGet(answer); err != nil {
		t.Fatal("owning sieve formula died with its cloned relation", err)
	}
}

func TestUDocSieveJoinsKeepSelectedEqualitiesAndIgnoreFullColumns(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(factory.Close)
	one, two := f.bv(1), f.bv(2)
	astTestPin(t, f.ast, one)
	astTestPin(t, f.ast, two)
	raw, err := NewEmptyUDocSieve(factory, []ast.ASTHandle{f.boolean, f.bits})
	left := ownUDocSieve(t, raw, err)
	_, err = left.AddFact([]ast.ASTHandle{f.truth, one})
	astTestMust(t, err)
	raw, err = NewEmptyUDocSieve(factory, []ast.ASTHandle{f.bits, f.boolean})
	right := ownUDocSieve(t, raw, err)
	_, err = right.AddFact([]ast.ASTHandle{one, f.falsehood})
	astTestMust(t, err)
	_, err = right.AddFact([]ast.ASTHandle{two, f.truth})
	astTestMust(t, err)
	raw, err = left.Join(right, []uint32{1, 0}, []uint32{0, 1})
	joined := ownUDocSieve(t, raw, err)
	assertUDocSieveMask(t, joined, false, true, true, false)
	// Source drops the Bool equality because those columns are ignored. The
	// two different Bool arguments remain admissible, while the BV key matches.
	assertUDocSieveContains(t, joined, true, f.truth, one, one, f.falsehood)
	assertUDocSieveContains(t, joined, false, f.truth, one, two, f.falsehood)
	assertUDocSievePointFormula(t, f.ast, joined, []uint32{1, 2}, []ast.ASTHandle{one, one})
	plain, err := factory.NewEmpty([]ast.ASTHandle{f.bits})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, plain.Close()) })
	_, err = plain.AddFact([]ast.ASTHandle{one})
	astTestMust(t, err)
	_, err = plain.AddFact([]ast.ASTHandle{two})
	astTestMust(t, err)
	raw, err = left.JoinUDoc(plain, []uint32{1}, []uint32{0})
	toRaw := ownUDocSieve(t, raw, err)
	assertUDocSieveMask(t, toRaw, false, true, true)
	assertUDocSieveContains(t, toRaw, true, f.falsehood, one, one)
	assertUDocSieveContains(t, toRaw, false, f.truth, one, two)
	raw, err = plain.JoinSieve(left, []uint32{0}, []uint32{1})
	fromRaw := ownUDocSieve(t, raw, err)
	assertUDocSieveMask(t, fromRaw, true, false, true)
	assertUDocSieveContains(t, fromRaw, true, one, f.falsehood, one)
	assertUDocSieveContains(t, fromRaw, false, two, f.truth, one)
	astTestMust(t, left.Inner().Reset())
	astTestMust(t, plain.Reset())
	assertUDocSieveContains(t, toRaw, true, f.falsehood, one, one)
	assertUDocSieveContains(t, fromRaw, true, one, f.falsehood, one)
}

func TestUDocSieveAdoptionValidatesMaskAndTransfersOnlyOnSuccess(t *testing.T) {
	factory, m, sort, values := udocTableUnionFixture(t)
	inner, err := factory.NewEmpty([]ast.ASTHandle{sort})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, inner.Close()) })
	_, err = inner.AddFact([]ast.ASTHandle{values[1]})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	if bad, err := AdoptUDocSieve([]ast.ASTHandle{boolean, sort}, []bool{true, false}, inner); err == nil || bad != nil {
		t.Fatal("mismatched original inner sorts were adopted", bad, err)
	}
	checkedUDocContains(t, inner, true, values[1])
	raw, err := AdoptUDocSieve([]ast.ASTHandle{sort, sort}, []bool{false, true}, inner)
	sieve := ownUDocSieve(t, raw, err)
	assertUDocSieveMask(t, sieve, false, true)
	assertUDocSieveContains(t, sieve, true, values[3], values[1])
	assertUDocSievePointFormula(t, m, sieve, []uint32{1}, []ast.ASTHandle{values[1]})
	plain, err := factory.NewEmpty([]ast.ASTHandle{sort})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, plain.Close()) })
	_, err = plain.AddFact([]ast.ASTHandle{values[3]})
	astTestMust(t, err)
	// An explicit source mask can ignore a supported sort. Equality to that
	// column is ignored even when its other endpoint belongs to a raw DOC.
	raw, err = sieve.JoinUDoc(plain, []uint32{0}, []uint32{0})
	toRaw := ownUDocSieve(t, raw, err)
	assertUDocSieveContains(t, toRaw, true, values[0], values[1], values[3])
	raw, err = plain.JoinSieve(sieve, []uint32{0}, []uint32{0})
	fromRaw := ownUDocSieve(t, raw, err)
	assertUDocSieveContains(t, fromRaw, true, values[3], values[0], values[1])
	checkedFactory, err := NewFormulaCheckedUDocFactory(m, arena.VectorConfig{}, formulaCheckerConfig(t, m))
	astTestMust(t, err)
	t.Cleanup(checkedFactory.Close)
	checked, err := NewFullUDocSieve(checkedFactory, []ast.ASTHandle{boolean, sort})
	checkedSieve := ownUDocSieve(t, checked, err)
	if !checkedSieve.Inner().FormulaChecked() {
		t.Fatal("selected check_relation(doc) backing lost its checker inside the sieve")
	}
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	assertUDocSieveContains(t, checkedSieve, true, falsehood, values[2])
	checkedClone, err := checkedSieve.Clone()
	checkedClone = ownUDocSieve(t, checkedClone, err)
	if !checkedClone.Inner().FormulaChecked() {
		t.Fatal("clone lost check_relation(doc) backing")
	}
	assertUDocSieveContains(t, checkedClone, true, truth, values[3])
	astTestMust(t, sieve.Close())
	if inner.Core() != nil {
		t.Fatal("successful sieve adoption did not own its inner relation")
	}
}

func TestUDocSieveOwnsIgnoredSortsThroughCloneAndProjection(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(factory.Close)
	name, err := f.ast.Symbols.Intern("doc-sieve-fresh-ignored-sort")
	astTestMust(t, err)
	fresh, err := f.ast.MkUninterpretedSort(name, nil)
	astTestMust(t, err)
	caller, err := f.ast.NewRef(fresh)
	astTestMust(t, err)
	defer func() {
		if caller != (ast.ASTRef{}) {
			astTestMust(t, f.ast.DelRef(&caller))
		}
	}()
	raw, err := NewEmptyUDocSieve(factory, []ast.ASTHandle{fresh, f.bits})
	source := ownUDocSieve(t, raw, err)
	assertUDocSieveMask(t, source, false, true)
	astTestMust(t, f.ast.DelRef(&caller))
	if _, err := f.ast.GetKind(fresh); err != nil {
		t.Fatal("sieve lost its ignored sort after the caller released it", err)
	}
	raw, err = source.Clone()
	clone := ownUDocSieve(t, raw, err)
	raw, err = source.Project([]uint32{1})
	projected := ownUDocSieve(t, raw, err)
	assertUDocSieveMask(t, projected, false)
	astTestMust(t, source.Close())
	if sorts := clone.Sorts(); len(sorts) != 2 || sorts[0] != fresh {
		t.Fatal("sieve clone lost the original ignored sort", sorts)
	}
	astTestMust(t, clone.Close())
	if _, err := f.ast.GetKind(fresh); err != nil {
		t.Fatal("projected sieve did not own its ignored original sort", err)
	}
	astTestMust(t, projected.Close())
	if _, err := f.ast.GetKind(fresh); err == nil {
		t.Fatal("closing the final sieve left its ignored sort alive")
	}
}

func TestUDocSieveAdoptionSortPinFailurePreservesCallerOwnership(t *testing.T) {
	failures, successes := 0, 0
	for _, allowance := range []int64{0, 1, 2, 4, 8, 16} {
		t.Run(fmt.Sprintf("reservations-%d", allowance), func(t *testing.T) {
			f := newTableRelationFixture(t)
			factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
			astTestMust(t, err)
			t.Cleanup(factory.Close)
			name, err := f.ast.Symbols.Intern("doc-sieve-allocation-ignored-sort")
			astTestMust(t, err)
			fresh, err := f.ast.MkUninterpretedSort(name, nil)
			astTestMust(t, err)
			astTestPin(t, f.ast, fresh)
			inner, err := factory.NewFull(nil)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, inner.Close()) })
			sorts, mask := make([]ast.ASTHandle, 512), make([]bool, 512)
			for i := range sorts {
				sorts[i] = fresh
			}
			before := f.ast.Stats()
			f.ast.Budget().FailAfter(allowance)
			sieve, cause := AdoptUDocSieve(sorts, mask, inner)
			f.ast.Budget().FailAfter(-1)
			var receipt memory.ConstructionFailure
			if errors.As(cause, &receipt) {
				if receipt.RuntimeOwner() != f.ast.NumeralService().RuntimeOwner() {
					t.Fatal("sort-pin cleanup lost its native runtime")
				}
				shown, exposed := receipt.Error(), receipt.Unwrap()
				astTestMust(t, receipt.Cleanup())
				astTestMust(t, receipt.Cleanup())
				if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil {
					t.Fatal("sort-pin cleanup changed its stable receipt")
				}
			}
			if cause != nil {
				failures++
				if !errors.Is(cause, arena.ErrInjectedFailure) || sieve != nil {
					t.Fatal("failed sieve adoption published an owner or changed its cause", sieve, cause)
				}
				if found, err := inner.ContainsFact(nil); err != nil || !found {
					t.Fatal("failed outer-sort pinning consumed caller-owned DOC", found, err)
				}
			} else {
				successes++
				if sieve.Inner() != inner || len(sieve.Sorts()) != len(sorts) {
					t.Fatal("successful adoption did not publish the exact source owner")
				}
				astTestMust(t, sieve.Close())
				if inner.Core() != nil {
					t.Fatal("successful adoption failed to transfer its DOC owner")
				}
			}
			after := f.ast.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
				t.Fatal("sieve adoption leaked outer-sort ownership", before, after, cause)
			}
			if _, err := f.ast.GetKind(fresh); err != nil {
				t.Fatal("sieve adoption consumed the caller's sort reference", err)
			}
		})
	}
	if failures == 0 || successes == 0 {
		t.Fatal("outer-sort pin campaign missed interruption or completion", failures, successes)
	}
}
