package rel

import (
	"bytes"
	"strings"
	"testing"
)

func TestLazySparseFunctionalOperationsForceAndMutateActualOwner(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	signature := lazySignature(t, 3, 16)
	base, err := factory.NewFunctional(signature, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = base.Add(Fact{1, 4}); err != nil {
		t.Fatal(err)
	}
	deferred, err := base.RenameCycle(nil)
	if err != nil {
		t.Fatal(err)
	}
	if deferred.Forced() {
		t.Fatal("derived functional root forced eagerly")
	}
	if err = deferred.EnsureFact(Fact{1, 9}); err != nil {
		t.Fatal(err)
	}
	if !deferred.Forced() {
		t.Fatal("functional ensure did not force the lazy root")
	}
	probe := Fact{1, 0}
	if found, fetchErr := deferred.FetchFact(probe); fetchErr != nil || !found || probe[1] != 9 {
		t.Fatal("functional fetch did not observe ensured suffix", found, probe, fetchErr)
	}
	if rows, rowsErr := deferred.SizeEstimateRows(); rowsErr != nil || rows != 1 {
		t.Fatal("functional row estimate", rows, rowsErr)
	}
	if size, sizeErr := deferred.SizeEstimateBytes(); sizeErr != nil || size == 0 {
		t.Fatal("functional byte estimate", size, sizeErr)
	}
	var display bytes.Buffer
	if err = deferred.Display(&display); err != nil || !strings.Contains(display.String(), "(1,9)") {
		t.Fatal("functional display", display.String(), err)
	}
	if removed, removeErr := deferred.Remove(Fact{1, 9}); removeErr != nil || !removed {
		t.Fatal("functional remove", removed, removeErr)
	}
	if length, lengthErr := deferred.Len(); lengthErr != nil || length != 0 {
		t.Fatal("functional removal did not mutate forced owner", length, lengthErr)
	}
	if found, containsErr := base.Contains(Fact{1, 4}); containsErr != nil || !found {
		t.Fatal("derived functional mutation changed source owner", found, containsErr)
	}
}

func TestLazySparseFactoryFunctionalRootsAreIndependentAndNonNullary(t *testing.T) {
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() {
		if err := factory.Close(); err != nil {
			t.Error(err)
		}
	})
	signature := lazySignature(t, 2, 8)
	first, err := factory.NewFunctional(signature, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := factory.NewFunctional(signature, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.engine != second.engine || first.root == second.root {
		t.Fatal("functional checker roots do not independently share factory engine")
	}
	if err = first.EnsureFact(Fact{0, 7}); err != nil {
		t.Fatal(err)
	}
	if found, containsErr := second.Contains(Fact{0, 7}); containsErr != nil || found {
		t.Fatal("independent functional root observed peer mutation", found, containsErr)
	}
	nullary, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	if table, cause := factory.NewFunctional(nullary, 1); cause == nil || table != nil {
		t.Fatal("functional lazy sparse accepted nullary signature", table, cause)
	}
}
