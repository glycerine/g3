package ast

import (
	"testing"
	"unsafe"
)

//go:noinline
func readStudyArgument(args AppArguments) (ASTHandle, error) { return args.At(1) }

func BenchmarkBorrowedArgumentRead(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	a, err := m.MkConst(astTestSymbol(b, m, "a"), sort)
	astTestMust(b, err)
	astTestPin(b, m, a)
	c, err := m.MkConst(astTestSymbol(b, m, "c"), sort)
	astTestMust(b, err)
	astTestPin(b, m, c)
	app, err := m.MkOr(a, c)
	astTestMust(b, err)
	astTestPin(b, m, app)
	values := []ASTHandle{a, c}
	view, err := m.AppArgumentView(app)
	astTestMust(b, err)
	for _, name := range []string{"Slice", "App", "CreateApp"} {
		b.Run(name, func(b *testing.B) {
			args := view
			if name == "Slice" {
				args = SliceArguments(values)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if name == "CreateApp" {
					v, e := m.AppArgumentView(app)
					if e != nil {
						b.Fatal(e)
					}
					args = v
				}
				got, e := readStudyArgument(args)
				if e != nil || got != c {
					b.Fatal(got, e)
				}
			}
			b.ReportMetric(float64(unsafe.Sizeof(args)), "B/view")
		})
	}
}
