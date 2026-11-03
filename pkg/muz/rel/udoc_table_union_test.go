package rel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestUnionUDocFromTablePreservesConcreteSourcesAndCumulativeDelta(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy"} {
		t.Run(kind, func(t *testing.T) {
			f := newTableRelationFixture(t)
			factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
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
			target, err := factory.NewEmpty([]ast.ASTHandle{sort})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, target.Close()) })
			delta, err := factory.NewEmpty([]ast.ASTHandle{sort})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, delta.Close()) })
			signature, err := NewSignature([]uint64{4})
			astTestMust(t, err)
			var source FiniteTable
			switch kind {
			case "hashtable":
				source = NewHashTable(signature)
			case "sparse":
				source, err = NewSparseTable(signature, 0)
			case "lazy":
				lazy := NewLazySparseFiniteFactory()
				t.Cleanup(func() { astTestMust(t, lazy.Close()) })
				source, err = lazy.New(signature)
			}
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, source.Close()) })
			for _, value := range []uint64{0, 1, 2} {
				_, err = source.Add(Fact{Element(value)})
				astTestMust(t, err)
			}
			_, err = target.AddFact([]ast.ASTHandle{values[0]})
			astTestMust(t, err)
			_, err = delta.AddFact([]ast.ASTHandle{values[3]})
			astTestMust(t, err)
			before := f.ast.Stats().LiveReferences
			for run := 0; run < 2; run++ {
				astTestMust(t, UnionUDocFromTable(target, source, f.relations, delta))
				for i, value := range values {
					checkedUDocContains(t, target, i != 3, value)
					checkedUDocContains(t, delta, i != 0, value)
				}
			}
			if f.ast.Stats().LiveReferences != before {
				t.Fatal("table union leaked decoded AST roots", before, f.ast.Stats())
			}
			astTestMust(t, target.Reset())
			astTestMust(t, UnionUDocFromTable(target, source, f.relations, nil))
			for i, value := range values {
				checkedUDocContains(t, target, i != 3, value)
				found, err := source.Contains(Fact{Element(i)})
				astTestMust(t, err)
				if found != (i != 3) {
					t.Fatal("DOC union changed its borrowed finite source", i, found)
				}
			}
		})
	}
}

func udocTableUnionFixture(t *testing.T) (*UDocRelationFactory, *ast.ASTManager, ast.ASTHandle, []ast.ASTHandle) {
	t.Helper()
	f := newTableRelationFixture(t)
	factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{PageSize: 1, Descriptors: arena.Config{Budget: f.ast.Budget(), PageSize: 1}})
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

func ownUDocTableUnionRelation(t *testing.T, relation *TypedUDocRelation, err error) *TypedUDocRelation {
	t.Helper()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relation.Close()) })
	return relation
}

func TestUnionUDocFromTableNullaryAndPreflight(t *testing.T) {
	factory, m, sort, _ := udocTableUnionFixture(t)
	manager, err := NewRelationManager(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, manager.Close()) })
	raw, err := factory.NewEmpty(nil)
	target := ownUDocTableUnionRelation(t, raw, err)
	raw, err = factory.NewFull(nil)
	delta := ownUDocTableUnionRelation(t, raw, err)
	signature, err := NewSignature(nil)
	astTestMust(t, err)
	source := NewHashTable(signature)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	_, err = source.Add(nil)
	astTestMust(t, err)
	astTestMust(t, UnionUDocFromTable(target, source, manager, delta))
	checkedUDocContains(t, target, true)
	checkedUDocContains(t, delta, true)
	astTestMust(t, target.Reset())

	raw, err = factory.NewEmpty([]ast.ASTHandle{sort})
	wrongShape := ownUDocTableUnionRelation(t, raw, err)
	if err = UnionUDocFromTable(target, source, manager, wrongShape); !errors.Is(err, ErrDOC) {
		t.Fatal("mismatched delta signature was accepted", err)
	}
	otherFactory, err := NewUDocRelationFactory(m, arena.VectorConfig{})
	astTestMust(t, err)
	t.Cleanup(otherFactory.Close)
	otherDelta, err := otherFactory.NewEmpty(nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, otherDelta.Close()) })
	if err = UnionUDocFromTable(target, source, manager, otherDelta); !errors.Is(err, ErrDOC) {
		t.Fatal("delta from another factory was accepted", err)
	}
	if err = UnionUDocFromTable(wrongShape, source, manager, nil); !errors.Is(err, ErrTableRelation) {
		t.Fatal("mismatched source signature was accepted", err)
	}
	other := newTableRelationFixture(t)
	if err = UnionUDocFromTable(target, source, other.relations, nil); !errors.Is(err, ErrDOC) {
		t.Fatal("foreign AST manager was accepted", err)
	}
	checkedFactory, err := NewFormulaCheckedUDocFactory(m, arena.VectorConfig{}, formulaCheckerConfig(t, m))
	astTestMust(t, err)
	t.Cleanup(checkedFactory.Close)
	raw, err = checkedFactory.NewEmpty(nil)
	checked := checkedUDocOwn(t, raw, err)
	if err = UnionUDocFromTable(checked, source, manager, nil); !errors.Is(err, ErrDOC) {
		t.Fatal("unreachable checked DOC/table dispatch was accepted", err)
	}
	if empty, err := checked.Empty(); err != nil || !empty {
		t.Fatal("checked dispatch rejection changed target", empty, err)
	}
	if err = UnionUDocFromTable(target, source, manager, checked); !errors.Is(err, ErrDOC) {
		t.Fatal("checked delta outside the target factory was accepted", err)
	}
	if empty, err := target.Empty(); err != nil || !empty {
		t.Fatal("preflight rejection changed target", empty, err)
	}
}

func TestUnionUDocFromTablePreservesSourceConservativeMembership(t *testing.T) {
	factory, m, sort, values := udocTableUnionFixture(t)
	manager, err := NewRelationManager(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, manager.Close()) })
	raw, err := factory.NewFull([]ast.ASTHandle{sort})
	target := ownUDocTableUnionRelation(t, raw, err)
	raw, err = factory.NewEmpty([]ast.ASTHandle{sort})
	negative := ownUDocTableUnionRelation(t, raw, err)
	checkedUDocAdd(t, negative, values[2])
	astTestMust(t, target.FilterByNegation(negative, []uint32{0}, []uint32{0}))
	raw, err = factory.NewEmpty([]ast.ASTHandle{sort})
	delta := ownUDocTableUnionRelation(t, raw, err)
	signature, err := NewSignature([]uint64{4})
	astTestMust(t, err)
	source := NewHashTable(signature)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	_, err = source.Add(Fact{0})
	astTestMust(t, err)
	before := m.Stats().LiveReferences
	if found, err := target.ContainsFact([]ast.ASTHandle{values[0]}); err != nil || found {
		t.Fatal("fixture did not reach source conservative membership", found, err)
	}
	// Source DOC contains_fact is conservative for negative cubes. The table
	// union follows that answer, appending the row and publishing it to delta.
	astTestMust(t, UnionUDocFromTable(target, source, manager, delta))
	checkedUDocContains(t, target, true, values[0])
	checkedUDocContains(t, delta, true, values[0])
	checkedUDocContains(t, target, false, values[2])
	if m.Stats().LiveReferences != before {
		t.Fatal("union leaked decoded fact roots", before, m.Stats())
	}
	// Source does not query membership when no delta is requested.
	astTestMust(t, UnionUDocFromTable(target, source, manager, nil))
}

func TestUnionUDocFromTableAllocationFailurePreservesBorrowedOwners(t *testing.T) {
	failures, successes := 0, 0
	for _, allowance := range []int64{0, 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		t.Run(fmt.Sprintf("reservations-%d", allowance), func(t *testing.T) {
			factory, m, sort, values := udocTableUnionFixture(t)
			manager, err := NewRelationManager(m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, manager.Close()) })
			raw, err := factory.NewEmpty([]ast.ASTHandle{sort})
			target := ownUDocTableUnionRelation(t, raw, err)
			raw, err = factory.NewEmpty([]ast.ASTHandle{sort})
			delta := ownUDocTableUnionRelation(t, raw, err)
			signature, err := NewSignature([]uint64{4})
			astTestMust(t, err)
			source := NewHashTable(signature)
			t.Cleanup(func() { astTestMust(t, source.Close()) })
			_, err = source.Add(Fact{1})
			astTestMust(t, err)
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(allowance)
			cause := UnionUDocFromTable(target, source, manager, delta)
			m.Budget().FailAfter(-1)
			var receipt memory.ConstructionFailure
			if errors.As(cause, &receipt) {
				if receipt.RuntimeOwner() != m.NumeralService().RuntimeOwner() {
					t.Fatal("table union cleanup lost its native runtime")
				}
				shown, exposed := receipt.Error(), receipt.Unwrap()
				astTestMust(t, receipt.Cleanup())
				astTestMust(t, receipt.Cleanup())
				if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil {
					t.Fatal("table union cleanup changed its stable receipt")
				}
			}
			if cause != nil {
				failures++
				if !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("unexpected union failure", cause)
				}
			} else {
				successes++
			}
			if m.Stats().LiveReferences != before {
				t.Fatal("failed table union lost decoded roots", before, m.Stats(), cause)
			}
			// The source union commits rows incrementally. Reset the two borrowed
			// output owners before retrying after an interrupted insertion.
			astTestMust(t, target.Reset())
			astTestMust(t, delta.Reset())
			astTestMust(t, UnionUDocFromTable(target, source, manager, delta))
			checkedUDocContains(t, target, true, values[1])
			checkedUDocContains(t, delta, true, values[1])
			if found, err := source.Contains(Fact{1}); err != nil || !found {
				t.Fatal("cleanup retired or changed borrowed source", found, err)
			}
		})
	}
	if failures == 0 || successes == 0 {
		t.Fatal("allocation campaign missed interruption or completion", failures, successes)
	}
}
