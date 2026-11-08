package spacer_test

// Source: src/muz/spacer/spacer_farkas_learner.cpp proof traversal and public
// split-literal/statistics controls.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/statistics"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/spacer"
)

func spacerMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestFarkasLearnerExtractsProofLemmaAndGuardsLifetime(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	learner, err := spacer.NewFarkasLearner(manager, globals.Context, globals)
	spacerMust(t, err)

	arithmetic, err := manager.RegisterArithmeticPlugin()
	spacerMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	spacerMust(t, err)
	xName, err := manager.Symbols.Intern("spacer-farkas-x")
	spacerMust(t, err)
	x, err := manager.MkConst(xName, integer)
	spacerMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	spacerMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	spacerMust(t, err)
	le, err := manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{x, zero})
	spacerMust(t, err)
	ge, err := manager.MkFamilyApp(arithmetic, 3, nil, []ast.ASTHandle{x, one})
	spacerMust(t, err)
	premise0, err := manager.MkProofAsserted(le)
	spacerMust(t, err)
	premise1, err := manager.MkProofAsserted(ge)
	spacerMust(t, err)
	falsity, err := manager.MkFalse()
	spacerMust(t, err)
	farkasName, err := manager.Symbols.Intern("farkas")
	spacerMust(t, err)
	farkas, err := manager.NewParameterSymbol(farkasName)
	spacerMust(t, err)
	coefficientValue, err := manager.NumeralService().NewValue()
	spacerMust(t, err)
	spacerMust(t, manager.NumeralService().SetString(&coefficientValue, "1"))
	coefficient0, err := manager.NewParameterRational(&coefficientValue)
	spacerMust(t, err)
	coefficient1, err := manager.NewParameterRational(&coefficientValue)
	spacerMust(t, err)
	spacerMust(t, manager.NumeralService().Del(&coefficientValue))
	root, err := manager.MkProofTheoryLemma(arithmetic, falsity, []ast.ASTHandle{premise0, premise1}, []ast.ASTParameter{farkas, coefficient0, coefficient1})
	spacerMust(t, err)
	spacerMust(t, manager.DelParameter(&coefficient1))
	spacerMust(t, manager.DelParameter(&coefficient0))
	spacerMust(t, manager.DelParameter(&farkas))
	rootRef, err := manager.NewRef(root)
	spacerMust(t, err)

	spacerMust(t, learner.SetSplitLiterals(true))
	lemmas, err := learner.GetLemmas(root, []ast.ASTHandle{le})
	spacerMust(t, err)
	if len(lemmas) != 1 {
		t.Fatal("proof traversal did not recover the Spacer Farkas lemma", len(lemmas))
	}
	spacerMust(t, manager.DelRef(&lemmas[0]))
	if len(statistics.Sums(learner.CollectStatistics)) != 0 {
		t.Fatal("source learner unexpectedly reported statistics")
	}
	spacerMust(t, learner.ResetStatistics())
	spacerMust(t, learner.Close())
	if err := learner.SetSplitLiterals(false); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed learner accepted split-literal update", err)
	}
	if _, err := learner.GetLemmas(root, nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed learner accepted proof traversal", err)
	}
	spacerMust(t, manager.DelRef(&rootRef))
	spacerMust(t, command.Close())
}
