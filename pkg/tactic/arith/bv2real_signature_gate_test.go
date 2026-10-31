package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:60-63.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestBV2RealOwnedSignatureAssertionOperandGate(t *testing.T) {
	u, _ := bv2RealFixture(t, 64)
	debug := equationSourceDebugFixture(t)
	c := &bv2RealCall{u: u}
	t.Cleanup(func() { equationMust(t, c.close()) })
	// Deliberately invalid operands detect evaluation of an erased assertion.
	// No numeral or AST access is permitted when the source gate is disabled.
	var unused unumeral.OwnedRational
	equationMust(t, debug.EnableAssertions(false))
	equationMust(t, c.checkOwnedSignature(ast.ASTHandle{}, ast.ASTHandle{}, nil, &unused, &unused))
	equationMust(t, debug.EnableAssertions(true))
	if !diagnostic.DebugBuild {
		equationMust(t, c.checkOwnedSignature(ast.ASTHandle{}, ast.ASTHandle{}, nil, &unused, &unused))
		return
	}
	// A failed divisor assertion stops before inspecting the root or widths.
	fraction := c.rational(big.NewRat(3, 2))
	equationMust(t, c.err)
	err := c.checkOwnedSignature(ast.ASTHandle{}, ast.ASTHandle{}, nil, fraction, &unused)
	if !errors.Is(err, diagnostic.ErrDebugAssertion) {
		t.Fatal("divisor assertion did not precede later operands", err)
	}
}

func TestBV2RealOwnedSignatureClosedDiagnosticPrecedesOperands(t *testing.T) {
	u, _ := bv2RealFixture(t, 64)
	debug := equationSourceDebugFixture(t)
	equationMust(t, debug.Close())
	c := &bv2RealCall{u: u}
	var unused unumeral.OwnedRational
	err := c.checkOwnedSignature(ast.ASTHandle{}, ast.ASTHandle{}, nil, &unused, &unused)
	if diagnostic.DebugBuild {
		if !errors.Is(err, arena.ErrClosed) {
			t.Fatal("numeric operands evaluated before diagnostic gate", err)
		}
	} else {
		equationMust(t, err)
	}
	equationMust(t, c.close())
}
