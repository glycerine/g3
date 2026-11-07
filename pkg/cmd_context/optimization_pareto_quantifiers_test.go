package cmd_context

// Source: opt_context.cpp:255-259,321-330,549-565 and opt_solver.cpp:288-309.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestParetoQuantifierUnknownInitialAdjustmentAndDirectContinuation(t *testing.T) {
	c, x := purifiedParetoFixture(t)
	p, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&p)) }()
	// Preprocessing stays on: pareto's mk_cmp builds (>= val term), which
	// theory_arith only accepts after asserted_formulas rewrites it.
	for _, name := range []string{"mbqi", "auto_config"} {
		contextMust(t, c.config.Params.SetBoolName(&p, &name, false))
	}
	contextMust(t, c.UpdateOptimizationParameters(&p))
	m := c.manager
	name, err := m.Symbols.Intern("pareto-unchecked-sort")
	contextMust(t, err)
	sort, err := m.MkUninterpretedSort(name, nil)
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	name, err = m.Symbols.Intern("pareto-unchecked-p")
	contextMust(t, err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	contextMust(t, err)
	variable, err := m.MkVar(0, sort)
	contextMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	contextMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{body})
	contextMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body, Patterns: []ast.ASTHandle{pattern}})
	contextMust(t, err)
	contextMust(t, c.Assert(quantifier, ""))
	negative, err := m.MkArithmeticNeg(x)
	contextMust(t, err)
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.AddObjective(negative, true))
	seen := make(map[string]bool)
	for i, want := range []lbool.LBool{lbool.Undef, lbool.True, lbool.True, lbool.False} {
		status, err := c.CheckSat(nil)
		contextMust(t, err)
		if status != want {
			t.Fatal("source initial/continuation status", i, status, want)
		}
		if status == lbool.False {
			if c.optimizationParetoBackend != nil || c.optimizationParetoPurification != nil {
				t.Fatal("terminal enumeration kept backend")
			}
			continue
		}
		if c.optimizationParetoBackend == nil {
			t.Fatal("nonterminal quantified result discarded active Pareto state")
		}
		unknown, err := c.optimizationParetoBackend.OptimizationWasUnknown()
		contextMust(t, err)
		if !unknown {
			t.Fatal("Pareto did not use optimization check policy")
		}
		if i == 0 {
			if !c.objectives[0].valueReady || !c.objectives[1].valueReady {
				t.Fatal("initial unknown lost yielded bounds")
			}
			// The final dominance check is unsatisfiable, so kernel reason can
			// already be unknown; the sticky flag determines status separately.
			reason, err := c.ReasonUnknown()
			contextMust(t, err)
			if reason == "" {
				t.Fatal("unknown result omitted a reason")
			}
		}
		md, err := c.Model()
		contextMust(t, err)
		decl, err := m.GetDecl(x)
		contextMust(t, err)
		value, err := md.ConstantInterpretation(decl)
		contextMust(t, err)
		text, _, matched, err := m.ArithmeticNumeralText(value)
		contextMust(t, err)
		if !matched || seen[text] {
			t.Fatal("repeated/missing candidate point", i, text)
		}
		seen[text] = true
		// The purified negative term has a fresh declaration, which must stay
		// hidden even in the initial unknown candidate model.
		fresh, err := m.GetDecl(c.optimizationParetoPurification.objectives[1].expression)
		contextMust(t, err)
		found, err := md.HasConstantInterpretation(fresh)
		contextMust(t, err)
		if found {
			t.Fatal("candidate model exposed purification declaration")
		}
		contextMust(t, md.DecRef())
	}
	if !seen["0"] || !seen["1"] || !seen["2"] {
		t.Fatal("incomplete frontier", seen)
	}
}
