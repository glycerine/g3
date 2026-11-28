package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestFiniteBinaryModelWitness(t *testing.T) {
	for _, tc := range []struct {
		name       string
		generation uint32
		sameValue  bool
		wantNamed  bool
	}{
		{"equivalent", 2, true, true},
		{"laterGeneration", 3, true, false},
		{"differentValue", 2, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "UF", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "processor"), nil)
			astTestMust(t, err)
			x, err := m.MkVar(0, sort)
			astTestMust(t, err)
			y, err := m.MkVar(1, sort)
			astTestMust(t, err)
			body, err := m.MkEq(x, y)
			astTestMust(t, err)
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort, sort}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body})
			astTestMust(t, err)
			state := solver.kernel.internalizer
			astTestMust(t, state.Internalize(q, true))
			finder := state.quantifierManager.modelFinder
			checker := state.quantifierManager.modelChecker
			var values, fresh [2]ast.ASTHandle
			evaluated := make(map[ast.ASTHandle]ast.ASTHandle)
			named, err := m.MkConst(astTestSymbol(t, m, "prior"), sort)
			astTestMust(t, err)
			for i := range values {
				values[i], err = m.MkModelValue(uint32(i), sort)
				astTestMust(t, err)
				astTestPin(t, m, values[i])
				fresh[i], err = m.MkFreshConst("p", sort)
				astTestMust(t, err)
				set, err := finder.instantiationSet(q, uint32(i), true)
				astTestMust(t, err)
				_, err = set.Insert(fresh[i], 2)
				astTestMust(t, err)
				evaluated[fresh[i]] = values[i]
				if i == 1 {
					_, err = set.Insert(named, tc.generation)
					astTestMust(t, err)
					evaluated[named] = values[0]
					if tc.sameValue {
						evaluated[named] = values[1]
					}
				}
				astTestMust(t, set.MakeInverse(func(h ast.ASTHandle) (ast.ASTHandle, error) { return evaluated[h], nil }))
				// Reproduce the table order in which the fresh term wins the tie.
				astTestMust(t, set.inverse.Insert(values[i], [2]ast.ASTHandle{fresh[i]}))
			}
			finder.aufSolver.evalCache[1] = evaluated
			added, err := checker.addFiniteCounterexample(q, values[:], true)
			astTestMust(t, err)
			if !added || len(checker.newInstances) != 1 {
				t.Fatal("expected one finite counterexample")
			}
			want := fresh[1]
			if tc.wantNamed {
				want = named
			}
			got := checker.newInstances[0]
			if got.bindings[0] != fresh[0] || got.bindings[1] != want || got.generation != 2 {
				t.Fatalf("got %v generation %d, want [%v %v] generation 2", got.bindings, got.generation, fresh[0], want)
			}
		})
	}
}
