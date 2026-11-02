package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Basic equality and arithmetic <= share a family-local declaration kind.
// Arithmetic comparisons must never seed the Boolean/equality substitution.
func TestInterpTailSimplifierPreservesArithmeticBounds(t *testing.T) {
	for _, op := range []string{"le", "ge", "lt", "gt"} {
		t.Run(op, func(t *testing.T) {
			f := newInterpTailFixture(t)
			m := f.manager
			five, err := m.ArithmeticIntegerNumeralText("5")
			coneMust(t, err)
			integer, err := m.GetSort(five)
			coneMust(t, err)
			x, err := m.MkVar(0, integer)
			coneMust(t, err)
			p := f.predicate(t, "bound-"+op, []ast.ASTHandle{integer})
			head, err := m.MkApp(p, []ast.ASTHandle{x})
			coneMust(t, err)
			var bound ast.ASTHandle
			switch op {
			case "le":
				bound, err = m.MkArithmeticLE(x, five)
			case "ge":
				bound, err = m.MkArithmeticGE(x, five)
			case "lt":
				bound, err = m.MkArithmeticLT(x, five)
			case "gt":
				bound, err = m.MkArithmeticGT(x, five)
			}
			coneMust(t, err)
			boundRef, err := m.NewRef(bound)
			coneMust(t, err)
			defer func() { coneMust(t, m.DelRef(&boundRef)) }()
			rule, err := f.rules.NewRule(head, []ast.ASTHandle{bound}, nil, "numeric-bound")
			coneMust(t, err)
			coneMust(t, f.set.Add(rule))
			coneMust(t, rule.Close())
			coneMust(t, f.set.SetOutputPredicate(p))
			valid, err := f.set.Finalize()
			coneMust(t, err)
			if !valid {
				t.Fatal("invalid rule set")
			}
			transformer := base.NewRuleTransformer()
			defer func() { coneMust(t, transformer.Close()) }()
			coneMust(t, transformer.Register(f.plugin))
			_, err = transformer.Apply(f.set)
			coneMust(t, err)
			result, err := f.set.Rule(0)
			coneMust(t, err)
			defer func() { coneMust(t, result.Close()) }()
			got, err := result.Head()
			coneMust(t, err)
			if got != head {
				t.Fatal("arithmetic comparison specialized the rule head", got, head)
			}
			n, err := result.TailSize()
			coneMust(t, err)
			if n != 1 {
				t.Fatal("arithmetic bound removed", n)
			}
			// Strict bounds may normalize to a negated non-strict bound; require
			// semantic equivalence with the original, independently of that spelling.
			tail, negative, err := result.Tail(0)
			coneMust(t, err)
			if negative {
				t.Fatal("interpreted tail acquired predicate negation metadata")
			}
			equal, err := m.MkIff(bound, tail)
			coneMust(t, err)
			universal, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: equal})
			coneMust(t, err)
			negated, err := m.MkNot(universal)
			coneMust(t, err)
			negRef, err := m.NewRef(negated)
			coneMust(t, err)
			defer func() { coneMust(t, m.DelRef(&negRef)) }()
			solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: f.globals.Context, Globals: f.globals})
			coneMust(t, err)
			defer func() { coneMust(t, solver.Close()) }()
			coneMust(t, solver.Assert(negated))
			status, err := solver.CheckSat(nil)
			coneMust(t, err)
			if status != lbool.False {
				t.Fatal("transformed bound differs from original", status)
			}
		})
	}
}
