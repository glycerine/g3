package base_test

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextWriteConstantRegisteredAndRaw(t *testing.T) {
	f := newRuleSetFixture(t)
	context, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	var output bytes.Buffer
	ruleMust(t, context.WriteConstant(&output, boolean, 17))
	if output.String() != "17" {
		t.Fatal("raw constant", output.String())
	}
	ruleMust(t, context.RegisterFiniteSort(boolean, base.DatalogSortSymbol))
	name, err := f.manager.Symbols.Intern("written")
	ruleMust(t, err)
	_, err = context.SymbolNumber(boolean, name)
	ruleMust(t, err)
	output.Reset()
	ruleMust(t, context.WriteConstant(&output, boolean, 0))
	if output.String() != "written" {
		t.Fatal("named constant", output.String())
	}
}
