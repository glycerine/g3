package ast

import (
	"strconv"
	"testing"
)

// Quantifier instantiation repeatedly builds short applications that already
// exist. Include arity four to exercise the separately allocated argument array.
func BenchmarkASTCanonicalApplication(b *testing.B) {
	for _, arity := range []int{1, 2, 3, 4} {
		b.Run(strconv.Itoa(arity), func(b *testing.B) {
			m := astTestManager(b)
			sort, err := m.MkBoolSort()
			astTestMust(b, err)
			domain, args := make([]ASTHandle, arity), make([]ASTHandle, arity)
			for i := range args {
				domain[i] = sort
				args[i], err = m.MkVar(uint32(i), sort)
				astTestMust(b, err)
			}
			decl, err := m.MkFuncDecl(astTestSymbol(b, m, "canonical-app"), domain, sort, ASTFuncDeclInfo{})
			astTestMust(b, err)
			want, err := m.MkApp(decl, args)
			astTestMust(b, err)
			astTestPin(b, m, want)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if got, err := m.MkApp(decl, args); err != nil || got != want {
					b.Fatalf("MkApp returned %v, %v; want %v", got, err, want)
				}
			}
		})
	}
}
