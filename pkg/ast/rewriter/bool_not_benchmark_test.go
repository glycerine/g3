package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func BenchmarkBoolNegationOperand(b *testing.B) {
	m := astTestManager(b)
	rw := boolRewriter{manager: m}
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	atom, err := m.MkConst(astTestSymbol(b, m, "p"), sort)
	astTestMust(b, err)
	astTestPin(b, m, atom)
	negated, err := m.MkNot(atom)
	astTestMust(b, err)
	astTestPin(b, m, negated)
	truth, err := m.MkTrue()
	astTestMust(b, err)
	falsity, err := m.MkFalse()
	astTestMust(b, err)
	for _, tc := range []struct {
		name      string
		arg, want ast.ASTHandle
		status    brStatus
	}{
		{"Atom", atom, truth, brFailed},
		{"Negated", negated, atom, brDone},
		{"True", truth, falsity, brDone},
	} {
		b.Run(tc.name, func(b *testing.B) {
			out := astTestPin(b, m, truth)
			b.ReportAllocs()
			for b.Loop() {
				status, err := rw.mkNotCore(tc.arg, out)
				if status != tc.status || err != nil {
					b.Fatalf("negation returned %v, %v", status, err)
				}
			}
			got, err := m.RefGet(out)
			astTestMust(b, err)
			if got != tc.want {
				b.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
