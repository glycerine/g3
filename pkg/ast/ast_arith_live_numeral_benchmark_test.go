package ast

import (
	"strconv"
	"testing"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// BenchmarkArithmeticLiveNumeral keeps the first result alive while repeating
// the rewriter's construction request. The plugin caches integers 0 through 15.
func BenchmarkArithmeticLiveNumeral(b *testing.B) {
	for _, n := range []int32{-1, 1, 16} {
		b.Run(strconv.Itoa(int(n)), func(b *testing.B) {
			m := astTestManager(b)
			value := unumeral.Rational{Num: unumeral.NewInt(n), Den: unumeral.NewInt(1)}
			want, err := m.ArithmeticNumeralFromRational(&value, true)
			astTestMust(b, err)
			astTestPin(b, m, want)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h, err := m.ArithmeticNumeralFromRational(&value, true)
				if err != nil || h != want {
					b.Fatalf("repeated numeral %d = %v, %v; want %v", n, h, err, want)
				}
			}
		})
	}
}

// BenchmarkDeclInfoParameters measures copying and releasing declaration
// parameters, including declarations wider than an arithmetic numeral.
func BenchmarkDeclInfoParameters(b *testing.B) {
	for _, width := range []int{0, 1, 2, 3, 8, 64} {
		b.Run(strconv.Itoa(width), func(b *testing.B) {
			m := astTestManager(b)
			input := make([]ASTParameter, width)
			for i := range input {
				var err error
				input[i], err = m.NewParameterInt(int32(i))
				astTestMust(b, err)
			}
			b.Cleanup(func() {
				for i := range input {
					astTestMust(b, m.DelParameter(&input[i]))
				}
			})
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				info, err := m.NewFuncDeclInfo(4, 0, input, ASTFuncFlags{}, false)
				if err != nil {
					b.Fatal(err)
				}
				if err := m.DelFuncDeclInfo(&info); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
