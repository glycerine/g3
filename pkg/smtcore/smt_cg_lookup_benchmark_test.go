package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func BenchmarkSMTCongruenceProbes(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkSort(symbol.Numerical(90300), ast.ASTSortInfo{})
	astTestMust(b, err)
	const size = 128
	var handles [size]ast.ASTHandle
	var roots [size]*SMTEnode
	for i := range roots {
		handles[i], roots[i] = smtEnodeConstant(b, m, uint32(90400+i), sort, 0)
	}
	decl := smtCGTestDeclaration(b, m, 90600, sort, ast.ASTFuncFlags{Commutative: true})
	table, err := NewSMTCongruenceTable(m)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, table.Close()) })
	var apps [size]*SMTEnode
	for i := range roots {
		j := (i + 1) % size
		apps[i] = smtCGTestApplication(b, m, decl, []ast.ASTHandle{handles[i], handles[j]}, []*SMTEnode{roots[i], roots[j]})
		_, err = table.Insert(apps[i])
		astTestMust(b, err)
	}
	for _, c := range []string{"Ordered", "Commuted", "Miss", "PositionalMiss", "Find", "EraseInsert", "Singleton"} {
		b.Run(c, func(b *testing.B) {
			var args [2]*SMTEnode
			b.ReportAllocs()
			small := table
			if c == "Singleton" {
				small, err = NewSMTCongruenceTable(m)
				astTestMust(b, err)
				_, err = small.Insert(apps[0])
				astTestMust(b, err)
				defer func() { astTestMust(b, small.Close()) }()
			}
			b.ResetTimer()
			for k := 0; k < b.N; k++ {
				i := k % size
				a := apps[i]
				switch c {
				case "Find":
					found, swap, err := table.Find(a)
					if err != nil || found != a || swap {
						b.Fatal(err)
					}
					continue
				case "EraseInsert":
					ok, err := table.Erase(a)
					if !ok || err != nil {
						b.Fatal(err)
					}
					_, err = table.Insert(a)
					astTestMust(b, err)
					continue
				case "Singleton":
					i = 0
					a = apps[0]
				}
				args[0], args[1] = roots[i], roots[(i+1)%size]
				if c == "Commuted" || c == "PositionalMiss" {
					args[0], args[1] = args[1], args[0]
				}
				if c == "Miss" {
					args[1] = roots[(i+3)%size]
				}
				found, err := small.FindApp(decl, args[:], c != "PositionalMiss")
				if c == "Miss" || c == "PositionalMiss" {
					a = nil
				}
				if err != nil || a != found {
					b.Fatal(c, i, err)
				}
			}
		})
	}
}
