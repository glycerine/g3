//go:build g3trace

package core

import (
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type blastProofTraceWriter struct {
	manager                    *ast.ASTManager
	seen                       bool
	mode                       ast.ASTProofMode
	modeErr                    error
	failOutput, failAllocation bool
}

func (w *blastProofTraceWriter) Write(p []byte) (int, error) {
	if !w.seen {
		w.seen = true
		w.mode, w.modeErr = w.manager.ProofMode()
		if w.failAllocation {
			w.manager.Budget().FailAfter(0)
		}
	}
	if w.failOutput {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func TestStandaloneBlastTermITEProofScopeObservedBySourceTrace(t *testing.T) {
	for _, mode := range []string{"success", "stream-refusal", "allocation-refusal", "borrowed-api"} {
		t.Run(mode, func(t *testing.T) {
			base, _, globals, _ := solveEqFixture(t)
			writer := &blastProofTraceWriter{failOutput: mode == "stream-refusal", failAllocation: mode == "allocation-refusal"}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags, Params: globals.Context, Globals: globals})
			simplifyMust(t, err)
			writer.manager = m
			simplifyMust(t, m.RegisterDeclarationPlugins())
			boolean, err := m.MkBoolSort()
			simplifyMust(t, err)
			arith, err := m.ArithmeticFamilyID()
			simplifyMust(t, err)
			realSort, err := m.MkFamilySort(arith, 0, nil)
			simplifyMust(t, err)
			name, err := m.Symbols.Intern("blast-proof-scope-c")
			simplifyMust(t, err)
			cond, err := m.MkConst(name, boolean)
			simplifyMust(t, err)
			one, err := m.ArithmeticRealNumeralText("1")
			simplifyMust(t, err)
			two, err := m.ArithmeticRealNumeralText("2")
			simplifyMust(t, err)
			choice, err := m.MkIte(cond, one, two)
			simplifyMust(t, err)
			name, err = m.Symbols.Intern("blast-proof-scope-f")
			simplifyMust(t, err)
			decl, err := m.MkFuncDecl(name, []ast.ASTHandle{realSort}, realSort, ast.ASTFuncDeclInfo{})
			simplifyMust(t, err)
			app, err := m.MkApp(decl, []ast.ASTHandle{choice})
			simplifyMust(t, err)
			formula, err := m.MkEq(app, one)
			simplifyMust(t, err)
			input, err := m.NewRef(formula)
			simplifyMust(t, err)
			out, err := m.NewRef(formula)
			simplifyMust(t, err)
			simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			tags.Enable("blast_term_ite")
			if mode == "borrowed-api" {
				_, err = BlastTermITE(m, formula)
			} else {
				err = BlastTermITEInto(m, formula, out)
			}
			m.Budget().FailAfter(-1)
			if mode == "allocation-refusal" {
				if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal("missing trace allocation refusal", err)
				}
			} else {
				simplifyMust(t, err)
			}
			if !writer.seen || writer.modeErr != nil || writer.mode != ast.ASTProofDisabled {
				t.Fatal("source trace did not observe disabled proofs", writer.seen, writer.mode, writer.modeErr)
			}
			after, err := m.ProofMode()
			simplifyMust(t, err)
			if after != ast.ASTProofEnabled {
				t.Fatal("standalone did not restore proof mode", after)
			}
			simplifyMust(t, m.DelRef(&out))
			simplifyMust(t, m.DelRef(&input))
			simplifyMust(t, m.Close())
		})
	}
}
