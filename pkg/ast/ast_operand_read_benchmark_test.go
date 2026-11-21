package ast

import (
	"strconv"
	"testing"
)

func BenchmarkASTOperandRead(b *testing.B) {
	for _, arity := range []int{1, 2, 4, 8, 16} {
		b.Run(strconv.Itoa(arity), func(b *testing.B) {
			m := astTestManager(b)
			sort, err := m.MkBoolSort()
			astTestMust(b, err)
			dom, args := make([]ASTHandle, arity), make([]ASTHandle, arity)
			for i := range dom {
				dom[i] = sort
				args[i], err = m.MkVar(uint32(i), sort)
				astTestMust(b, err)
				astTestPin(b, m, args[i])
			}
			decl, err := m.MkFuncDecl(astTestSymbol(b, m, "operand-read"), dom, sort, ASTFuncDeclInfo{})
			astTestMust(b, err)
			term, err := m.MkApp(decl, args)
			astTestMust(b, err)
			astTestPin(b, m, term)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				k := i & (arity - 1)
				h, err := m.GetArg(term, uint32(k))
				if err != nil || h != args[k] {
					b.Fatal(k, err)
				}
			}
		})
	}
}
