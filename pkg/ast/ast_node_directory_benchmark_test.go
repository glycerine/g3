package ast

import (
	"strconv"
	"testing"
)

func BenchmarkASTNodeTraversal(b *testing.B) {
	for _, population := range []int{1, 16, 256, 4096} {
		b.Run(strconv.Itoa(population), func(b *testing.B) {
			m := astTestManager(b)
			sort, err := m.MkBoolSort()
			astTestMust(b, err)
			predicate, err := m.MkFuncDecl(astTestSymbol(b, m, "p"), []ASTHandle{sort, sort}, sort, ASTFuncDeclInfo{})
			astTestMust(b, err)
			apps := make([]ASTHandle, population)
			children := make([]ASTHandle, population)
			for i := range apps {
				children[i], err = m.MkVar(uint32(i), sort)
				astTestMust(b, err)
				apps[i], err = m.MkApp(predicate, []ASTHandle{children[i], children[i]})
				astTestMust(b, err)
				astTestPin(b, m, apps[i])
			}
			b.ReportAllocs()
			b.ResetTimer()
			var index uint32
			for i := 0; i < b.N; i++ {
				index = index*1664525 + 1013904223
				selected := int(index & uint32(population-1))
				app := apps[selected]
				kind, _, arity, err := m.NodeShape(app)
				if err != nil || kind != ASTApp || arity != 2 {
					b.Fatal(kind, arity, err)
				}
				got, err := m.GetArg(app, 1)
				if err != nil || got != children[selected] {
					b.Fatal(got, err)
				}
			}
		})
	}
}
