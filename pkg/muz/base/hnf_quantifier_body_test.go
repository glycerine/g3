package base_test

// Source: src/muz/base/hnf.cpp imp::eliminate_quantifier_body.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornNormalizerIntroducesPredicateForQuantifiedAntecedentBody(t *testing.T) {
	for _, tc := range []struct {
		name              string
		duplicate, simple bool
	}{
		{"compound-body", false, false},
		{"repeated-quantifier", true, false},
		{"predicate-body-already-normal", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRuleSetFixture(t, "H")
			m := f.manager
			boolean, err := m.MkBoolSort()
			ruleMust(t, err)
			variable, err := m.MkVar(0, boolean)
			ruleMust(t, err)
			var pins []ast.ASTRef
			defer func() {
				for i := len(pins) - 1; i >= 0; i-- {
					ruleMust(t, m.DelRef(&pins[i]))
				}
			}()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				ruleMust(t, err)
				ref, err := m.NewRef(h)
				ruleMust(t, err)
				pins = append(pins, ref)
				return h
			}
			atom := func(name string) ast.ASTHandle {
				t.Helper()
				s, err := m.Symbols.Intern(name)
				ruleMust(t, err)
				decl := own(m.MkFuncDecl(s, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
				return own(m.MkApp(decl, []ast.ASTHandle{variable}))
			}
			p, q := atom("quantified-hnf-P"), atom("quantified-hnf-Q")
			body := p
			if !tc.simple {
				body = own(m.MkAnd(p, q))
			}
			quantified := own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
			antecedent := quantified
			if tc.duplicate {
				antecedent = own(m.MkAnd(quantified, quantified))
			}
			formula := own(m.MkImplies(antecedent, f.app["H"]))
			normalizer, err := base.NewHornNormalizer(m, "quant-body")
			ruleMust(t, err)
			result, err := normalizer.Normalize(formula)
			ruleMust(t, err)
			defer func() { ruleMust(t, result.Close()) }()
			count := 1
			if tc.duplicate {
				count = 2
			}
			if tc.simple {
				count = 0
			}
			if len(result.FreshPredicates) != count || len(result.Formulas) != count+1 {
				t.Fatalf("fresh predicates/formulas = %d/%d, want %d/%d", len(result.FreshPredicates), len(result.Formulas), count, count+1)
			}
			main, err := m.RefGet(result.Formulas[0])
			ruleMust(t, err)
			mainBody, err := m.GetArg(main, 0)
			ruleMust(t, err)
			if tc.duplicate {
				mainBody, err = m.GetArg(mainBody, 0)
				ruleMust(t, err)
			}
			inner, err := m.GetQuantifierBody(mainBody)
			ruleMust(t, err)
			decl, err := m.GetDecl(inner)
			ruleMust(t, err)
			if tc.simple {
				expected, err := m.GetDecl(p)
				ruleMust(t, err)
				if decl != expected {
					t.Fatal("already-predicate quantifier body changed")
				}
				return
			}
			fresh := make(map[ast.ASTHandle]bool)
			for _, ref := range result.FreshPredicates {
				d, err := m.RefGet(ref)
				ruleMust(t, err)
				fresh[d] = true
				arity, err := m.GetArity(d)
				ruleMust(t, err)
				if arity != 1 {
					t.Fatal("fresh predicate must bind the quantified variable", arity)
				}
			}
			if !fresh[decl] {
				t.Fatal("quantified compound body was not replaced by its fresh predicate")
			}
			for _, ref := range result.Formulas[1:] {
				definition, err := m.RefGet(ref)
				ruleMust(t, err)
				head, err := m.GetArg(definition, 1)
				ruleMust(t, err)
				d, err := m.GetDecl(head)
				ruleMust(t, err)
				if !fresh[d] {
					t.Fatal("definition does not conclude a fresh predicate")
				}
				conditions, err := m.GetArg(definition, 0)
				ruleMust(t, err)
				parts, err := m.FlattenAnd([]ast.ASTHandle{conditions})
				ruleMust(t, err)
				if len(parts) != 2 || !((parts[0] == p && parts[1] == q) || (parts[0] == q && parts[1] == p)) {
					t.Fatal("definition changed quantified body", conditions, body, parts, p, q)
				}
			}
		})
	}
}

func TestHornNormalizerDoesNotTreatTheoryConnectivesAsPredicates(t *testing.T) {
	f := newRuleSetFixture(t, "H")
	m := f.manager
	zero, err := m.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	integer, err := m.GetSort(zero)
	ruleMust(t, err)
	x, err := m.MkVar(0, integer)
	ruleMust(t, err)
	positive, err := m.MkArithmeticGT(x, zero)
	ruleMust(t, err)
	negative, err := m.MkArithmeticLT(x, zero)
	ruleMust(t, err)
	disjunction, err := m.MkOr(positive, negative)
	ruleMust(t, err)
	formula, err := m.MkImplies(disjunction, f.app["H"])
	ruleMust(t, err)
	pin, err := m.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&pin)) }()
	normalizer, err := base.NewHornNormalizer(m, "theory-body")
	ruleMust(t, err)
	result, err := normalizer.Normalize(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, result.Close()) }()
	if len(result.FreshPredicates) != 0 || len(result.Formulas) != 1 {
		t.Fatalf("theory-only disjunction introduced predicates or split rules: %d/%d", len(result.FreshPredicates), len(result.Formulas))
	}
	actual, err := m.RefGet(result.Formulas[0])
	ruleMust(t, err)
	if actual != formula {
		t.Fatal("theory-only antecedent changed", actual, formula)
	}
}
