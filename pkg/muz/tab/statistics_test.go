package tab

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
	ustatistics "github.com/glycerine/g3/pkg/util/statistics"
)

func TestTabStatisticsCollectAndResetSourceKeys(t *testing.T) {
	values := Statistics{Unfold: 7, UnfoldFail: 5, Subsumed: 3}
	collected := ustatistics.New()
	if err := values.Collect(collected); err != nil {
		t.Fatal(err)
	}
	wantKeys := []ustatistics.Key{"tab.num_unfold", "tab.num_unfold_fail", "tab.num_subsumed"}
	wantValues := []uint32{7, 5, 3}
	if collected.Size() != len(wantKeys) {
		t.Fatal("statistic count", collected.Size())
	}
	for i := range wantKeys {
		if !collected.IsUint(i) || collected.Key(i) != wantKeys[i] || collected.UintValue(i) != wantValues[i] {
			t.Fatal("statistic", i, collected.Key(i), collected.UintValue(i))
		}
	}
	values.Reset()
	if values != (Statistics{}) {
		t.Fatal("statistics reset", values)
	}
	after := ustatistics.New()
	if err := values.Collect(after); err != nil || after.Size() != 0 {
		t.Fatal("zero statistics collection", after.Size(), err)
	}
	if err := values.Collect(nil); !errors.Is(err, base.ErrEngineBase) {
		t.Fatal("nil statistics destination", err)
	}
	var absent *Statistics
	absent.Reset()
}
