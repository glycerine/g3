package smtcore

// Source: src/smt/smt_theory.cpp:50-56 and
// src/smt/theory_dense_diff_logic_def.h:663-706.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTDenseDifferenceDisplayInfinitesimalText(t *testing.T) {
	tests := []struct {
		rational      *big.Rat
		infinitesimal *big.Rat
		want          string
	}{
		{big.NewRat(3, 2), big.NewRat(0, 1), "3/2"},
		{big.NewRat(3, 2), big.NewRat(2, 3), "(3/2 +e*2/3)"},
		{big.NewRat(-4, 5), big.NewRat(-7, 6), "(-4/5 -e*7/6)"},
	}
	q := testQ()
	for _, test := range tests {
		value := newSMTInfRational()
		astTestMust(t, smtQFromBig(q, &value.first, test.rational))
		astTestMust(t, smtQFromBig(q, &value.second, test.infinitesimal))
		if got := smtInfRationalText(q, &value); got != test.want {
			t.Fatalf("inf_rational::to_string(%s,%s) = %q, want %q", test.rational, test.infinitesimal, got, test.want)
		}
		astTestMust(t, value.del(q))
	}
}

func TestSMTDenseDifferenceDisplayThroughContext(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTDenseDiffTheory(family, smtDenseMI)
	astTestMust(t, err)
	astTestMust(t, theory.ConfigureArithmetic(SMTArithState{Reflect: true}))
	astTestMust(t, state.RegisterTheory(theory))
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "dense-display-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "dense-display-y"), sort)
	astTestMust(t, err)
	fixture := smtIntegerDiffFixture{manager: m, family: family, state: state, sort: sort, x: x, y: y}
	bound := fixture.bound(t, smtArithOpLE, x, y, 2)
	astTestMust(t, state.Internalize(bound, true))
	literal, err := state.Literal(bound)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	astTestMust(t, state.PropagateTheoryAssignments())

	atom := theory.atoms[0]
	sourceID, err := theory.variables[atom.source].OwnerID()
	astTestMust(t, err)
	targetID, err := theory.variables[atom.target].OwnerID()
	astTestMust(t, err)
	want := "Theory dense difference logic:\n" +
		fmt.Sprintf("v0 -> #%d\nv1 -> #%d\n", sourceID, targetID) +
		fmt.Sprintf("#%-5d -- %-10s : id%-5d --> #%d\n", sourceID, "2", 1, targetID) +
		"atoms:\n" +
		fmt.Sprintf("#%-5d - #%-5d <= %-10s        assignment: l_true\n", targetID, sourceID, "2")
	var output bytes.Buffer
	astTestMust(t, state.DisplayTheories(&output))
	if output.String() != want {
		t.Fatalf("dense theory display\ngot:\n%s\nwant:\n%s", output.String(), want)
	}
	if !strings.Contains(output.String(), "v0 -> #") {
		t.Fatal("base display_var2enode output missing")
	}
	if err = theory.Display(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil output", err)
	}
	if err = theory.Display(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short output", err)
	}
}
