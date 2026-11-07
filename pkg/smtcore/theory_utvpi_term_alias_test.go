package smtcore

// Source: src/smt/theory_utvpi_def.h:516-574 mk_term and mk_num.

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTUTVPINumeralsAndConcreteTermAliases(t *testing.T) {
	for _, integer := range []bool{false, true} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			theory, err := NewSMTUTVPITheory(family)
			astTestMust(t, err)
			astTestMust(t, state.RegisterTheory(theory))

			sourceZero, err := m.ArithmeticNumeralRat(new(big.Rat), true)
			astTestMust(t, err)
			zeroNode := state.context.enodes.get(sourceZero)
			if zeroNode == nil || zeroNode.TheoryVar(family) != theory.zero || theory.zero != 0 {
				t.Fatal("source zero variable was not initialized once", theory.zero)
			}
			zero, err := m.ArithmeticNumeralRat(new(big.Rat), integer)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(zero, false))
			baseEdges, baseVariables := 0, 1
			if !integer {
				// The C++ distinguished zero is Int. Go's typed Real-zero enode
				// therefore receives a concrete unconditional alias to it.
				baseEdges, baseVariables = 4, 2
			}
			if len(theory.graph.graph.edges) != baseEdges || len(theory.graph.integer) != baseVariables {
				t.Fatal("typed zero alias graph state", len(theory.graph.graph.edges), len(theory.graph.integer))
			}

			seven, err := m.ArithmeticNumeralRat(big.NewRat(7, 1), integer)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(seven, false))
			sevenVar := state.context.enodes.get(seven).TheoryVar(family)
			if sevenVar == SMTNullTheoryVar || sevenVar == theory.zero || len(theory.graph.graph.edges) != baseEdges+4 {
				t.Fatal("nonzero numeral did not receive its paired equality", sevenVar, len(theory.graph.graph.edges))
			}

			sort, err := m.GetSort(zero)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "utvpi-alias-x"), sort)
			astTestMust(t, err)
			plus, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, seven})
			astTestMust(t, err)
			minus, err := m.MkFamilyApp(family, smtArithOpSub, nil, []ast.ASTHandle{seven, x})
			astTestMust(t, err)
			for _, expression := range []ast.ASTHandle{plus, minus} {
				astTestMust(t, state.Internalize(expression, false))
				node := state.context.enodes.get(expression)
				if node == nil || !theory.IsAttachedToVariable(node) {
					t.Fatal("concrete term alias has no attached UTVPI variable")
				}
			}
			if len(theory.graph.graph.edges) != baseEdges+12 {
				t.Fatal("term aliases did not add two unconditional paired inequalities each", len(theory.graph.graph.edges))
			}
			for i := range theory.graph.graph.edges {
				edge := &theory.graph.graph.edges[i]
				if !edge.enabled || edge.explanation != SMTNullLiteral {
					t.Fatal("term alias edge was not unconditional", i, edge.enabled, edge.explanation)
				}
			}
			variables, edges := len(theory.graph.integer), len(theory.graph.graph.edges)
			astTestMust(t, state.Internalize(plus, false))
			if len(theory.graph.integer) != variables || len(theory.graph.graph.edges) != edges {
				t.Fatal("term alias reinternalization was not idempotent")
			}

			values, err := theory.graph.modelValues(int32(theory.zero))
			astTestMust(t, err)
			for _, expression := range []ast.ASTHandle{seven, plus, minus} {
				variable := state.context.enodes.get(expression).TheoryVar(family)
				if variable < 0 || int(variable) >= len(values) || values[variable].Cmp(big.NewRat(7, 1)) != 0 {
					t.Fatal("numeral/alias model value", expression, variable, values)
				}
			}
		})
	}
}
