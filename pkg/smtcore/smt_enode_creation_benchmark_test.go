package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func BenchmarkSMTEnodeCreation(b *testing.B) {
	f := newSMTEqualityUndoFixture(b, "create", 16)
	for _, n := range []int{0, 1, 2, 3, 4, 8, 16} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			domain := make([]ast.ASTHandle, n)
			for k := range domain {
				domain[k] = f.sort
			}
			decl, e := f.m.MkFuncDecl(astTestSymbol(b, f.m, "arity"+strconv.Itoa(n)), domain, f.sort, ast.ASTFuncDeclInfo{})
			astTestMust(b, e)
			term, e := f.m.MkApp(decl, f.base[:n])
			astTestMust(b, e)
			astTestPin(b, f.m, term)
			byApp := f.s.context.enodes.get
			parents := make([]uint32, n)
			for i, h := range f.base[:n] {
				parents[i] = byApp(h).NumParents()
			}
			options := SMTEnodeOptions{CongruenceEnabled: true, UpdateChildrenParent: true}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				node, e := NewSMTEnode(f.m, term, byApp, options)
				if e != nil {
					b.Fatal(e)
				}
				if node.NumArgs() != uint32(n) || node.declaration != decl {
					b.Fatal("invalid application")
				}
				if e = node.Delete(true); e != nil {
					b.Fatal(e)
				}
			}
			b.StopTimer()
			for i, h := range f.base[:n] {
				if got := byApp(h).NumParents(); got != parents[i] {
					b.Fatalf("argument %d has %d parents after deletion, want %d", i, got, parents[i])
				}
			}
		})
	}
}
