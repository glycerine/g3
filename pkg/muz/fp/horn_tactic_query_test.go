package fp_test

// Source: horn_tactic.cpp:212-240, 267-296. A single verification query is
// passed directly; all other cases create and filter the tactic query symbol.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornTacticQueryConstructionAndModelOwnership(t *testing.T) {
	for _, simplify := range []bool{false, true} {
		for _, models := range []bool{false, true} {
			for _, count := range []int{0, 1, 2} {
				t.Run(fmt.Sprintf("simplify=%t/models=%t/queries=%d", simplify, models, count), func(t *testing.T) {
					c, err := cmd_context.NewStandaloneContext()
					fpMust(t, err)
					defer func() { fpMust(t, c.Close()) }()
					m, err := c.Manager()
					fpMust(t, err)
					g, err := c.GlobalParameters()
					fpMust(t, err)
					boolean, err := m.MkBoolSort()
					fpMust(t, err)
					declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
						sym, e := m.Symbols.Intern(name)
						fpMust(t, e)
						d, e := m.MkFuncDecl(sym, nil, boolean, ast.ASTFuncDeclInfo{})
						fpMust(t, e)
						a, e := m.MkApp(d, nil)
						fpMust(t, e)
						return d, a
					}
					p, pa := declare("horn-query-original-p")
					_, qa := declare("horn-query-original-q")
					_, ra := declare("horn-query-original-r")
					goal, err := tactic.NewGoal(m, false, models, false)
					fpMust(t, err)
					defer func() { fpMust(t, goal.Close()) }()
					fpMust(t, goal.Assert(pa))
					for _, a := range []ast.ASTHandle{qa, ra}[:count] {
						n, e := m.MkNot(a)
						fpMust(t, e)
						fpMust(t, goal.Assert(n))
					}
					var horn *fp.HornTactic
					if simplify {
						horn, err = fp.NewHornSimplifyTactic(m, g.Context, g)
					} else {
						horn, err = fp.NewHornTactic(m, g.Context, g)
					}
					fpMust(t, err)
					fpMust(t, horn.IncRef())
					result := &tactic.TacticResult{}
					defer func() { fpMust(t, result.Close()) }()
					fpMust(t, horn.Apply(goal, result))
					fpMust(t, horn.DecRef()) // Results own their converters after the tactic closes.
					synthetic := simplify || count != 1
					if !models || simplify {
						if (result.ModelConverter != nil) != synthetic {
							t.Fatal("unexpected filter presence", synthetic, result.ModelConverter)
						}
					}
					if simplify || !models {
						return
					}
					if !result.DecidedSat() || result.ModelConverter == nil {
						t.Fatal("expected satisfiable goal and model")
					}
					var model tactic.ModelRef
					defer func() { fpMust(t, model.Close()) }()
					fpMust(t, result.ModelConverter.ApplyModel(&model, 0))
					core, err := model.Get()
					fpMust(t, err)
					value, err := core.ConstantInterpretation(p)
					fpMust(t, err)
					if !m.IsTrue(value) {
						t.Fatal("original fact missing from model")
					}
					n, err := core.NumConstants()
					fpMust(t, err)
					for i := uint32(0); i < n; i++ {
						d, e := core.Constant(i)
						fpMust(t, e)
						name, e := m.GetASTName(d)
						fpMust(t, e)
						text, e := m.Symbols.String(name)
						fpMust(t, e)
						if strings.HasPrefix(text, "query!") {
							t.Fatal("synthetic query leaked into model", text)
						}
					}
				})
			}
		}
	}
}

// The single query can be an open, non-nullary predicate after source
// normalization strips a universal quantifier; it is not a declaration query.
func TestHornTacticSingleQuantifiedQuery(t *testing.T) {
	for _, multiple := range []bool{false, true} {
		t.Run(fmt.Sprintf("multiple=%t", multiple), func(t *testing.T) {
			c, err := cmd_context.NewStandaloneContext()
			fpMust(t, err)
			defer func() { fpMust(t, c.Close()) }()
			m, err := c.Manager()
			fpMust(t, err)
			g, err := c.GlobalParameters()
			fpMust(t, err)
			fpMust(t, g.SourceSetName("fixedpoint.xform.fix_unbound_vars", "false"))
			boolean, err := m.MkBoolSort()
			fpMust(t, err)
			name, err := m.Symbols.Intern("horn-open-query-r")
			fpMust(t, err)
			decl, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			fpMust(t, err)
			truth, err := m.MkTrue()
			fpMust(t, err)
			fact, err := m.MkApp(decl, []ast.ASTHandle{truth})
			fpMust(t, err)
			variable, err := m.MkVar(0, boolean)
			fpMust(t, err)
			body, err := m.MkApp(decl, []ast.ASTHandle{variable})
			fpMust(t, err)
			body, err = m.MkNot(body)
			fpMust(t, err)
			binder, err := m.Symbols.Intern("x")
			fpMust(t, err)
			query, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{binder}, Body: body})
			fpMust(t, err)
			goal, err := tactic.NewGoal(m, false, false, false)
			fpMust(t, err)
			defer func() { fpMust(t, goal.Close()) }()
			fpMust(t, goal.Assert(fact))
			fpMust(t, goal.Assert(query))
			if multiple {
				n, e := m.MkNot(truth)
				fpMust(t, e)
				fpMust(t, goal.Assert(n))
			}
			horn, err := fp.NewHornTactic(m, g.Context, g)
			fpMust(t, err)
			fpMust(t, horn.IncRef())
			defer func() { fpMust(t, horn.DecRef()) }()
			result := &tactic.TacticResult{}
			defer func() { fpMust(t, result.Close()) }()
			fpMust(t, horn.Apply(goal, result))
			if !result.DecidedUnsat() || (result.ModelConverter != nil) != multiple {
				t.Fatal("open query was not reached directly", result.DecidedUnsat(), result.ModelConverter)
			}
		})
	}
}
