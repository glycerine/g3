package pdr

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPDRDisequalityReductionUsesSourceSwapRemovalOrder(t *testing.T) {
	for _, name := range []string{"guard-first", "guard-last", "below-threshold"} {
		t.Run(name, func(t *testing.T) {
			m, oldOwner, transformer, _, closeBase := closureFixture(t)
			config := oldOwner.contexts.config
			closureMust(t, transformer.Close())
			closureMust(t, oldOwner.Close())
			c, err := NewContext(m, nil, 2, ReachableNoCache, config)
			closureMust(t, err)
			defer func() { closureMust(t, c.Close()); closeBase() }()
			var refs []ast.ASTRef
			defer func() { closureMust(t, closeASTRefs(m, refs)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				closureMust(t, err)
				ref, err := m.NewRef(h)
				closureMust(t, err)
				refs = append(refs, ref)
				return h
			}
			seven := own(m.ArithmeticRealNumeralText("7"))
			sort, err := m.GetSort(seven)
			closureMust(t, err)
			x := own(m.MkFreshConst("disequality-order-x", sort))
			boolean := own(m.MkBoolSort())
			q := own(m.MkFreshConst("disequality-order-guard", boolean))
			diseqs := make([]ast.ASTHandle, 3)
			for i := range diseqs {
				value := own(m.ArithmeticRealNumeralText(strconv.Itoa(i)))
				diseqs[i] = own(m.MkNot(own(m.MkEq(x, value))))
			}
			args := append([]ast.ASTHandle{q}, diseqs...)
			definition := own(m.MkEq(x, seven))
			expected := []ast.ASTHandle{definition, q}
			if name == "guard-last" {
				args = append(append([]ast.ASTHandle(nil), diseqs...), q)
				expected = []ast.ASTHandle{q, definition}
			}
			if name == "below-threshold" {
				args = []ast.ASTHandle{q, diseqs[0], diseqs[1]}
				expected = []ast.ASTHandle{diseqs[1], q, diseqs[0]}
			}
			input := own(m.MkAndArray(args))
			md, err := model.NewModel(m)
			closureMust(t, err)
			defer func() { closureMust(t, md.Close()) }()
			for _, entry := range []struct{ variable, value ast.ASTHandle }{{x, seven}, {q, own(m.MkTrue())}} {
				decl, err := m.GetDecl(entry.variable)
				closureMust(t, err)
				closureMust(t, md.Core().RegisterConstant(decl, entry.value))
			}
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
			closureMust(t, diagnostic.SetProcessVerbosityLevel(2))
			result, err := c.reduceDisequalities(md, 3, input)
			closureMust(t, err)
			defer func() { closureMust(t, m.DelRef(&result)) }()
			message, err := stream.String()
			closureMust(t, err)
			expectedMessage := "Deleted 3 disequalities 2 conjuncts\n"
			if name == "below-threshold" {
				expectedMessage = ""
			}
			if message != expectedMessage {
				t.Fatal("source reduction diagnostics", message, expectedMessage)
			}

			root, err := m.RefGet(result)
			closureMust(t, err)
			count, err := m.GetNumArgs(root)
			closureMust(t, err)
			if int(count) != len(expected) {
				t.Fatal("reduced conjunction size", count, len(expected))
			}
			for i, want := range expected {
				got, err := m.GetArg(root, uint32(i))
				closureMust(t, err)
				if got != want {
					t.Fatal("source conjunct order", i, got, want)
				}
			}
		})
	}
}
