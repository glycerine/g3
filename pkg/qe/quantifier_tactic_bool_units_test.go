package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierBoolUnitsEliminateExistentialAndUniversal(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	v := f.own(m.MkVar(0, f.boolean))
	r := f.constant("quant-unit-free", f.boolean)
	conjunction := f.own(m.MkAnd(v, r))
	xor := f.own(m.MkXor(v, r))
	for _, tc := range []struct {
		name     string
		body     ast.ASTHandle
		forall   bool
		want     ast.ASTHandle
		complete bool
	}{
		{"exists", v, false, f.own(m.MkTrue()), true},
		{"forall", v, true, f.own(m.MkFalse()), true},
		{"free", conjunction, false, r, true},
		{"needs-branch", xor, false, ast.ASTHandle{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: tc.forall, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: tc.body}))
			result, complete, err := eliminateQuantifierBoolUnits(m, quantifier, f.r.config)
			fmMust(t, err)
			if complete != tc.complete {
				t.Fatal("unexpected completion status", complete)
			}
			if !complete {
				if result != (ast.ASTRef{}) {
					t.Fatal("unsupported case returned a result")
				}
				return
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			if value != tc.want {
				t.Fatal("incorrect quantifier result")
			}
		})
	}
}
