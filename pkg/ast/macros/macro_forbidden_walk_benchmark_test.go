package macros_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
)

// Each group of eight roots shares an application. The wide case also
// exercises unrelated expressions whose arena slots have the same low bits.
func BenchmarkMacroForbiddenWalk(b *testing.B) {
	for _, width := range []int{8, 64, 512} {
		b.Run(fmt.Sprint(width), func(b *testing.B) {
			m := fixture(b)
			owner, err := macros.NewManager(m)
			must(b, err)
			b.Cleanup(func() { must(b, owner.Close()) })
			sort, err := m.MkBoolSort()
			must(b, err)
			f := declaration(b, m, "f", []ast.ASTHandle{sort}, sort)
			var shared ast.ASTHandle
			terms := make([]ast.ASTHandle, width)
			declarations := make([]ast.ASTHandle, width)
			for i := range terms {
				if i%8 == 0 {
					x, err := m.MkVar(uint32(i), sort)
					must(b, err)
					shared, err = m.MkApp(f, []ast.ASTHandle{x})
					must(b, err)
				}
				d := declaration(b, m, fmt.Sprintf("g%d", i), []ast.ASTHandle{sort}, sort)
				declarations[i] = d
				terms[i], err = m.MkApp(d, []ast.ASTHandle{shared})
				must(b, err)
			}
			b.ReportAllocs()
			for b.Loop() {
				must(b, owner.MarkForbidden(terms))
			}
			for _, d := range declarations {
				if !owner.IsForbidden(d) {
					b.Fatal("root declaration was skipped", d)
				}
			}
			if !owner.IsForbidden(f) {
				b.Fatal("shared declaration was skipped")
			}
		})
	}
}
