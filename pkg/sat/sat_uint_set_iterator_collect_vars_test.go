package sat

import (
	"reflect"
	"testing"
)

func TestSATModelConverterCollectedVarsUseUintSetIterator(t *testing.T) {
	converter, _ := satConverterTestOwner(t)
	set, _ := satSolverSetOwner(t)
	for _, variable := range []BoolVar{7, 1, 4} {
		_, err := converter.Mk(satModelBlockLit, variable)
		paramsMust(t, err)
	}
	paramsMust(t, converter.CollectVars(set))
	it, err := set.Begin()
	paramsMust(t, err)
	var got []uint32
	for value, yes := it.Next(); yes; value, yes = it.Next() {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []uint32{7, 1, 4}) {
		t.Fatal(got)
	}
}
