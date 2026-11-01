package pdr

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPDRFragmentRejectionUsesSourceTraversalAndVerbosity(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			closureMust(t, m.DelRef(&refs[i]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		closureMust(t, err)
		ref, err := m.NewRef(h)
		closureMust(t, err)
		refs = append(refs, ref)
		return h
	}
	zero := own(m.ArithmeticRealNumeralText("0"))
	sort, err := m.GetSort(zero)
	closureMust(t, err)
	name, err := m.Symbols.Intern("rejection-x")
	closureMust(t, err)
	x := own(m.MkConst(name, sort))
	bad := func(name string) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(name)
		closureMust(t, err)
		decl := own(m.MkFuncDecl(symbol, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{}))
		return own(m.MkApp(decl, []ast.ASTHandle{x}))
	}
	firstTerm, secondTerm := bad("rejection-first"), bad("rejection-second")
	first, second := own(m.MkEq(firstTerm, zero)), own(m.MkEq(secondTerm, zero))
	conjunction := own(m.MkAndArray([]ast.ASTHandle{first, second}))
	truth := own(m.MkTrue())
	quantified := own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{name}, Body: truth, NoPatterns: []ast.ASTHandle{firstTerm, secondTerm}}))
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	closureMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	closureMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	closureMust(t, err)
	defer func() {
		closureMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		closureMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		closureMust(t, stream.Close())
	}()
	closureMust(t, diagnostic.SetProcessVerboseStream(stream))
	for _, utvpi := range []bool{false, true} {
		prefix := "non-diff: "
		if utvpi {
			prefix = "non-utvpi: "
		}
		for _, level := range []uint32{0, 1} {
			closureMust(t, diagnostic.SetProcessVerbosityLevel(level))
			for _, tc := range []struct {
				forms []ast.ASTHandle
				term  string
			}{
				{[]ast.ASTHandle{first, second}, "rejection-first"},
				{[]ast.ASTHandle{second, first}, "rejection-second"},
				{[]ast.ASTHandle{conjunction}, "rejection-first"},
				{[]ast.ASTHandle{quantified}, "rejection-first"},
			} {
				closureMust(t, stream.ResetText())
				accepted, err := classifyArithmeticFragment(m, tc.forms, utvpi)
				closureMust(t, err)
				if accepted {
					t.Fatal("uninterpreted arithmetic function accepted")
				}
				got, err := stream.String()
				closureMust(t, err)
				want := ""
				if level != 0 {
					want = prefix + "(" + tc.term + " rejection-x)\n"
				}
				if got != want {
					t.Fatalf("utvpi=%v level=%d: got %q, want %q", utvpi, level, got, want)
				}
				formats, err := m.GetFormatManager()
				closureMust(t, err)
				if formats.Stats().LiveReferences != 0 {
					t.Fatal("rejection printer leaked a format reference")
				}
			}
		}
	}
}
