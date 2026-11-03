package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseIntervalPrecisionTracksRawAndSieveJoinShape(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	finite, err := NewSparseIntervalFiniteOriginal(relations, []ast.ASTHandle{f.boolean}, true)
	astTestMust(t, err)
	arithmetic, err := NewSparseIntervalArithmeticOriginal(relations, []ast.ASTHandle{integer}, false)
	astTestMust(t, err)
	astTestMust(t, arithmetic.AddFact([]ast.ASTHandle{numeral("7")}))
	if precise, cause := arithmetic.IsPrecise(); cause != nil || precise {
		t.Fatal("raw interval relation reported precise", precise, cause)
	}
	rawNullary, err := arithmetic.Project([]int{0})
	astTestMust(t, err)
	if precise, cause := rawNullary.IsPrecise(); cause != nil || precise {
		t.Fatal("projecting raw interval to nullary changed precision", precise, cause)
	}
	if snapshot, cause := rawNullary.ArithmeticSnapshot(); cause != nil || snapshot.Empty() {
		t.Fatal("raw interval nullary projection lost interval role", snapshot, cause)
	}
	astTestMust(t, rawNullary.Close())
	joined, err := JoinSparseIntervalSieveProducts(finite, arithmetic, nil, nil)
	astTestMust(t, err)
	if precise, cause := joined.IsPrecise(); cause != nil || !precise {
		t.Fatal("finite/interval join did not create precise sieve shape", precise, cause)
	}
	projected, err := joined.Project([]int{0})
	astTestMust(t, err)
	if precise, cause := projected.IsPrecise(); cause != nil || !precise {
		t.Fatal("projection lost sieve wrapper precision", precise, cause)
	}
	astTestMust(t, projected.Close())
	astTestMust(t, joined.Close())

	nullary, err := NewSparseIntervalHashOriginal(relations, nil, true)
	astTestMust(t, err)
	rawJoin, err := JoinSparseIntervalSieveProducts(nullary, arithmetic, nil, nil)
	astTestMust(t, err)
	if precise, cause := rawJoin.IsPrecise(); cause != nil || precise {
		t.Fatal("nullary completion incorrectly wrapped raw interval", precise, cause)
	}
	astTestMust(t, rawJoin.Close())
	astTestMust(t, nullary.Close())
	astTestMust(t, arithmetic.Close())
	astTestMust(t, finite.Close())
}
