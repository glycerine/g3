package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Each shift visits a shared application below a binder. The result must
// match an independently constructed expression with the expected indices.
func BenchmarkQuantifiedVariableShift(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	decl, err := m.MkFuncDecl(usymbol.Numerical(200), []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(b, err)
	app := func(x, y ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(decl, []ast.ASTHandle{x, y})
		astTestMust(b, err)
		return h
	}
	variable := func(index uint32) ast.ASTHandle {
		h, err := m.MkVar(index, sort)
		astTestMust(b, err)
		return h
	}
	var bodies [32]ast.ASTHandle
	for i := range bodies {
		shared := app(variable(uint32(i)+1), variable(uint32(i)+2))
		body := app(shared, app(variable(0), shared))
		q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{sort},
			Names: []usymbol.Symbol{usymbol.Numerical(201)}, Body: body,
		})
		astTestMust(b, err)
		bodies[i] = q
		astTestPin(b, m, q)
	}
	out := astTestPin(b, m, ast.ASTHandle{})
	forward, err := newVarShifter(m)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, forward.Close()) })
	inverse, err := newInvVarShifter(m)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, inverse.Close()) })
	for _, mode := range []string{"Forward", "Inverse"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				offset := uint32(i%(len(bodies)-1) + 1)
				want := bodies[offset]
				var err error
				if mode == "Forward" {
					err = forward.ShiftBy(bodies[0], offset, out)
				} else {
					err = inverse.InverseShift(bodies[offset], offset, out)
					want = bodies[0]
				}
				if err != nil {
					b.Fatal(err)
				}
				if got, err := m.RefGet(out); err != nil || got != want {
					b.Fatalf("shift %d gave %v, want %v (err %v)", offset, got, want, err)
				}
			}
		})
	}
}
