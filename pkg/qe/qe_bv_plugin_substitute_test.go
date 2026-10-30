package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBitVectorPluginSubstitutionNormalizesBranchValue(t *testing.T) {
	f := newBVPluginFixture(t, 8)
	formula, err := f.manager.NewRef(f.formula)
	fmMust(t, err)
	definition, err := f.manager.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, f.manager.DelRef(&definition)); fmMust(t, f.manager.DelRef(&formula)) }()
	fmMust(t, f.plugin.Substitute(f.contains, f.rational("260"), formula, &definition))
	rewritten, err := f.manager.RefGet(formula)
	fmMust(t, err)
	contains, err := f.contains.Check(rewritten)
	fmMust(t, err)
	if contains {
		t.Fatal("substitution retained target")
	}
	realizer, err := f.manager.RefGet(definition)
	fmMust(t, err)
	service := f.manager.NumeralService()
	value, err := service.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, service.CleanupValue(&value)) }()
	var width uint32
	matched, err := f.manager.BitVectorNumeralValue(realizer, &value, &width)
	fmMust(t, err)
	if !matched || width != 8 {
		t.Fatalf("matched=%v width=%d", matched, width)
	}
	four := f.rational("4")
	equal, err := service.Equal(&value, four)
	if err != nil || !equal {
		t.Fatalf("normalized value equal=%v err=%v", equal, err)
	}
}
