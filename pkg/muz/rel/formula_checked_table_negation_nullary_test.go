package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFormulaCheckedTableNullaryNegation(t *testing.T) {
	for _, test := range []struct {
		name            string
		nullaryTarget   bool
		negativePresent bool
	}{
		{name: "unary-empty"},
		{name: "unary-present", negativePresent: true},
		{name: "nullary-empty", nullaryTarget: true},
		{name: "nullary-present", nullaryTarget: true, negativePresent: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newTableRelationFixture(t)
			config := formulaCheckerConfig(t, f.ast)
			create := func(sorts []ast.ASTHandle) *FormulaCheckedTable {
				table, err := NewTableRelation(f.relations, sorts)
				astTestMust(t, err)
				checked, err := NewFormulaCheckedTable(table, config)
				astTestMust(t, err)
				return checked
			}
			sorts := []ast.ASTHandle{f.boolean}
			facts := [][]ast.ASTHandle{{f.falsehood}, {f.truth}}
			if test.nullaryTarget {
				sorts = nil
				facts = [][]ast.ASTHandle{{}}
			}
			target := create(sorts)
			defer func() { astTestMust(t, target.Close()) }()
			negative := create(nil)
			defer func() { astTestMust(t, negative.Close()) }()
			for _, fact := range facts {
				_, err := target.AddFact(fact)
				astTestMust(t, err)
			}
			if test.negativePresent {
				_, err := negative.AddFact(nil)
				astTestMust(t, err)
			}

			astTestMust(t, target.FilterByNegation(negative, nil, nil))
			wantRows := len(facts)
			wantFormula := f.truth
			if test.negativePresent {
				wantRows = 0
				wantFormula = f.falsehood
			}
			if rows, err := target.Len(); err != nil || rows != wantRows {
				t.Fatal("nullary negation target rows", rows, wantRows, err)
			}
			for _, fact := range facts {
				if present, err := target.ContainsFact(fact); err != nil || present == test.negativePresent {
					t.Fatal("nullary negation target membership", fact, present, err)
				}
			}
			if present, err := negative.ContainsFact(nil); err != nil || present != test.negativePresent {
				t.Fatal("nullary negation changed negative relation", present, err)
			}
			formula, err := target.ToFormula()
			astTestMust(t, err)
			defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
			root, err := f.ast.RefGet(formula)
			astTestMust(t, err)
			astTestMust(t, target.verifyGrounded(root, wantFormula, false))
			if consistent, err := target.ConsistentFormula(nil); err != nil || !consistent {
				t.Fatal("nullary negation formula differs from relation", consistent, err)
			}
		})
	}
}
