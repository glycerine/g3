package qe

import (
	"fmt"
	"github.com/glycerine/g3/pkg/model"
	"testing"
)

func TestBitVectorPluginProjectModelSubstitutesConcreteValue(t *testing.T) {
	f := newBVModelProjectFixture(t, 8, 37)
	result := f.project(t)
	found, err := f.contains.Check(result)
	fmMust(t, err)
	if found {
		t.Fatal("model projection retained target")
	}
	value := f.rational("37")
	var width uint32
	matched, err := f.manager.BitVectorNumeralValue(result, value, &width)
	fmMust(t, err)
	if !matched || width != 8 {
		t.Fatalf("matched=%v width=%d", matched, width)
	}
}

// qe_bv_plugin.cpp initializes val to zero and ignores is_numeral's return.
// Without model completion a missing or symbolic interpretation can remain
// non-numeral; projection still substitutes zero and reports success.
func TestBitVectorPluginProjectModelDefaultsNonNumeralToZero(t *testing.T) {
	for _, width := range []uint32{1, 8, 65, 257} {
		for _, symbolic := range []bool{false, true} {
			t.Run(fmt.Sprintf("width=%d/symbolic=%v", width, symbolic), func(t *testing.T) {
				f := newBVPluginFixture(t, width)
				md, err := model.NewModel(f.manager)
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, md.Close()) })
				if symbolic {
					decl, err := f.manager.GetDecl(f.variable)
					fmMust(t, err)
					y := f.base.constant("qe-bv-symbolic-value", f.sort)
					fmMust(t, md.Core().RegisterConstant(decl, y))
				}
				formula, err := f.manager.NewRef(f.variable)
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, f.manager.DelRef(&formula)) })
				projected, err := f.plugin.ProjectModel(f.contains, md, formula, f.base.r.config)
				fmMust(t, err)
				if !projected {
					t.Fatal("source projection declined non-numeral value")
				}
				result, err := f.manager.RefGet(formula)
				fmMust(t, err)
				value := f.rational("99")
				var gotWidth uint32
				matched, err := f.manager.BitVectorNumeralValue(result, value, &gotWidth)
				fmMust(t, err)
				zero, err := f.manager.NumeralService().IsZero(value)
				fmMust(t, err)
				if !matched || !zero || gotWidth != width {
					t.Fatal("source zero fallback", matched, zero, gotWidth)
				}
				decl, err := f.manager.GetDecl(f.variable)
				fmMust(t, err)
				interpreted, err := md.Core().HasConstantInterpretation(decl)
				fmMust(t, err)
				if interpreted != symbolic {
					t.Fatal("projection completed the model")
				}
			})
		}
	}
}
