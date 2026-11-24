package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Two assertions use the same deep term. The second assertion should look up
// that term without walking all the other live Boolean variables.
func BenchmarkSMTDeepSharedAssertion(b *testing.B) {
	for _, unrelated := range []int{0, 1024, 16384} {
		b.Run(fmt.Sprintf("Unrelated%d", unrelated), func(b *testing.B) {
			m := astTestManager(b)
			boolean, err := m.MkBoolSort()
			astTestMust(b, err)
			info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
			astTestMust(b, err)
			wrapper, err := m.MkFuncDeclWithInfo(astTestSymbol(b, m, "f"), []ast.ASTHandle{boolean}, boolean, info)
			astTestMust(b, err)
			astTestMust(b, m.DelFuncDeclInfo(&info))
			shared := smtRelevancyBool(b, m, "a")
			for depth := uint32(0); depth <= smtDeepExpressionThreshold; depth++ {
				shared, err = m.MkApp(wrapper, []ast.ASTHandle{shared})
				astTestMust(b, err)
			}
			_, _, state := smtInternalizerFixture(b, m, nil)
			astTestMust(b, state.Assert(shared, ast.ASTHandle{}, 0))
			for i := 0; i < unrelated; i++ {
				h := smtRelevancyBool(b, m, fmt.Sprintf("p%d", i))
				astTestMust(b, state.Internalize(h, true))
			}
			next, err := m.MkApp(wrapper, []ast.ASTHandle{shared})
			astTestMust(b, err)
			astTestMust(b, m.IncRef(next))
			b.Cleanup(func() { astTestMust(b, m.DecRef(next)) })
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := state.PushScope(); err != nil {
					b.Fatal(err)
				}
				if err := state.Assert(next, ast.ASTHandle{}, 0); err != nil {
					b.Fatal(err)
				}
				if err := state.PopScopes(1); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			if _, found := state.BoolVar(next); found {
				b.Fatal("scoped assertion survived the pop")
			}
			if _, found := state.BoolVar(shared); !found {
				b.Fatal("shared Boolean disappeared after the pop")
			}
		})
	}
}
