package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp's default preprocessing and
// src/muz/base/dl_engine_base.h's empty get_model implementation.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBMCScaleQueryAndDefaultModel(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc"} {
		for _, enabled := range []bool{false, true} {
			name := backend + "/disabled"
			if enabled {
				name = backend + "/enabled"
			}
			t.Run(name, func(t *testing.T) {
				m, g := newPDRLifecycleServices(t)
				for _, key := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
					fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
				}
				if enabled {
					fragmentMust(t, g.SourceSetName("fixedpoint.xform.scale", "true"))
				}
				e, err := NewPropositionalEngineWithServices(m, g.Context, g)
				fragmentMust(t, err)
				defer func() { fragmentMust(t, e.Close()) }()
				fragmentMust(t, e.SelectEngine(backend))
				var pins []ast.ASTRef
				defer func() {
					for i := len(pins) - 1; i >= 0; i-- {
						fragmentMust(t, m.DelRef(&pins[i]))
					}
				}()
				own := func(h ast.ASTHandle, err error) ast.ASTHandle {
					fragmentMust(t, err)
					ref, err := m.NewRef(h)
					fragmentMust(t, err)
					pins = append(pins, ref)
					return h
				}
				one := own(m.ArithmeticRealNumeralText("1"))
				realSort, err := m.GetSort(one)
				fragmentMust(t, err)
				boolean := own(m.MkBoolSort())
				symbol, err := m.Symbols.Intern("scale-bmc-p")
				fragmentMust(t, err)
				p := own(m.MkFuncDecl(symbol, []ast.ASTHandle{realSort}, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(p))
				fact := own(m.MkApp(p, []ast.ASTHandle{one}))
				fragmentMust(t, e.AddRule(fact, "scale-fact"))
				status, err := e.QueryPredicate(p)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("reachable real relation", status)
				}
				first, err := e.Model(p)
				fragmentMust(t, err)
				second, err := e.Model(p)
				fragmentMust(t, err)
				checkEmptyEngineDefaultModel(t, first)
				checkEmptyEngineDefaultModel(t, second)
				fragmentMust(t, first.Close())
				checkEmptyEngineDefaultModel(t, second)
				fragmentMust(t, second.Close())
			})
		}
	}
}
