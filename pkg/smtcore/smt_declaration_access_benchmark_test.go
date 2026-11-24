package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Time registration and the actual first and repeated declaration lookups of
// a fresh solver. Each lookup checks the contents, including suppressed terms.
func BenchmarkSMTDeclarationIndexAccess(b *testing.B) {
	const width = 512
	for _, count := range []int{1, 64, 512} {
		for _, mode := range []string{"NoRead", "MatchOne", "MatchEvery", "BothEvery", "Interleaved"} {
			b.Run(fmt.Sprintf("%d/%s", count, mode), func(b *testing.B) {
				f := newSMTEqualityUndoFixture(b, "decl-access", width/count)
				decls := make([]ast.ASTHandle, count)
				for i := range decls {
					d, e := f.m.MkFuncDecl(astTestSymbol(b, f.m, fmt.Sprintf("decl-access-%d", i)), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
					astTestMust(b, e)
					decls[i] = d
				}
				var terms [width]ast.ASTHandle
				for j := range terms {
					h, e := f.m.MkApp(decls[j%count], []ast.ASTHandle{f.base[j/count]})
					astTestMust(b, e)
					astTestPin(b, f.m, h)
					terms[j] = h
				}
				b.ReportAllocs()
				b.ResetTimer()
				b.StopTimer()
				for i := 0; i < b.N; i++ {
					_, _, s := smtInternalizerFixture(b, f.m, nil)
					for _, x := range f.base {
						astTestMust(b, s.Internalize(x, false))
					}
					astTestMust(b, s.PushScope())
					it := newSMTMAMInterpreter(s, nil, false)
					var hits [width]int
					check := func(k, num int, both bool) {
						ds := it.enodesOf(decls[k])
						if len(ds) != hits[k] {
							b.Fatalf("declaration %d has %d candidates, want %d", k, len(ds), hits[k])
						}
						if both {
							if n := len(s.context.declarationEnodes(decls[k])); n != num {
								b.Fatalf("declaration %d has %d terms, want %d", k, n, num)
							}
						}
					}
					b.StartTimer()
					for j, term := range terms {
						suppress := j%5 == 0
						k := j % count
						_, e := s.MkEnode(term, suppress, false, true)
						astTestMust(b, e)
						if !suppress {
							hits[k]++
						}
						if mode == "Interleaved" {
							check(k, j/count+1, true)
						}
					}
					switch mode {
					case "MatchOne":
						check(0, width/count, false)
					case "MatchEvery", "BothEvery":
						for round := 0; round < 3; round++ {
							for k := range decls {
								check(k, width/count, mode == "BothEvery")
							}
						}
					}
					astTestMust(b, s.PopScopes(1))
					b.StopTimer()
					if mode != "NoRead" {
						for k := range decls {
							if len(it.enodesOf(decls[k])) != 0 {
								b.Fatal("retired candidate remained")
							}
						}
					}
					astTestMust(b, s.Close())
				}
			})
		}
	}
}
