package ast

import (
	"strconv"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// A changed quantifier body keeps the binder sorts and names. Measure both
// publication of a new live node and lookup of an already retained result.
func BenchmarkASTQuantifierBodyUpdate(b *testing.B) {
	benchmarkASTQuantifier(b, true)
}

func BenchmarkASTQuantifierConstruction(b *testing.B) {
	benchmarkASTQuantifier(b, false)
}

func benchmarkASTQuantifier(b *testing.B, update bool) {
	for _, binders := range []int{1, 2, 4, 8, 64} {
		b.Run(strconv.Itoa(binders), func(b *testing.B) {
			m := astTestManager(b)
			boolean, err := m.MkBoolSort()
			astTestMust(b, err)
			body, err := m.MkVar(0, boolean)
			astTestMust(b, err)
			sorts := make([]ASTHandle, binders)
			names := make([]usymbol.Symbol, binders)
			for i := range sorts {
				sorts[i] = boolean
				names[i] = astTestSymbol(b, m, "x"+strconv.Itoa(i))
			}
			root, err := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body})
			astTestMust(b, err)
			astTestPin(b, m, root)
			newBody, err := m.MkNot(body)
			astTestMust(b, err)
			astTestPin(b, m, newBody)
			spec := ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: newBody}
			makeQuantifier := func() (ASTHandle, error) {
				if update {
					return m.UpdateQuantifierBody(root, newBody)
				}
				return m.MkQuantifier(spec)
			}
			for _, name := range []string{"Fresh", "Retained"} {
				b.Run(name, func(b *testing.B) {
					var retained ASTHandle
					if name == "Retained" {
						retained, err = makeQuantifier()
						astTestMust(b, err)
						astTestPin(b, m, retained)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						got, err := makeQuantifier()
						if err != nil || got == root || (!retained.IsNull() && got != retained) {
							b.Fatal(got, err)
						}
						if err := m.IncRef(got); err != nil {
							b.Fatal(err)
						}
						if err := m.DecRef(got); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
