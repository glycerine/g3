package qe

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestBoolPluginProjectionSourceDontCareTrace(t *testing.T) {
	for _, interpretation := range []string{"missing", "symbolic", "true", "false"} {
		t.Run(interpretation, func(t *testing.T) {
			f := newBoolPluginFixture(t)
			m := f.r.manager
			md, err := model.NewModel(m)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, md.Close()) })
			if interpretation != "missing" {
				value := f.q
				if interpretation != "symbolic" {
					value = f.own(m.MkBoolVal(interpretation == "true"))
				}
				d, err := m.GetDecl(f.x)
				fmMust(t, err)
				fmMust(t, md.Core().RegisterConstant(d, value))
			}
			formula, err := m.NewRef(f.x)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, m.DelRef(&formula)) })
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			tags.Enable("qe")
			config := f.r.config
			config.Trace = tags
			fmMust(t, f.plugin.Project(f.contains, md, formula, config))
			result, err := m.RefGet(formula)
			fmMust(t, err)
			if m.IsTrue(result) != (interpretation == "true") || m.IsFalse(result) == (interpretation == "true") {
				t.Fatal("source projection result")
			}
			want := ""
			if tags.ShouldTrace("qe") && (interpretation == "missing" || interpretation == "symbolic") {
				want = "-------- [qe] project src/qe/qe_bool_plugin.cpp:65 ---------\nBoolean is a don't care: bool-solve-x\n------------------------------------------------\n"
			}
			if got := output.String(); got != want {
				t.Fatalf("don't-care trace got %q, want %q", got, want)
			}
		})
	}
}

func TestBoolPluginContextPolaritySourceTrace(t *testing.T) {
	for _, services := range []bool{false, true} {
		for _, mode := range []string{"positive", "mixed", "unit", "disabled"} {
			t.Run(fmt.Sprintf("services=%v/%s", services, mode), func(t *testing.T) {
				f := newFMFixture(t)
				var output bytes.Buffer
				tags := diagnostic.NewTraceTagsWithWriter(&output)
				config := ast.ASTManagerConfig{Symbols: f.r.manager.Symbols, Numerals: f.r.manager.NumeralService(), Budget: f.r.manager.Budget(), TraceTags: tags}
				if services {
					config.Params = f.r.config.Context
					config.Globals = f.r.config.Globals
				}
				m, err := ast.NewASTManager(config)
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, m.Close()) })
				boolean, err := m.MkBoolSort()
				fmMust(t, err)
				var atoms []ast.ASTHandle
				for _, name := range []string{"P", "Q"} {
					symbol, err := m.Symbols.Intern(name)
					fmMust(t, err)
					d, err := m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
					fmMust(t, err)
					a, err := m.MkApp(d, nil)
					fmMust(t, err)
					atoms = append(atoms, a)
				}
				formula := atoms[0]
				if mode != "unit" {
					formula, err = m.MkOr(atoms[0], atoms[1])
					fmMust(t, err)
				}
				root, err := m.NewRef(formula)
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, m.DelRef(&root)) })
				ctx, err := NewSolverContext(m, atoms[:1])
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, ctx.Close()) })
				plugin, err := NewBoolPlugin(ctx)
				fmMust(t, err)
				fmMust(t, ctx.AddPlugin(plugin))
				conjs, err := NewConjunctionEnum(m, formula)
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, conjs.Close()) })
				var negative []ast.ASTHandle
				if mode == "mixed" {
					negative = atoms[:1]
				}
				index, err := NewBoolAtomIndex(m, atoms[:1], negative)
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, index.Close()) })
				if mode != "disabled" {
					tags.Enable("quant_elim")
				}
				solved, err := plugin.SolveContext(conjs, formula, index)
				fmMust(t, err)
				if solved != (mode != "mixed") {
					t.Fatal("trace changed source selection", solved)
				}
				want := ""
				if tags.ShouldTrace("quant_elim") && mode != "unit" {
					want = "-------- [quant_elim] solve_polarized src/qe/qe_bool_plugin.cpp:136 ---------\nP (or P Q)\n------------------------------------------------\n"
				}
				if got := output.String(); got != want {
					t.Fatalf("polarity trace got %q, want %q", got, want)
				}
			})
		}
	}
}
