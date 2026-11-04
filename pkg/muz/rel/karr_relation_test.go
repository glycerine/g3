package rel

import (
	"math/big"
	"testing"
)

func karrRat(value int64) *big.Rat { return big.NewRat(value, 1) }

func karrRow(values ...int64) []big.Rat {
	result := make([]big.Rat, len(values))
	for i, value := range values {
		result[i].SetInt64(value)
	}
	return result
}

func evaluateKarr(row []big.Rat, constant *big.Rat, point []int64) big.Rat {
	var result big.Rat
	result.Set(constant)
	for i := range row {
		var product big.Rat
		product.Mul(&row[i], big.NewRat(point[i], 1))
		result.Add(&result, &product)
	}
	return result
}

func requireKarrPoint(t *testing.T, matrix KarrMatrix, point []int64) {
	t.Helper()
	for i := range matrix.A {
		value := evaluateKarr(matrix.A[i], &matrix.B[i], point)
		if matrix.Eq[i] && value.Sign() != 0 || !matrix.Eq[i] && value.Sign() < 0 {
			t.Fatalf("point %v violates row %d: value=%s equal=%v", point, i, &value, matrix.Eq[i])
		}
	}
}

func TestKarrFactUnionAndDualization(t *testing.T) {
	first := NewKarrEmpty(2, nil)
	if err := first.AddFact([]*big.Rat{karrRat(1), karrRat(2)}); err != nil {
		t.Fatal(err)
	}
	second := NewKarrEmpty(2, nil)
	if err := second.AddFact([]*big.Rat{karrRat(2), karrRat(4)}); err != nil {
		t.Fatal(err)
	}
	changed, err := first.Union(second)
	if err != nil || !changed {
		t.Fatalf("Union() = %v, %v", changed, err)
	}
	matrix, err := first.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	if matrix.Len() == 0 {
		t.Fatal("union lost every affine invariant")
	}
	requireKarrPoint(t, matrix, []int64{1, 2})
	requireKarrPoint(t, matrix, []int64{2, 4})
	if changed, err = first.Union(second); err != nil || changed {
		t.Fatalf("idempotent Union() = %v, %v", changed, err)
	}
}

func TestKarrJoinProjectAndRename(t *testing.T) {
	left := NewKarrEmpty(2, nil)
	if err := left.AddFact([]*big.Rat{karrRat(2), karrRat(3)}); err != nil {
		t.Fatal(err)
	}
	right := NewKarrEmpty(1, nil)
	if err := right.AddFact([]*big.Rat{karrRat(2)}); err != nil {
		t.Fatal(err)
	}
	joined, err := KarrJoin(left, right, []uint32{0}, []uint32{0})
	if err != nil {
		t.Fatal(err)
	}
	joinedMatrix, err := joined.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	requireKarrPoint(t, joinedMatrix, []int64{2, 3, 2})
	projected, err := joined.Project([]uint32{0, 2})
	if err != nil {
		t.Fatal(err)
	}
	projectedMatrix, err := projected.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	requireKarrPoint(t, projectedMatrix, []int64{3})
	if projectedMatrix.Len() == 0 {
		t.Fatal("projection lost the retained coordinate invariant")
	}
	renamed, err := left.Rename([]uint32{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	renamedMatrix, err := renamed.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	requireKarrPoint(t, renamedMatrix, []int64{3, 2})
}

func TestKarrFiltersAndSourceEqualQuirk(t *testing.T) {
	relation := NewKarrFull(3, nil)
	if err := relation.FilterIdentical([]uint32{0, 2}); err != nil {
		t.Fatal(err)
	}
	if err := relation.AddConstraint(karrRow(0, 1, 0), karrRat(-4), true); err != nil {
		t.Fatal(err)
	}
	if err := relation.FilterEqual(0, true); err != nil {
		t.Fatal(err)
	}
	matrix, err := relation.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	requireKarrPoint(t, matrix, []int64{1, 4, 1})
	if matrix.Len() != 3 || matrix.B[2].Cmp(karrRat(-1)) != 0 {
		t.Fatalf("source filter_equal fixed constant not retained: %+v", matrix.B)
	}
}

func TestKarrGuards(t *testing.T) {
	relation := NewKarrEmpty(2, nil)
	if err := relation.AddFact([]*big.Rat{karrRat(1)}); err != ErrKarrDimension {
		t.Fatalf("AddFact dimension error = %v", err)
	}
	if _, err := relation.Project([]uint32{2}); err != ErrKarrColumn {
		t.Fatalf("Project column error = %v", err)
	}
	if _, err := relation.Rename(nil); err != ErrKarrDimension {
		t.Fatalf("Rename cycle error = %v", err)
	}
}

func TestKarrRenameCyclePreservesLargeRationalRows(t *testing.T) {
	source := NewKarrFull(3, nil)
	large := new(big.Int).Lsh(big.NewInt(1), 140)
	row := []big.Rat{*new(big.Rat).SetInt(large), *big.NewRat(2, 1), *big.NewRat(3, 1)}
	if err := source.AddConstraint(row, big.NewRat(0, 1), true); err != nil {
		t.Fatal(err)
	}
	result, err := source.Rename([]uint32{0, 2, 1})
	if err != nil {
		t.Fatal(err)
	}
	matrix, err := result.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	if matrix.A[0][0].Cmp(big.NewRat(3, 1)) != 0 || matrix.A[0][1].Cmp(new(big.Rat).SetInt(large)) != 0 || matrix.A[0][2].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("cycle corrupted coefficients", matrix)
	}
	original, err := source.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	if original.A[0][0].Cmp(new(big.Rat).SetInt(large)) != 0 {
		t.Fatal("rename mutated source", original)
	}
}
