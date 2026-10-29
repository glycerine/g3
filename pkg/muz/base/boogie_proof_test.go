package base_test

// Source: src/muz/base/dl_boogie_proof.cpp proof normalization,
// substitution extraction, reverse step order, and concrete output grammar.

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func boogieProofFixture(t *testing.T) (*ruleSetFixture, ast.ASTHandle) {
	t.Helper()
	f := newRuleSetFixture(t, "P", "Q")
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	body, err := f.manager.MkImplies(x, f.app["P"])
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("boogie-x")
	ruleMust(t, err)
	quantified, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{boolean},
		Names:  []symbol.Symbol{name},
		Body:   body,
	})
	ruleMust(t, err)
	ruleProof, err := f.manager.MkProofAsserted(quantified)
	ruleMust(t, err)
	premise, err := f.manager.MkProofHypothesis(f.app["P"])
	ruleMust(t, err)
	conclusion, err := f.manager.MkImplies(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	root, err := f.manager.MkProofHyperResolve(
		[]ast.ASTHandle{ruleProof, premise},
		conclusion,
		[]ast.ASTProofPosition{{First: 1}},
		[][]ast.ASTHandle{{f.app["P"]}, nil},
	)
	ruleMust(t, err)
	return f, root
}

func TestBoogieProofSubstitutionAndOutput(t *testing.T) {
	f, root := boogieProofFixture(t)
	var diagnostics bytes.Buffer
	proof, err := base.NewBoogieProof(f.manager, base.BoogieProofConfig{Diagnostic: &diagnostics})
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, proof.Close()) })
	ruleMust(t, proof.SetProof(root))
	if diagnostics.String() != "set proof\nproof set\n" {
		t.Fatal("literal proof diagnostics", diagnostics.String())
	}
	var out bytes.Buffer
	ruleMust(t, proof.PP(&out))
	text := out.String()
	for _, want := range []string{
		"(derivation\n",
		"(step\n s!0 ",
		"(= boogie-x ruleset-P)",
		"(labels)\n",
		"(ref)\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in Boogie proof:\n%s", want, text)
		}
	}
	if !strings.HasSuffix(text, ")\n") {
		t.Fatal("derivation terminator", text)
	}
}

type boogieRefusingWriter struct{ writes int }

func (w *boogieRefusingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == 2 {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func TestBoogieProofWriterFailureAndCloseGuards(t *testing.T) {
	f, root := boogieProofFixture(t)
	proof, err := base.NewBoogieProof(f.manager, base.BoogieProofConfig{Diagnostic: io.Discard})
	ruleMust(t, err)
	ruleMust(t, proof.SetProof(root))
	sourceModel, err := model.NewModel(f.manager)
	ruleMust(t, err)
	ruleMust(t, proof.SetModel(sourceModel))
	// The source model_ref keeps the model alive. The Go specialization owns a
	// concrete copy, so closing the caller's model must not affect Close below.
	ruleMust(t, sourceModel.Close())
	if err = proof.PP(&boogieRefusingWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("writer refusal", err)
	}
	ruleMust(t, proof.Close())
	if err = proof.PP(io.Discard); !errors.Is(err, base.ErrBoogieProofOwner) {
		t.Fatal("closed proof accepted", err)
	}
	ruleMust(t, proof.Close())
}
