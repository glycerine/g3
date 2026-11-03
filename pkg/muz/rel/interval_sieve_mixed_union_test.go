package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func intervalMixedUnionPoint(t *testing.T, manager *ast.ASTManager, sorts []ast.ASTHandle, value ast.ASTHandle) *IntervalRelation {
	t.Helper()
	relation := NewIntervalEmpty(uint32(len(sorts)))
	if err := relation.AddFactExpressions(manager, sorts, []ast.ASTHandle{value}); err != nil {
		t.Fatal(err)
	}
	return relation
}

func requireIntervalMixedBounds(t *testing.T, relation *IntervalRelation, lower, upper int64) {
	t.Helper()
	lo, loOpen, hi, hiOpen, err := relation.Bounds(0)
	if err != nil {
		t.Fatal(err)
	}
	if lo == nil || lo.Cmp(big.NewRat(lower, 1)) != 0 || loOpen || hi == nil || hi.Cmp(big.NewRat(upper, 1)) != 0 || hiOpen {
		t.Fatal("unexpected interval bounds", lo, loOpen, hi, hiOpen)
	}
}

func TestIntervalSieveMixedUnionBothDirectionsAndDeltaKinds(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	sorts := []ast.ASTHandle{integer}
	one, two := numeral("1"), numeral("2")

	target, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	rawSource := intervalMixedUnionPoint(t, f.ast, sorts, one)
	rawDelta := NewIntervalEmpty(1)
	changed, err := target.UnionFromInterval(rawSource, rawDelta, sorts, true)
	if err != nil || !changed {
		t.Fatal("raw source into sieve target", changed, err)
	}
	requireIntervalMixedBounds(t, target.inner, 1, 1)
	requireIntervalMixedBounds(t, rawDelta, 1, 1)

	sieveDelta, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	rawTwo := intervalMixedUnionPoint(t, f.ast, sorts, two)
	changed, err = target.UnionFromIntervalWithSieveDelta(rawTwo, sorts, sieveDelta, true)
	if err != nil || !changed {
		t.Fatal("raw source with sieve delta", changed, err)
	}
	requireIntervalMixedBounds(t, target.inner, 1, 2)
	requireIntervalMixedBounds(t, sieveDelta.inner, 1, 2)

	rawTarget := NewIntervalEmpty(1)
	rawTargetDelta := NewIntervalEmpty(1)
	changed, err = target.UnionIntoInterval(rawTarget, rawTargetDelta, sorts, true)
	if err != nil || !changed {
		t.Fatal("sieve source into raw target", changed, err)
	}
	requireIntervalMixedBounds(t, rawTarget, 1, 2)
	requireIntervalMixedBounds(t, rawTargetDelta, 1, 2)

	secondSieveDelta, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	emptyRawTarget := NewIntervalEmpty(1)
	changed, err = target.UnionIntoIntervalWithSieveDelta(emptyRawTarget, sorts, secondSieveDelta, true)
	if err != nil || !changed {
		t.Fatal("sieve source with sieve delta", changed, err)
	}
	requireIntervalMixedBounds(t, emptyRawTarget, 1, 2)
	requireIntervalMixedBounds(t, secondSieveDelta.inner, 1, 2)
}

func TestIntervalSieveMixedUnionRejectsIgnoredColumnsAtomically(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	sorts := []ast.ASTHandle{f.boolean, integer}
	partial, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	raw := intervalMixedUnionPoint(t, f.ast, []ast.ASTHandle{integer}, numeral("4"))
	target := NewIntervalEmpty(1)
	if changed, unionErr := partial.UnionFromInterval(raw, nil, []ast.ASTHandle{integer}, false); unionErr == nil || changed {
		t.Fatal("partial sieve accepted raw source", changed, unionErr)
	}
	if changed, unionErr := partial.UnionIntoInterval(target, nil, []ast.ASTHandle{integer}, false); unionErr == nil || changed {
		t.Fatal("partial sieve accepted raw target", changed, unionErr)
	}
	if !partial.Empty() || !target.Empty() || raw.Empty() {
		t.Fatal("rejected mixed union mutated an operand")
	}
}
