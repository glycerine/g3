package qe

import (
	"fmt"
	"github.com/glycerine/g3/pkg/util/symbol"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBitVectorPluginSubstitutionNormalizesBranchValue(t *testing.T) {
	f := newBVPluginFixture(t, 8)
	formula, err := f.manager.NewRef(f.formula)
	fmMust(t, err)
	definition, err := f.manager.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, f.manager.DelRef(&definition)); fmMust(t, f.manager.DelRef(&formula)) }()
	fmMust(t, f.plugin.Substitute(f.contains, f.rational("260"), formula, &definition))
	rewritten, err := f.manager.RefGet(formula)
	fmMust(t, err)
	contains, err := f.contains.Check(rewritten)
	fmMust(t, err)
	if contains {
		t.Fatal("substitution retained target")
	}
	realizer, err := f.manager.RefGet(definition)
	fmMust(t, err)
	service := f.manager.NumeralService()
	value, err := service.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, service.CleanupValue(&value)) }()
	var width uint32
	matched, err := f.manager.BitVectorNumeralValue(realizer, &value, &width)
	fmMust(t, err)
	if !matched || width != 8 {
		t.Fatalf("matched=%v width=%d", matched, width)
	}
	four := f.rational("4")
	equal, err := service.Equal(&value, four)
	if err != nil || !equal {
		t.Fatalf("normalized value equal=%v err=%v", equal, err)
	}
}

func TestBitVectorPluginSubstitutesWideAndNegativeValuesUnderBinders(t *testing.T) {
	for _, tc := range []struct {
		width       uint32
		value, want string
	}{
		{8, "-1", "255"}, {8, "-257", "255"}, {65, "36893488147419103235", "3"},
		{257, "-1", "231584178474632390847141970017375815706539969331281128078915168015826259279871"},
	} {
		for _, absent := range []bool{false, true} {
			for _, withDefinition := range []bool{false, true} {
				t.Run(fmt.Sprintf("width=%d/value=%s/absent=%v/definition=%v", tc.width, tc.value, absent, withDefinition), func(t *testing.T) {
					f := newBVPluginFixture(t, tc.width)
					m := f.manager
					x := f.variable
					if absent {
						x = f.base.constant("qe-bv-unselected", f.sort)
					}
					v := f.base.own(m.MkVar(0, f.sort))
					body := f.base.own(m.MkEq(x, v))
					name, err := m.Symbols.Intern("bound")
					fmMust(t, err)
					quantified := f.base.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.sort}, Names: []symbol.Symbol{name}, Body: body}))
					formula, err := m.NewRef(quantified)
					fmMust(t, err)
					t.Cleanup(func() { fmMust(t, m.DelRef(&formula)) })
					var definition ast.ASTRef
					var definitionArg *ast.ASTRef
					if withDefinition {
						definition, err = m.NewRef(ast.ASTHandle{})
						fmMust(t, err)
						t.Cleanup(func() { fmMust(t, m.DelRef(&definition)) })
						definitionArg = &definition
					}
					fmMust(t, f.plugin.Substitute(f.contains, f.rational(tc.value), formula, definitionArg))
					got, err := m.RefGet(formula)
					fmMust(t, err)
					if absent && got != quantified {
						t.Fatal("absent target changed the formula")
					}
					newBody, err := m.GetQuantifierBody(got)
					fmMust(t, err)
					right, err := m.GetArg(newBody, 1)
					fmMust(t, err)
					if right != v {
						t.Fatal("substitution changed the bound variable")
					}
					checkNumeral := func(h ast.ASTHandle) {
						t.Helper()
						value := f.rational("0")
						var width uint32
						yes, err := m.BitVectorNumeralValue(h, value, &width)
						fmMust(t, err)
						equal, err := m.NumeralService().Equal(value, f.rational(tc.want))
						fmMust(t, err)
						if !yes || !equal || width != tc.width {
							t.Fatal("source modular numeral mismatch", yes, equal, width)
						}
					}
					if !absent {
						left, err := m.GetArg(newBody, 0)
						fmMust(t, err)
						checkNumeral(left)
					}
					if withDefinition {
						realizer, err := m.RefGet(definition)
						fmMust(t, err)
						checkNumeral(realizer)
					}
				})
			}
		}
	}
}
