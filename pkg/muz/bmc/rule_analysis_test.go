package bmc_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBMCLinearSelectionChecksTailArityAndQuantifiers(t *testing.T) {
	for _, tc := range []struct {
		name           string
		predicateTails int
		quantified     bool
		secondTails    int
		want           bool
	}{
		{"fact", 0, false, 0, true}, {"one-predicate", 1, false, 0, true},
		{"two-predicates", 2, false, 0, false}, {"quantified-tail", 0, true, 0, false},
		{"later-rule-has-two-predicates", 0, false, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			context, err := cmd_context.NewStandaloneContext()
			must(err)
			defer func() { must(context.Close()) }()
			m, err := context.Manager()
			must(err)
			rm, err := base.NewRuleManager(m)
			must(err)
			defer func() { must(rm.Close()) }()
			set, err := base.NewRuleSet(rm)
			must(err)
			defer func() { must(set.Close()) }()
			boolean, err := m.MkBoolSort()
			must(err)
			predicate := func(name string) ast.ASTHandle {
				sym, err := m.Symbols.Intern(name)
				must(err)
				d, err := m.MkFuncDecl(sym, nil, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(rm.RegisterPredicate(d))
				h, err := m.MkApp(d, nil)
				must(err)
				return h
			}
			head, p, q := predicate("bmc-head"), predicate("bmc-p"), predicate("bmc-q")
			tails := []ast.ASTHandle{p, q}[:tc.predicateTails]
			if tc.quantified {
				v, err := m.MkVar(0, boolean)
				must(err)
				qf, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: v})
				must(err)
				negated, err := m.MkNot(qf)
				must(err)
				tails = append(tails, negated)
			}
			rule, err := rm.NewRule(head, tails, nil, "linear-test")
			must(err)
			must(set.Add(rule))
			must(rule.Close())
			if tc.secondTails > 0 {
				second, err := rm.NewRule(head, []ast.ASTHandle{p, q}[:tc.secondTails], nil, "nonlinear-later-rule")
				must(err)
				must(set.Add(second))
				must(second.Close())
			}
			got, err := bmc.IsLinear(set)
			must(err)
			if got != tc.want {
				t.Fatal("linear selection", got, tc.want)
			}
			if !tc.want {
				compiler, err := bmc.NewCompiler(m)
				must(err)
				defer func() { must(compiler.Close()) }()
				if formulas, err := compiler.CompileLinear(set, 1); err == nil || len(formulas) != 0 {
					t.Fatal("nonlinear rule accepted by linear compiler", formulas, err)
				}
			}
		})
	}
}
