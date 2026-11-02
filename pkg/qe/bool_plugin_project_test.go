package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/model"
)

func TestBoolPluginProjectUsesModelAndDefaultsDontCareFalse(t *testing.T) {
	for _, interpreted := range []bool{true, false} {
		t.Run(map[bool]string{true: "true", false: "dont-care"}[interpreted], func(t *testing.T) {
			f := newBoolPluginFixture(t)
			_, pluginErr := f.r.manager.RegisterBitVectorPlugin()
			fmMust(t, pluginErr)
			md, err := model.NewModel(f.r.manager)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			if interpreted {
				decl, cause := f.r.manager.GetDecl(f.x)
				fmMust(t, cause)
				truth, cause := f.r.manager.MkTrue()
				fmMust(t, cause)
				fmMust(t, md.Core().RegisterConstant(decl, truth))
			}
			formula, err := f.r.manager.NewRef(f.x)
			fmMust(t, err)
			defer func() { fmMust(t, f.r.manager.DelRef(&formula)) }()
			fmMust(t, f.plugin.Project(f.contains, md, formula, f.r.config))
			result, err := f.r.manager.RefGet(formula)
			fmMust(t, err)
			if interpreted && !f.r.manager.IsTrue(result) {
				t.Fatal("true interpretation not projected")
			}
			if !interpreted && !f.r.manager.IsFalse(result) {
				t.Fatal("don't-care interpretation did not default false")
			}
		})
	}
}
