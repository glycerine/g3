package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTLiteralASTAwareCompactVerboseAndVectorDisplay(t *testing.T) {
	manager := astTestManager(t)
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	declaration, err := manager.MkFuncDecl(symbol.Numerical(2401), nil, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	atom, err := manager.MkApp(declaration, nil)
	astTestMust(t, err)
	expressions := []ast.ASTHandle{{}, atom}
	id, err := manager.GetASTID(atom)
	astTestMust(t, err)
	positive, negative := NewSMTLiteral(1, false), NewSMTLiteral(1, true)

	var compact bytes.Buffer
	astTestMust(t, DisplaySMTLiteralsCompact(&compact, manager, []SMTLiteral{SMTTrueLiteral, positive, negative, SMTFalseLiteral}, expressions))
	wantCompact := fmt.Sprintf("true #%d (not #%d) false", id, id)
	if compact.String() != wantCompact {
		t.Fatal("compact", compact.String(), wantCompact)
	}
	var verbose bytes.Buffer
	astTestMust(t, DisplaySMTLiteralsVerbose(&verbose, manager, []SMTLiteral{SMTNullLiteral, positive, negative}, expressions, " | "))
	if verbose.String() != "null | k!2401 | (not k!2401)" {
		t.Fatal("verbose", verbose.String())
	}
	negated := NegateSMTLiterals([]SMTLiteral{positive, negative, SMTTrueLiteral})
	if len(negated) != 3 || negated[0] != negative || negated[1] != positive || negated[2] != SMTFalseLiteral {
		t.Fatal("negated", negated)
	}
}

func TestSMTLiteralASTAwareDisplayGuardsAndWriterErrors(t *testing.T) {
	manager := astTestManager(t)
	var out bytes.Buffer
	if err := NewSMTLiteral(2, false).DisplayCompact(&out, manager, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("missing map entry", err)
	}
	if err := SMTNullLiteral.DisplayCompact(&out, manager, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("compact null source precondition", err)
	}
	if err := SMTTrueLiteral.Display(nil, manager, nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil writer", err)
	}
}

// AUDIT.md smtcore_04-F6: literal::display honors Z3_TRACE_NO_PP itself, so
// every caller prints #id instead of mk_pp.
func TestSMTLiteralDisplayHonorsTraceNoPP(t *testing.T) {
	manager := astTestManager(t)
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	declaration, err := manager.MkFuncDecl(symbol.Numerical(2402), nil, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	atom, err := manager.MkApp(declaration, nil)
	astTestMust(t, err)
	id, err := manager.GetASTID(atom)
	astTestMust(t, err)
	previous := traceNoPPSet
	traceNoPPSet = true
	t.Cleanup(func() { traceNoPPSet = previous })
	var out bytes.Buffer
	astTestMust(t, DisplaySMTLiteralsVerbose(&out, manager, []SMTLiteral{NewSMTLiteral(1, true), NewSMTLiteral(1, false)}, []ast.ASTHandle{{}, atom}, " "))
	if want := fmt.Sprintf("(not #%d) #%d", id, id); out.String() != want {
		t.Fatal(out.String(), want)
	}
}
