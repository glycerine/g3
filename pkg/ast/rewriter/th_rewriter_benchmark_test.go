package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

//go:noinline
func blankTheoryReduction(_ *theoryRewriterConfiguration, _ ast.ASTHandle, _ ast.AppArguments, _ ast.ASTRef) (RewriteStatus, error) {
	return RewriteFailed, nil
}

// BenchmarkTheoryReduction calls the same reducer as processApp. Its argument
// slice and output reference stay live across calls, just as they do in traversal.
func BenchmarkTheoryReduction(b *testing.B) {
	m, c, g, p := boolRewriterConfigFixture(b)
	rw, err := NewTheoryRewriter(m, c, g, p)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, rw.Close()) })
	hold := func(h ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(b, err)
		astTestPin(b, m, h)
		return h
	}
	s := hold(m.MkBoolSort())
	x := hold(m.MkConst(astTestSymbol(b, m, "x"), s))
	y := hold(m.MkConst(astTestSymbol(b, m, "y"), s))
	f := hold(m.MkFuncDecl(astTestSymbol(b, m, "f"), []ast.ASTHandle{s, s}, s, ast.ASTFuncDeclInfo{}))
	u := hold(m.MkUninterpretedSort(astTestSymbol(b, m, "U"), nil))
	left := hold(m.MkConst(astTestSymbol(b, m, "left"), u))
	right := hold(m.MkConst(astTestSymbol(b, m, "right"), u))
	neg := hold(m.MkNot(x))
	out := astTestPin(b, m, ast.ASTHandle{})
	for _, tc := range []struct {
		name string
		app  ast.ASTHandle
		args []ast.ASTHandle
		want ast.ASTHandle
	}{
		{"UF", hold(m.MkApp(f, []ast.ASTHandle{x, y})), []ast.ASTHandle{x, y}, ast.ASTHandle{}},
		{"Not", neg, []ast.ASTHandle{x}, ast.ASTHandle{}},
		{"DoubleNot", hold(m.MkNot(neg)), []ast.ASTHandle{neg}, x},
		{"Or", hold(m.MkOr(x, y)), []ast.ASTHandle{x, y}, ast.ASTHandle{}},
		{"OrDuplicate", hold(m.MkOr(x, x)), []ast.ASTHandle{x, x}, x},
		{"UninterpretedEquality", hold(m.MkEq(left, right)), []ast.ASTHandle{left, right}, ast.ASTHandle{}},
		{"Iff", hold(m.MkIff(x, y)), []ast.ASTHandle{x, y}, ast.ASTHandle{}},
	} {
		decl, err := m.GetDecl(tc.app)
		astTestMust(b, err)
		args := ast.SliceArguments(tc.args)
		for _, blank := range []bool{false, true} {
			reduce := (*theoryRewriterConfiguration).reduceAppCore
			name := tc.name
			want := tc.want
			if blank {
				reduce = blankTheoryReduction
				name += "/Blank"
				want = ast.ASTHandle{}
			} else {
				name += "/Core"
			}
			b.Run(name, func(b *testing.B) {
				astTestMust(b, m.SetRef(out, ast.ASTHandle{}))
				status := RewriteFailed
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					status, err = reduce(&rw.configuration, decl, args, out)
					if err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				got, err := m.RefGet(out)
				astTestMust(b, err)
				wantStatus := RewriteFailed
				if !want.IsNull() {
					wantStatus = RewriteDone
				}
				if status != wantStatus || got != want {
					b.Fatalf("status %v, result %v, want %v, %v", status, got, wantStatus, want)
				}
			})
		}
	}
}

// Changed formulas in macro expansion each construct and close a theory rewriter.
func BenchmarkTheoryRewriterConstruction(b *testing.B) {
	for _, name := range []string{"Defaults", "LocalAndGlobal"} {
		b.Run(name, func(b *testing.B) {
			m, context, globals, local := boolRewriterConfigFixture(b)
			if name == "LocalAndGlobal" {
				astTestMust(b, globals.SourceSetName("rewriter.som", "true"))
				key := "som_blowup"
				astTestMust(b, context.SetUIntName(local, &key, 17))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r, err := NewTheoryRewriter(m, context, globals, local)
				if err != nil {
					b.Fatal(err)
				}
				if err := r.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
