package ast

import (
	"strconv"
	"testing"
)

// Four trees share declarations and overlapping leaves. Reset once per tree
// or once per group of four to measure both cache lifetimes.
func BenchmarkASTTranslationSiblings(b *testing.B) {
	for _, width := range []int{8, 128, 1024} {
		for _, reuse := range []bool{false, true} {
			b.Run(strconv.Itoa(width)+"/cache-"+strconv.FormatBool(reuse), func(b *testing.B) {
				from, to := astTestManager(b), astTestManager(b)
				boolean, err := from.MkBoolSort()
				astTestMust(b, err)
				decl, err := from.MkFuncDecl(astTestSymbol(b, from, "binary"), []ASTHandle{boolean, boolean}, boolean, ASTFuncDeclInfo{})
				astTestMust(b, err)
				var roots []ASTHandle
				for j := 0; j < 4; j++ {
					var level []ASTHandle
					for i := 0; i < width; i++ {
						left, err := from.MkConst(astTestSymbol(b, from, "c"+strconv.Itoa(i)), boolean)
						astTestMust(b, err)
						right, err := from.MkConst(astTestSymbol(b, from, "d"+strconv.Itoa(j+i%11)), boolean)
						astTestMust(b, err)
						leaf, err := from.MkApp(decl, []ASTHandle{left, right})
						astTestMust(b, err)
						level = append(level, leaf)
					}
					for len(level) > 1 {
						var next []ASTHandle
						for i := 0; i < len(level); i += 2 {
							parent, err := from.MkAnd(level[i], level[i+1])
							astTestMust(b, err)
							next = append(next, parent)
						}
						level = next
					}
					roots = append(roots, level[0])
					astTestPin(b, from, level[0])
				}
				translation, err := NewASTTranslation(from, to, false)
				astTestMust(b, err)
				defer func() { astTestMust(b, translation.Close()) }()
				out, err := to.NewRef(ASTHandle{})
				astTestMust(b, err)
				defer func() { astTestMust(b, to.DelRef(&out)) }()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if !reuse || i%len(roots) == 0 {
						astTestMust(b, translation.ResetCache())
					}
					h, err := translation.Translate(roots[i%len(roots)])
					astTestMust(b, err)
					astTestMust(b, to.SetRef(out, h))
				}
				b.StopTimer()
				got, err := to.RefGet(out)
				astTestMust(b, err)
				n, err := to.GetNumArgs(got)
				astTestMust(b, err)
				if n != 2 {
					b.Fatalf("translated root has %d children", n)
				}
			})
		}
	}
}
