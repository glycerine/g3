package rel

// Source: dl_check_table.h forwards both estimates to m_tocheck;
// dl_lazy_table.h returns the conservative constant 1 for either estimate.

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestFiniteCheckedPairEstimatesForwardPrimaryThroughExecutionProfile(t *testing.T) {
	for _, primary := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, checker := range []string{"hashtable", "sparse", "lazy_sparse"} {
			for _, count := range []int{0, 2} {
				t.Run(fmt.Sprintf("%s/%s/rows%d", primary, checker, count), func(t *testing.T) {
					lazy := NewLazySparseFiniteFactory()
					defer func() { must(t, lazy.Close()) }()
					factory, err := NewFiniteCheckedFactoryWithLazy(primary, checker, lazy)
					must(t, err)
					pair, err := factory.New(lazySignature(t, 3))
					must(t, err)
					context := NewExecutionContext(nil)
					defer func() { must(t, context.Close()) }()
					must(t, context.SetFiniteRegister(0, pair))
					must(t, context.SetRegisterAnnotation(0, "checked"))
					for i := 0; i < count; i++ {
						_, err = pair.Add(Fact{Element(i)})
						must(t, err)
					}
					var wantRows, wantBytes uint32
					switch concrete := pair.primary.(type) {
					case *HashTable:
						wantRows, err = concrete.SizeEstimateRows()
						must(t, err)
						wantBytes, err = concrete.SizeEstimateBytes()
					case *SparseTable:
						wantRows, err = concrete.SizeEstimateRows()
						must(t, err)
						wantBytes, err = concrete.SizeEstimateBytes()
					case *LazySparseTable:
						wantRows, wantBytes = 1, 1
					}
					must(t, err)
					var output bytes.Buffer
					must(t, context.ReportBigRelations(0, &output))
					want := fmt.Sprintf("bytes %d\nbytes\trows\tannotation\n%d\t%d\tchecked\n", wantBytes, wantBytes, wantRows)
					if output.String() != want {
						t.Fatalf("profile did not forward the selected primary estimate: got %q want %q", output.String(), want)
					}
					facts, err := pair.Facts()
					must(t, err)
					if len(facts) != count {
						t.Fatal("profiling changed actual rows", facts, count)
					}
					must(t, context.Close())
					if _, err = pair.SizeEstimateRows(); !errors.Is(err, ErrCheckTable) {
						t.Fatal("closed estimate accepted", err)
					}
				})
			}
		}
	}
}

func TestFiniteCheckedPairEstimatesLeaveDataMismatchGuardsActive(t *testing.T) {
	lazy := NewLazySparseFiniteFactory()
	defer func() { must(t, lazy.Close()) }()
	factory, err := NewFiniteCheckedFactoryWithLazy("lazy_sparse", "sparse", lazy)
	must(t, err)
	pair, err := factory.New(lazySignature(t, 3))
	must(t, err)
	defer func() { must(t, pair.Close()) }()
	_, err = pair.Add(Fact{0})
	must(t, err)
	// A wrong checker row must still be detected by checked data operations.
	// Source size metadata deliberately consults only the selected primary.
	_, err = pair.checker.Add(Fact{2})
	must(t, err)
	rows, err := pair.SizeEstimateRows()
	must(t, err)
	bytes, err := pair.SizeEstimateBytes()
	must(t, err)
	if rows != 1 || bytes != 1 {
		t.Fatal("lazy primary metadata", rows, bytes)
	}
	if _, err = pair.Facts(); !errors.Is(err, ErrCheckTable) {
		t.Fatal("data observation lost mismatch guard", err)
	}
	if _, err = pair.Contains(Fact{2}); !errors.Is(err, ErrCheckTable) {
		t.Fatal("membership lost mismatch guard", err)
	}
}
