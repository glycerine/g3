package smtcore

// Source: src/smt/theory_utvpi_def.h:82-94,461-466 and
// src/smt/diff_logic.h:888-922.

import (
	"bytes"
	"errors"
	"io"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTUTVPIDisplayAtomsGraphAssignmentsAndGuards(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTUTVPITheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "utvpi-display-x"), sort)
	astTestMust(t, err)
	seven, err := bigTestArithNumeral(m, big.NewRat(7, 1), true)
	astTestMust(t, err)
	bound, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{x, seven})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(bound, true))
	literal, err := state.Literal(bound)
	astTestMust(t, err)
	astTestMust(t, theory.Assign(literal.Var(), true))
	astTestMust(t, theory.Propagate())

	const want = "p1 (<= utvpi-display-x 7) \n" +
		"p1 (<= (- $2 $3) 14) 0\n" +
		"p1 (<= (- $2 $3) 14) 1\n" +
		"$0 := 0\n$1 := 0\n$2 := 0\n$3 := 0\n"
	var output bytes.Buffer
	astTestMust(t, state.DisplayTheories(&output))
	if output.String() != want {
		t.Fatalf("UTVPI display\ngot:\n%s\nwant:\n%s", output.String(), want)
	}
	if err = theory.Display(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil output", err)
	}
	if err = (*SMTUTVPITheory)(nil).Display(&output); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil theory", err)
	}
	if err = (&SMTUTVPITheory{}).Display(&output); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("uninitialized theory", err)
	}
	if err = theory.Display(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short output", err)
	}
}
