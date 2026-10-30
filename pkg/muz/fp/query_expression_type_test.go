package fp_test

import (
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestFixedpointQueryExpressionRejectsNonBoolean(t *testing.T) {
	f := newQueryExpressionFixture(t)
	sort := f.pin(f.manager.MkUninterpretedSort(usymbol.Numerical(901), nil))
	value := f.pin(f.manager.MkFreshConst("query-non-boolean", sort))
	if _, err := f.engine.QueryExpression(value); err == nil {
		t.Fatal("non-Boolean query accepted")
	}
}
