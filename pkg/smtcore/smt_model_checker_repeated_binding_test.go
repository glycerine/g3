package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRepeatedAuxiliaryBinding(t *testing.T) {
	for _, tc := range []struct {
		name    string
		equal   bool
		common  bool
		replace bool
	}{
		{"falsified", false, true, true},
		{"satisfied", true, true, false},
		{"differentRepresentatives", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "UF", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			state := solver.kernel.internalizer
			sort, err := m.MkBoolSort()
			astTestMust(t, err)
			x, err := m.MkVar(0, sort)
			astTestMust(t, err)
			y, err := m.MkVar(1, sort)
			astTestMust(t, err)
			var body ast.ASTHandle
			if tc.equal {
				body, err = m.MkEq(x, y)
			} else {
				var either ast.ASTHandle
				either, err = m.MkOr(x, y)
				astTestMust(t, err)
				body, err = m.MkNot(either)
			}
			astTestMust(t, err)
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{sort, sort},
				Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body,
			})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(q, true))
			qm := state.quantifierManager
			no, err := m.MkFalse()
			astTestMust(t, err)
			yes, err := m.MkTrue()
			astTestMust(t, err)
			for i := 0; i < 2; i++ {
				term := yes
				if i == 0 && !tc.common {
					term = no
				}
				set, err := qm.modelFinder.instantiationSet(q, uint32(i), true)
				astTestMust(t, err)
				_, err = set.Insert(term, uint32(i+2))
				astTestMust(t, err)
				astTestMust(t, set.MakeInverse(func(ast.ASTHandle) (ast.ASTHandle, error) { return yes, nil }))
			}
			md, err := protomodel.NewProtoModel(m, config)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, md.Close()) })
			raw := []ast.ASTHandle{no, yes}
			bindings := []ast.ASTHandle{no, yes}
			gen, err := qm.modelChecker.tryRepeatedAuxiliaryBinding(md, q, raw, bindings, 9)
			astTestMust(t, err)
			want, wantGen := no, uint32(9)
			if tc.replace {
				want, wantGen = yes, 3
			}
			if bindings[0] != want || bindings[1] != yes || gen != wantGen || raw[0] != no {
				t.Fatalf("got bindings %v, generation %d and raw values %v", bindings, gen, raw)
			}
		})
	}
}
