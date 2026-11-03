package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLiteralExpansionListCleanupPreservesReverseFrontier(t *testing.T) {
	manager, _ := newSpacerTestServices(t)
	boolean, err := manager.MkBoolSort()
	depthMust(t, err)
	firstValue, err := manager.MkFreshConst("expansion-list-first", boolean)
	depthMust(t, err)
	secondValue, err := manager.MkFreshConst("expansion-list-second", boolean)
	depthMust(t, err)
	first := &literalExpansion{manager: manager, literals: []ast.ASTHandle{firstValue}}
	_, err = first.keep(firstValue, nil)
	depthMust(t, err)
	second := &literalExpansion{manager: manager, literals: []ast.ASTHandle{secondValue}}
	_, err = second.keep(secondValue, nil)
	depthMust(t, err)
	stale := second.refs[0]
	depthMust(t, manager.DecRef(stale))

	owner := &literalExpansionListOwner{runtime: manager.NumeralService().RuntimeOwner()}
	owner.adopt(first)
	owner.adopt(second)
	origin := errors.New("inductive generalization failed")
	failure := finishLiteralExpansionList(owner, origin)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || receipt.RuntimeOwner() != owner.runtime {
		t.Fatal("expansion-list cleanup refusal lost typed owner", failure)
	}
	if len(owner.expansions) != 2 || len(first.refs) != 1 || len(second.refs) != 1 {
		t.Fatal("expansion-list cleanup did not preserve reverse frontier")
	}
	message, exposed := receipt.Error(), receipt.Unwrap()
	if receipt.OriginalCause() != origin || receipt.CleanupCause() == nil {
		t.Fatal("expansion-list receipt lost operation or cleanup cause")
	}
	second.refs[0] = ast.ASTHandle{}
	depthMust(t, receipt.Cleanup())
	depthMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || len(owner.expansions) != 0 || receipt.OriginalCause() != origin || receipt.Error() != message || receipt.Unwrap() != exposed {
		t.Fatal("expansion-list cleanup retry changed stable receipt or left owners")
	}
}

func TestLiteralExpansionListCleanupAdoptsNestedAndHeldFrontiers(t *testing.T) {
	manager, _ := newSpacerTestServices(t)
	boolean, err := manager.MkBoolSort()
	depthMust(t, err)
	innerValue, err := manager.MkFreshConst("expansion-list-inner", boolean)
	depthMust(t, err)
	outerValue, err := manager.MkFreshConst("expansion-list-outer", boolean)
	depthMust(t, err)
	inner := &literalExpansion{manager: manager, literals: []ast.ASTHandle{innerValue}}
	_, err = inner.keep(innerValue, nil)
	depthMust(t, err)
	innerStale := inner.refs[0]
	depthMust(t, manager.DecRef(innerStale))
	innerCause := errors.New("nested literal expansion failed")
	innerFailure := finishLiteralExpansionFailure(inner, innerCause)
	var innerReceipt memory.ConstructionFailure
	if !errors.As(innerFailure, &innerReceipt) {
		t.Fatal("inner expansion did not publish its cleanup owner", innerFailure)
	}

	outer := &literalExpansion{manager: manager, literals: []ast.ASTHandle{outerValue}}
	_, err = outer.keep(outerValue, nil)
	depthMust(t, err)
	outerStale := outer.refs[0]
	depthMust(t, manager.DecRef(outerStale))
	owner := &literalExpansionListOwner{manager: manager, runtime: manager.NumeralService().RuntimeOwner()}
	owner.adopt(outer)
	failure := finishLiteralExpansionList(owner, innerFailure)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || receipt.OriginalCause() != innerFailure {
		t.Fatal("outer cleanup did not adopt the nested owner", failure)
	}
	if owner.nested != innerReceipt || len(owner.expansions) != 1 || len(outer.refs) != 1 {
		t.Fatal("nested refusal advanced into the outer frontier")
	}
	message, exposed := receipt.Error(), receipt.Unwrap()
	inner.refs[0] = ast.ASTHandle{}
	if err = receipt.Cleanup(); err == nil || owner.nested != nil || len(owner.expansions) != 1 || len(outer.refs) != 1 {
		t.Fatal("cleanup did not advance from nested to outer frontier", err)
	}
	outer.refs[0] = ast.ASTHandle{}
	depthMust(t, receipt.Cleanup())
	depthMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || len(owner.expansions) != 0 || receipt.Error() != message || receipt.Unwrap() != exposed || receipt.OriginalCause() != innerFailure {
		t.Fatal("nested cleanup retry lost its stable receipt")
	}

	live, err := manager.MkFreshConst("expansion-list-held", boolean)
	depthMust(t, err)
	front, err := manager.NewRef(live)
	depthMust(t, err)
	stale, err := manager.NewRef(live)
	depthMust(t, err)
	staleCopy := stale
	depthMust(t, manager.DelRef(&stale))
	heldOwner := &literalExpansionListOwner{
		manager: manager, runtime: manager.NumeralService().RuntimeOwner(),
		held: []ast.ASTRef{front, {}, staleCopy},
	}
	heldFailure := finishLiteralExpansionList(heldOwner, nil)
	var heldReceipt memory.ConstructionFailure
	if !errors.As(heldFailure, &heldReceipt) || len(heldOwner.held) != 3 {
		t.Fatal("held-reference refusal lost its exact frontier", heldFailure, len(heldOwner.held))
	}
	heldOwner.held[2] = ast.ASTRef{}
	depthMust(t, heldReceipt.Cleanup())
	depthMust(t, heldReceipt.Cleanup())
	if heldReceipt.CleanupCause() != nil || len(heldOwner.held) != 0 {
		t.Fatal("held-reference cleanup did not skip consumed slots")
	}
}

func TestInductiveGeneralizationCleanupOwnsUnpublishedLemma(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	truth, err := manager.MkTrue()
	depthMust(t, err)
	body, err := manager.NewRef(truth)
	depthMust(t, err)
	cube, err := manager.NewRef(truth)
	depthMust(t, err)
	lemma := &Lemma{manager: manager, context: globals.Context, globals: globals, body: body, cube: []ast.ASTRef{cube}}

	boolean, err := manager.MkBoolSort()
	depthMust(t, err)
	value, err := manager.MkFreshConst("unpublished-generalized-lemma", boolean)
	depthMust(t, err)
	expansion := &literalExpansion{manager: manager, literals: []ast.ASTHandle{value}}
	_, err = expansion.keep(value, nil)
	depthMust(t, err)
	stale := expansion.refs[0]
	depthMust(t, manager.DecRef(stale))
	owner := &literalExpansionListOwner{manager: manager, runtime: manager.NumeralService().RuntimeOwner()}
	owner.adopt(expansion)

	result, failure := finishInductiveGeneralization(owner, nil, lemma, nil)
	var receipt memory.ConstructionFailure
	if result != nil || !errors.As(failure, &receipt) || owner.lemma != lemma || lemma.closed {
		t.Fatal("cleanup refusal published or lost the new lemma", result, failure)
	}
	if _, err = lemma.Body(); err != nil {
		t.Fatal("pending cleanup did not keep the unpublished lemma live", err)
	}
	message, exposed, origin := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
	expansion.refs[0] = ast.ASTHandle{}
	depthMust(t, receipt.Cleanup())
	depthMust(t, receipt.Cleanup())
	if owner.lemma != nil || !lemma.closed || receipt.CleanupCause() != nil {
		t.Fatal("cleanup retry did not close the unpublished lemma")
	}
	if receipt.Error() != message || receipt.Unwrap() != exposed || receipt.OriginalCause() != origin {
		t.Fatal("unpublished-lemma cleanup changed its stable receipt")
	}
}
