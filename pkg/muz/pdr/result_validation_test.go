package pdr

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// isPDRValidationException recognizes the default_exception thrown by
// context::validate_search, validate_proof and validate_model.
func isPDRValidationException(err error) bool {
	var exception z3exception.DefaultException
	if !errors.As(err, &exception) {
		return false
	}
	message := exception.Message()
	return message == "proof validation failed" || strings.HasPrefix(message, "rule validation failed when checking: ")
}

func TestValidateSATFormulaAcceptsWitnessAndRejectsCorruption(t *testing.T) {
	manager, owner, transformer, _, closeBase := closureFixture(t)
	context := &Context{manager: manager, owner: owner}
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	name, err := manager.Symbols.Intern("pdr_validation_atom")
	closureMust(t, err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	atom, err := manager.MkApp(declaration, nil)
	closureMust(t, err)
	negated, err := manager.MkNot(atom)
	closureMust(t, err)
	witness, err := manager.MkOr(atom, negated)
	closureMust(t, err)
	corrupted, err := manager.MkAnd(atom, negated)
	closureMust(t, err)

	if err = context.validateSATFormula(witness); err != nil {
		t.Fatal("satisfiable reconstructed trace rejected", err)
	}
	if err = context.validateSATFormula(corrupted); !isPDRValidationException(err) {
		t.Fatal("corrupted reconstructed trace accepted", err)
	}

	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	closeBase()
}

func TestValidationFailureInvalidatesDirectContextResult(t *testing.T) {
	manager, owner, transformer, _, closeBase := closureFixture(t)
	head, err := transformer.Head()
	closureMust(t, err)
	context := &Context{
		manager: manager, owner: owner, validateResult: true,
		query: transformer, queryDecl: head,
	}
	if err = context.finishValidatedResult(lbool.True); err == nil {
		t.Fatal("missing reachable trace passed validation")
	}
	status, err := context.LastResult()
	closureMust(t, err)
	if status != lbool.Undef {
		t.Fatal("failed validation left a publishable result", status)
	}
	if answer, answerErr := context.Answer(head); answerErr == nil || answer != (ast.ASTRef{}) {
		t.Fatal("failed validation published a stale answer", answer, answerErr)
	}
	if md, modelErr := context.Model(head); modelErr == nil || md != nil {
		t.Fatal("failed validation published a stale model", md, modelErr)
	}
	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	closeBase()
}

func TestValidateProofHandleChecksSemanticSideConditions(t *testing.T) {
	manager, owner, transformer, _, closeBase := closureFixture(t)
	context := &Context{manager: manager, owner: owner}
	closureMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	name, err := manager.Symbols.Intern("pdr_validation_proof_atom")
	closureMust(t, err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	atom, err := manager.MkApp(declaration, nil)
	closureMust(t, err)
	asserted, err := manager.MkProofAsserted(atom)
	closureMust(t, err)
	if err = context.validateProofHandle(asserted); err != nil {
		t.Fatal("valid proof rejected", err)
	}
	negated, err := manager.MkNot(atom)
	closureMust(t, err)
	unsupportedRewrite, err := manager.MkProofRewrite(atom, negated)
	closureMust(t, err)
	if err = context.validateProofHandle(unsupportedRewrite); !isPDRValidationException(err) {
		t.Fatal("unsupported proof side condition accepted", err)
	}

	closureMust(t, manager.ToggleProofMode(ast.ASTProofDisabled))
	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	closeBase()
}
