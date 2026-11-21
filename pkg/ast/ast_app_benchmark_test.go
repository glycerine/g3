package ast

import (
	"strconv"
	"testing"
)

// Quantifier instantiation repeatedly builds short applications that already
// exist. Include arity four to exercise the separately allocated argument array.
func BenchmarkASTCanonicalApplication(b *testing.B) {
	for _, arity := range []int{1, 2, 3, 4} {
		b.Run(strconv.Itoa(arity), func(b *testing.B) {
			m := astTestManager(b)
			sort, err := m.MkBoolSort()
			astTestMust(b, err)
			domain, args := make([]ASTHandle, arity), make([]ASTHandle, arity)
			for i := range args {
				domain[i] = sort
				args[i], err = m.MkVar(uint32(i), sort)
				astTestMust(b, err)
			}
			decl, err := m.MkFuncDecl(astTestSymbol(b, m, "canonical-app"), domain, sort, ASTFuncDeclInfo{})
			astTestMust(b, err)
			want, err := m.MkApp(decl, args)
			astTestMust(b, err)
			astTestPin(b, m, want)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if got, err := m.MkApp(decl, args); err != nil || got != want {
					b.Fatalf("MkApp returned %v, %v; want %v", got, err, want)
				}
			}
		})
	}
}

// Bound variables with the same de Bruijn index and sort are canonical.
func BenchmarkASTCanonicalVariable(b *testing.B) {
	for _, count := range []int{8, 256, 4096} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			m := astTestManager(b)
			sort, err := m.MkBoolSort()
			astTestMust(b, err)
			want := make([]ASTHandle, count)
			for i := range want {
				want[i], err = m.MkVar(uint32(i), sort)
				astTestMust(b, err)
				astTestPin(b, m, want[i])
			}
			b.ReportAllocs()
			b.ResetTimer()
			var position uint32
			for i := 0; i < b.N; i++ {
				position = position*1664525 + 1013904223
				index := position & uint32(count-1)
				got, err := m.MkVar(index, sort)
				if err != nil || got != want[index] {
					b.Fatalf("index %d returned %v, %v", index, got, err)
				}
			}
		})
	}
}

// Rebuild applications over 256 bindings. Retained applications already have
// an owner. Each fresh result loses its last reference in the timed loop.
func BenchmarkASTApplicationReuse(b *testing.B) {
	const width = 256
	for _, arity := range []int{1, 2, 3, 4} {
		for _, resident := range []int{0, 192, 256} {
			b.Run(strconv.Itoa(arity)+"/Resident"+strconv.Itoa(resident), func(b *testing.B) {
				m := astTestManager(b)
				boolean, err := m.MkBoolSort()
				astTestMust(b, err)
				variables := make([]ASTHandle, width+arity)
				for i := range variables {
					variables[i], err = m.MkVar(uint32(i), boolean)
					astTestMust(b, err)
					astTestPin(b, m, variables[i])
				}
				domain, args := make([]ASTHandle, arity), make([]ASTHandle, arity)
				for i := range domain {
					domain[i], args[i] = boolean, variables[width+i]
				}
				decl, err := m.MkFuncDecl(astTestSymbol(b, m, "canonical-app"), domain, boolean, ASTFuncDeclInfo{})
				astTestMust(b, err)
				astTestPin(b, m, decl)
				roots := make([]ASTHandle, resident)
				for i := range roots {
					args[0] = variables[i]
					roots[i], err = m.MkApp(decl, args)
					astTestMust(b, err)
					astTestPin(b, m, roots[i])
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					index := (uint(i) * 197) & (width - 1)
					args[0] = variables[index]
					result, err := m.MkApp(decl, args)
					if err != nil || (int(index) < resident && result != roots[index]) {
						b.Fatal(index, result, err)
					}
					if err = m.IncRef(result); err != nil {
						b.Fatal(err)
					}
					if err = m.DecRef(result); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
