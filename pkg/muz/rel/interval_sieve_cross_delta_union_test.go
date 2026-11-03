package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalSieveCrossDeltaUnionCompletesMixedDispatch(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	sorts := []ast.ASTHandle{integer}
	one, two := numeral("1"), numeral("2")

	target, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	if err = target.AddFact([]ast.ASTHandle{one}); err != nil {
		t.Fatal(err)
	}
	if err = source.AddFact([]ast.ASTHandle{two}); err != nil {
		t.Fatal(err)
	}
	rawDelta := NewIntervalEmpty(1)
	changed, err := target.UnionFromSieveWithIntervalDelta(source, rawDelta, sorts, true)
	if err != nil || !changed {
		t.Fatal("sieve union with raw delta", changed, err)
	}
	requireIntervalMixedBounds(t, target.inner, 1, 2)
	requireIntervalMixedBounds(t, rawDelta, 1, 2)

	rawTarget := intervalMixedUnionPoint(t, f.ast, sorts, one)
	rawSource := intervalMixedUnionPoint(t, f.ast, sorts, two)
	sieveDelta, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	changed, err = sieveDelta.UnionIntervalsWithSieveDelta(rawTarget, rawSource, sorts, true)
	if err != nil || !changed {
		t.Fatal("raw union with sieve delta", changed, err)
	}
	requireIntervalMixedBounds(t, rawTarget, 1, 2)
	requireIntervalMixedBounds(t, sieveDelta.inner, 1, 2)
}

func TestIntervalSieveCrossDeltaUnionRejectsPartialSieveAtomically(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	partialSorts := []ast.ASTHandle{f.boolean, integer}
	partialTarget, err := NewIntervalSieveEmpty(f.ast, partialSorts)
	if err != nil {
		t.Fatal(err)
	}
	partialSource, err := NewIntervalSieveEmpty(f.ast, partialSorts)
	if err != nil {
		t.Fatal(err)
	}
	if err = partialSource.AddFact([]ast.ASTHandle{f.truth, numeral("3")}); err != nil {
		t.Fatal(err)
	}
	rawDelta := NewIntervalEmpty(1)
	if changed, unionErr := partialTarget.UnionFromSieveWithIntervalDelta(partialSource, rawDelta, []ast.ASTHandle{integer}, false); unionErr == nil || changed {
		t.Fatal("partial sieve pair accepted raw delta", changed, unionErr)
	}
	if !partialTarget.Empty() || rawDelta.Empty() == false || partialSource.Empty() {
		t.Fatal("rejected sieve/raw-delta union mutated operands")
	}

	rawTarget := NewIntervalEmpty(1)
	rawSource := intervalMixedUnionPoint(t, f.ast, []ast.ASTHandle{integer}, numeral("4"))
	if changed, unionErr := partialTarget.UnionIntervalsWithSieveDelta(rawTarget, rawSource, []ast.ASTHandle{integer}, false); unionErr == nil || changed {
		t.Fatal("partial sieve accepted as raw union delta", changed, unionErr)
	}
	if !rawTarget.Empty() || rawSource.Empty() || !partialTarget.Empty() {
		t.Fatal("rejected raw/sieve-delta union mutated operands")
	}
}
