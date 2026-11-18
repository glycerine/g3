package qe_test

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
)

func TestArithmeticCooperDivisorCombinesDirectAndNested(t *testing.T) {
	f := newArithmeticQEFixture(t)
	processor, err := qe.NewArithmeticBoundsProcessor(f.util)
	pluginMust(t, err)
	defer func() { pluginMust(t, processor.Close()) }()
	contains, err := ast.NewContainsApp(f.manager, f.x)
	pluginMust(t, err)
	defer func() { pluginMust(t, contains.Close()) }()
	zero := f.integerNumeral(t, "0")
	one := f.integerNumeral(t, "1")
	five := f.integerNumeral(t, "5")
	seven := f.integerNumeral(t, "7")
	directMod := arithmeticQEMod(t, f, f.x, five)
	direct := f.pin(f.manager.MkEq(zero, directMod))
	if handled, cause := processor.Add(contains, direct); cause != nil || !handled {
		t.Fatal("direct", handled, cause)
	}
	nestedMod := arithmeticQEMod(t, f, f.x, seven)
	nested := f.pin(f.manager.MkEq(nestedMod, one))
	if handled, cause := processor.Add(contains, nested); cause != nil || !handled {
		t.Fatal("nested", handled, cause)
	}
	divisor := unumeral.NewRational()
	found, err := qe.ArithmeticCooperDivisor(processor, &divisor)
	pluginMust(t, err)
	if !found || !isSmallRat(&divisor, 35, 1) {
		t.Fatal("common divisor", found)
	}
}
