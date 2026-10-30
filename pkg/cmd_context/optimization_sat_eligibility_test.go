package cmd_context

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func optimizationSATFixture(t *testing.T, c *Context) ast.ASTHandle {
	t.Helper()
	boolean, err := c.LookupSort("Bool")
	contextMust(t, err)
	_, err = c.DeclareFunction("sat-eligibility-p", nil, boolean)
	contextMust(t, err)
	p, err := c.MakeApp("sat-eligibility-p", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.AddSoftConstraint(p, big.NewRat(1, 1), ""))
	return p
}

func setOptimizationParameter(t *testing.T, c *Context, name string, value any) {
	t.Helper()
	parameters, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&parameters)) }()
	switch v := value.(type) {
	case bool:
		contextMust(t, c.config.Params.SetBoolName(&parameters, &name, v))
	case string:
		symbol, e := c.manager.Symbols.Intern(v)
		contextMust(t, e)
		contextMust(t, c.config.Params.SetSymbolName(&parameters, &name, symbol))
	default:
		t.Fatalf("unsupported optimization parameter value %T", value)
	}
	contextMust(t, c.UpdateOptimizationParameters(&parameters))
}

func TestOptimizationSATCoreEligibilityMatchesSourceGate(t *testing.T) {
	tests := []struct {
		name   string
		before func(*testing.T, *Context)
		mutate func(*testing.T, *Context)
		want   bool
	}{
		{name: "basic Boolean soft constraint", want: true},
		{name: "SAT disabled", mutate: func(t *testing.T, c *Context) { setOptimizationParameter(t, c, "enable_sat", false) }},
		{name: "Pareto priority", mutate: func(t *testing.T, c *Context) { setOptimizationParameter(t, c, "priority", "pareto") }},
		{name: "unsupported MaxSAT engine", mutate: func(t *testing.T, c *Context) { setOptimizationParameter(t, c, "maxsat_engine", "sortmax") }},
		{name: "proofs enabled", before: func(t *testing.T, c *Context) { contextMust(t, c.SetBoolOption("produce-proofs", true)) }},
		{name: "arithmetic hard constraint", mutate: func(t *testing.T, c *Context) {
			m, err := c.Manager()
			contextMust(t, err)
			integer, err := c.LookupSort("Int")
			contextMust(t, err)
			_, err = c.DeclareFunction("sat-eligibility-x", nil, integer)
			contextMust(t, err)
			x, err := c.MakeApp("sat-eligibility-x", nil, nil, ast.ASTHandle{})
			contextMust(t, err)
			zero, err := m.ArithmeticIntegerNumeralText("0")
			contextMust(t, err)
			lower, err := m.MkArithmeticGE(x, zero)
			contextMust(t, err)
			contextMust(t, c.Assert(lower, ""))
		}},
		{name: "uninterpreted function application", mutate: func(t *testing.T, c *Context) {
			boolean, err := c.LookupSort("Bool")
			contextMust(t, err)
			_, err = c.DeclareFunction("sat-eligibility-f", []ast.ASTHandle{boolean}, boolean)
			contextMust(t, err)
			p, err := c.MakeApp("sat-eligibility-p", nil, nil, ast.ASTHandle{})
			contextMust(t, err)
			formula, err := c.MakeApp("sat-eligibility-f", []ast.ASTHandle{p}, nil, ast.ASTHandle{})
			contextMust(t, err)
			contextMust(t, c.AddSoftConstraint(formula, big.NewRat(1, 1), ""))
		}},
		{name: "pseudo-Boolean hard formula", mutate: func(t *testing.T, c *Context) {
			m, err := c.Manager()
			contextMust(t, err)
			boolean, err := c.LookupSort("Bool")
			contextMust(t, err)
			_, err = c.DeclareFunction("sat-eligibility-q", nil, boolean)
			contextMust(t, err)
			q, err := c.MakeApp("sat-eligibility-q", nil, nil, ast.ASTHandle{})
			contextMust(t, err)
			p, err := c.MakeApp("sat-eligibility-p", nil, nil, ast.ASTHandle{})
			contextMust(t, err)
			atMost, err := m.MkPseudoBooleanAtMost([]ast.ASTHandle{p, q}, 1)
			contextMust(t, err)
			contextMust(t, c.Assert(atMost, ""))
		}, want: true},
		{name: "bit-vector terms remain eligible", mutate: func(t *testing.T, c *Context) {
			m, err := c.Manager()
			contextMust(t, err)
			bitVector, err := m.BitVectorSort(4)
			contextMust(t, err)
			name := "sat-eligibility-bv"
			symbol, err := m.Symbols.Intern(name)
			contextMust(t, err)
			term, err := m.MkConst(symbol, bitVector)
			contextMust(t, err)
			zero, err := m.BitVectorNumeralUint64(0, 4)
			contextMust(t, err)
			equal, err := m.MkEq(term, zero)
			contextMust(t, err)
			contextMust(t, c.AddSoftConstraint(equal, big.NewRat(1, 1), ""))
		}, want: true},
		{name: "bit-vector objective lowering remains eligible", mutate: func(t *testing.T, c *Context) {
			m, err := c.Manager()
			contextMust(t, err)
			bitVector, err := m.BitVectorSort(4)
			contextMust(t, err)
			name := "sat-eligibility-bv-objective"
			symbol, err := m.Symbols.Intern(name)
			contextMust(t, err)
			term, err := m.MkConst(symbol, bitVector)
			contextMust(t, err)
			contextMust(t, c.AddObjective(term, true))
		}, want: true},
		{name: "arithmetic objective", mutate: func(t *testing.T, c *Context) {
			integer, err := c.LookupSort("Int")
			contextMust(t, err)
			_, err = c.DeclareFunction("sat-eligibility-objective", nil, integer)
			contextMust(t, err)
			term, err := c.MakeApp("sat-eligibility-objective", nil, nil, ast.ASTHandle{})
			contextMust(t, err)
			contextMust(t, c.AddObjective(term, true))
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := contextFixture(t)
			if tc.before != nil {
				tc.before(t, c)
			}
			optimizationSATFixture(t, c)
			if tc.mutate != nil {
				tc.mutate(t, c)
			}
			got, err := c.optimizationSATCoreEligible(nil)
			contextMust(t, err)
			if got != tc.want {
				t.Fatal("SAT core eligibility", got, "want", tc.want)
			}
		})
	}
}

func TestOptimizationSATCoreEligibilityChecksAdditionalHardAssumptions(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	optimizationSATFixture(t, c)
	integer, err := c.LookupSort("Int")
	contextMust(t, err)
	_, err = c.DeclareFunction("sat-eligibility-assumption-x", nil, integer)
	contextMust(t, err)
	x, err := c.MakeApp("sat-eligibility-assumption-x", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	assumption, err := m.MkArithmeticGE(x, zero)
	contextMust(t, err)
	eligible, err := c.optimizationSATCoreEligible([]ast.ASTHandle{assumption})
	contextMust(t, err)
	if eligible {
		t.Fatal("the SAT core path accepted a non-propositional check assumption")
	}
}
