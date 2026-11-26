package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func BenchmarkSMTAssertedGreaterGroundDepth(b *testing.B) {
	for _, depth := range []int{1, 16, 1024} {
		b.Run(strconv.Itoa(depth), func(b *testing.B) {
			m := astTestManager(b)
			k := &SMTKernel{manager: m}
			sort, err := m.MkSort(astTestSymbol(b, m, "ordering-sort"), ast.ASTSortInfo{})
			astTestMust(b, err)
			decl, err := m.MkFuncDecl(astTestSymbol(b, m, "ordering-app"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
			astTestMust(b, err)
			longer, err := m.MkConst(astTestSymbol(b, m, "ordering-const"), sort)
			astTestMust(b, err)
			var shorter ast.ASTHandle
			for i := 0; i < depth; i++ {
				shorter = longer
				longer, err = m.MkApp(decl, []ast.ASTHandle{shorter})
				astTestMust(b, err)
			}
			astTestMust(b, m.IncRef(longer))
			defer func() { astTestMust(b, m.DecRef(longer)) }()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				greater, err := k.assertedGreater(longer, shorter, make(map[ast.ASTHandle]uint32))
				if err != nil || !greater {
					b.Fatal(greater, err)
				}
			}
		})
	}
}
