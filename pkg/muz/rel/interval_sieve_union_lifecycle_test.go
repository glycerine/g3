package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalSieveWidenFallsBackToOrdinaryUnion(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	target, err := NewIntervalSieveEmpty(f.ast, []ast.ASTHandle{integer})
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewIntervalSieveEmpty(f.ast, []ast.ASTHandle{integer})
	if err != nil {
		t.Fatal(err)
	}
	if err = target.AddFact([]ast.ASTHandle{numeral("1")}); err != nil {
		t.Fatal(err)
	}
	if err = source.AddFact([]ast.ASTHandle{numeral("2")}); err != nil {
		t.Fatal(err)
	}
	changed, err := target.UnionFrom(source, nil, true)
	if err != nil || !changed {
		t.Fatal("sieve widen fallback", changed, err)
	}
	lower, lowerOpen, upper, upperOpen, err := target.inner.Bounds(0)
	if err != nil {
		t.Fatal(err)
	}
	if lower == nil || lower.Cmp(big.NewRat(1, 1)) != 0 || lowerOpen || upper == nil || upper.Cmp(big.NewRat(2, 1)) != 0 || upperOpen {
		t.Fatal("sieve widened inner interval instead of taking finite union", lower, lowerOpen, upper, upperOpen)
	}
}

func TestIntervalSieveUnionDeltaAndResetPreserveLayout(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	sorts := []ast.ASTHandle{f.boolean, integer, f.boolean}
	target, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	delta, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	two := numeral("2")
	if err = source.AddFact([]ast.ASTHandle{f.truth, two, f.falsehood}); err != nil {
		t.Fatal(err)
	}
	changed, err := target.UnionFrom(source, delta, false)
	if err != nil || !changed {
		t.Fatal("first union", changed, err)
	}
	for _, values := range [][]ast.ASTHandle{{f.truth, two, f.falsehood}, {f.falsehood, two, f.truth}} {
		found, containsErr := target.ContainsFact(values)
		if containsErr != nil || !found {
			t.Fatal("target tuple", values, found, containsErr)
		}
		found, containsErr = delta.ContainsFact(values)
		if containsErr != nil || !found {
			t.Fatal("delta tuple", values, found, containsErr)
		}
	}
	changed, err = target.UnionFrom(source, delta, false)
	if err != nil || changed {
		t.Fatal("idempotent union", changed, err)
	}
	if err = target.Reset(); err != nil {
		t.Fatal(err)
	}
	if !target.Empty() {
		t.Fatal("reset interval sieve remains nonempty")
	}
	if got := target.InnerColumns(); len(got) != 3 || got[0] || !got[1] || got[2] {
		t.Fatal("reset changed layout", got)
	}
	if source.Empty() || delta.Empty() {
		t.Fatal("reset target mutated source or delta")
	}
}

func TestIntervalSieveUnionRejectsDifferentLayoutsAtomically(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	sorts := []ast.ASTHandle{integer}
	target, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	one := numeral("1")
	if err = source.AddFact([]ast.ASTHandle{one}); err != nil {
		t.Fatal(err)
	}
	otherManager := docASTManager(t)
	arithmetic, err := otherManager.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	otherInteger, err := otherManager.MkFamilySort(arithmetic, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := NewIntervalSieveEmpty(otherManager, []ast.ASTHandle{otherInteger})
	if err != nil {
		t.Fatal(err)
	}
	if changed, unionErr := target.UnionFrom(foreign, nil, false); unionErr == nil || changed {
		t.Fatal("foreign layout accepted", changed, unionErr)
	}
	if !target.Empty() || source.Empty() {
		t.Fatal("rejected union mutated operands")
	}
}
