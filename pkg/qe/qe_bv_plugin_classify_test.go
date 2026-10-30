package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBitVectorPluginUninterpretedDivisionKinds(t *testing.T) {
	f := newBVPluginFixture(t, 8)
	one := f.base.own(f.manager.BitVectorNumeralUint64(1, 8))
	two := f.base.own(f.manager.BitVectorNumeralUint64(2, 8))
	for kind := int32(12); kind <= 21; kind++ {
		arguments := []ast.ASTHandle{one, two}
		if kind <= 16 {
			arguments = arguments[:1]
		}
		application := f.base.own(f.manager.MkFamilyApp(f.plugin.FamilyID(), kind, nil, arguments))
		if yes, err := f.plugin.IsUninterpreted(application); err != nil || !yes {
			t.Fatalf("kind %d uninterpreted=%v err=%v", kind, yes, err)
		}
	}
	add := f.base.own(f.manager.MkBitVectorAdd(one, two))
	if yes, err := f.plugin.IsUninterpreted(add); err != nil || yes {
		t.Fatalf("add case=%v err=%v", yes, err)
	}
}
