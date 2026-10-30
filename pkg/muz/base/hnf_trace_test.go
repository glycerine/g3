package base_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestHornNormalizerSourceTrace(t *testing.T) {
	for _, withProof := range []bool{false, true} {
		for _, mode := range []string{"disabled", "already-horn", "split", "canceled", "failed-stream"} {
			t.Run(fmt.Sprintf("proof=%v/%s", withProof, mode), func(t *testing.T) {
				f := newRuleSetFixture(t)
				var output bytes.Buffer
				var writer io.Writer = &output
				if mode == "failed-stream" {
					writer = hornTraceRefusingWriter{}
				}
				tags := diagnostic.NewTraceTagsWithWriter(writer)
				m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: f.manager.Symbols, Numerals: f.manager.NumeralService(), Budget: f.manager.Budget(), TraceTags: tags})
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, m.Close()) })
				boolean, err := m.MkBoolSort()
				ruleMust(t, err)
				var atoms []ast.ASTHandle
				for _, name := range []string{"P", "Q", "H"} {
					symbol, err := m.Symbols.Intern(name)
					ruleMust(t, err)
					d, err := m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
					ruleMust(t, err)
					a, err := m.MkApp(d, nil)
					ruleMust(t, err)
					atoms = append(atoms, a)
				}
				body := atoms[0]
				if mode != "already-horn" {
					body, err = m.MkOr(atoms[0], atoms[1])
					ruleMust(t, err)
				}
				formula, err := m.MkImplies(body, atoms[2])
				ruleMust(t, err)
				input, err := m.NewRef(formula)
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, m.DelRef(&input)) })
				n, err := base.NewHornNormalizer(m, "trace")
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, n.Close()) })
				var proof ast.ASTHandle
				if withProof {
					ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
					proof, err = m.MkProofAsserted(formula)
					ruleMust(t, err)
				}
				if mode != "disabled" {
					tags.Enable("hnf")
				}
				if mode == "canceled" {
					m.Limit.Cancel()
					defer m.Limit.ResetCancel()
				}
				var result *base.HornNormalization
				if withProof {
					result, err = n.NormalizeWithProof(formula, proof)
				} else {
					result, err = n.Normalize(formula)
				}
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, result.Close()) })
				wantCount := 2
				if mode == "already-horn" {
					wantCount = 1
				}
				if mode == "canceled" {
					wantCount = 0
				}
				if len(result.Formulas) != wantCount {
					t.Fatal("trace changed normalization", len(result.Formulas), wantCount)
				}
				if !tags.ShouldTrace("hnf") || mode == "failed-stream" {
					if output.Len() != 0 {
						t.Fatal("disabled/refused trace emitted output", output.String())
					}
					return
				}
				record := func(function string, line int, body string) string {
					return fmt.Sprintf("-------- [hnf] %s src/muz/base/hnf.cpp:%d ---------\n%s------------------------------------------------\n", function, line, body)
				}
				var want string
				switch mode {
				case "already-horn":
					want = record("operator()", 524, "(=> P H)\nHNF result:\n(=> P H)\n")
				case "canceled":
					want = record("operator()", 168, "(=> (or P Q) H)\n==>\n") + record("operator()", 524, "(=> (or P Q) H)\nHNF result:\n")
				default:
					want = record("eliminate_disjunctions", 337, "Q\n") + record("eliminate_disjunctions", 337, "P\n") +
						record("operator()", 168, "(=> (or P Q) H)\n==>\n(=> Q H)\n(=> P H)\n") +
						record("operator()", 524, "(=> (or P Q) H)\nHNF result:\n(=> Q H)\n(=> P H)\n")
				}
				if got := output.String(); got != want {
					t.Fatalf("source trace got %q, want %q", got, want)
				}
			})
		}
	}
}

type hornTraceRefusingWriter struct{}

func (hornTraceRefusingWriter) Write([]byte) (int, error) {
	return 0, errors.New("trace stream refused")
}
