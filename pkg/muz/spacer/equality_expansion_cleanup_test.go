package spacer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestEqualityExpansionCleanupPreservesReferenceFrontier(t *testing.T) {
	manager, _ := newSpacerTestServices(t)
	boolean, err := manager.MkBoolSort()
	depthMust(t, err)
	first, err := manager.MkFreshConst("equality-expansion-first", boolean)
	depthMust(t, err)
	second, err := manager.MkFreshConst("equality-expansion-second", boolean)
	depthMust(t, err)
	firstRef, err := manager.NewRef(first)
	depthMust(t, err)
	secondRef, err := manager.NewRef(second)
	depthMust(t, err)
	stale := secondRef
	depthMust(t, manager.DelRef(&secondRef))
	owner := &equalityExpansionOwner{manager: manager, refs: []ast.ASTRef{firstRef, stale}}
	origin := errors.New("equality expansion failed")
	failure := finishEqualityExpansion(owner, origin)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || receipt.RuntimeOwner() != manager.NumeralService().RuntimeOwner() {
		t.Fatal("equality-expansion cleanup refusal lost typed owner", failure)
	}
	if len(owner.refs) != 2 || owner.refs[0] == (ast.ASTRef{}) || owner.refs[1] == (ast.ASTRef{}) {
		t.Fatal("equality-expansion cleanup did not preserve exact reverse frontier")
	}
	message, exposed := receipt.Error(), receipt.Unwrap()
	if receipt.OriginalCause() != origin || receipt.CleanupCause() == nil {
		t.Fatal("equality-expansion receipt lost operation or cleanup cause")
	}
	owner.refs[1] = ast.ASTRef{}
	depthMust(t, receipt.Cleanup())
	depthMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || len(owner.refs) != 0 || receipt.OriginalCause() != origin || receipt.Error() != message || receipt.Unwrap() != exposed {
		t.Fatal("equality-expansion cleanup retry changed stable receipt or left refs")
	}
}

func TestEqualityExpansionCleanupAdoptsNestedFailure(t *testing.T) {
	manager, _ := newSpacerTestServices(t)
	boolean, err := manager.MkBoolSort()
	depthMust(t, err)
	innerValue, err := manager.MkFreshConst("equality-expansion-nested-inner", boolean)
	depthMust(t, err)
	innerRef, err := manager.NewRef(innerValue)
	depthMust(t, err)
	stale := innerRef
	depthMust(t, manager.DelRef(&innerRef))
	innerOwner := &equalityExpansionOwner{manager: manager, refs: []ast.ASTRef{stale}}
	innerFailure := finishEqualityExpansion(innerOwner, errors.New("inner equality expansion failed"))
	var innerReceipt memory.ConstructionFailure
	if !errors.As(innerFailure, &innerReceipt) {
		t.Fatal("inner cleanup did not expose a retry owner", innerFailure)
	}

	outerValue, err := manager.MkFreshConst("equality-expansion-nested-outer", boolean)
	depthMust(t, err)
	outerRef, err := manager.NewRef(outerValue)
	depthMust(t, err)
	outerOwner := &equalityExpansionOwner{manager: manager, refs: []ast.ASTRef{outerRef}}
	outerFailure := finishEqualityExpansion(outerOwner, innerFailure)
	var outerReceipt memory.ConstructionFailure
	if !errors.As(outerFailure, &outerReceipt) || outerOwner.nested == nil {
		t.Fatal("outer cleanup did not adopt nested retry owner", outerFailure)
	}
	innerOwner.refs[0] = ast.ASTRef{}
	depthMust(t, outerReceipt.Cleanup())
	depthMust(t, outerReceipt.Cleanup())
	if outerReceipt.CleanupCause() != nil || outerOwner.nested != nil || len(outerOwner.refs) != 0 {
		t.Fatal("nested cleanup retry left an ownership frontier")
	}
}

func TestSpacerEqualityExpansionFailurePreservesPartialOwners(t *testing.T) {
	failures, succeeded := 0, false
	for stop := int64(0); stop < 2048 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			manager, _ := newSpacerTestServices(t)
			transformer := &PredicateTransformer{manager: manager}
			zero, err := manager.ArithmeticIntegerNumeralText("0")
			depthMust(t, err)
			integer, err := manager.GetSort(zero)
			depthMust(t, err)
			boolean, err := manager.MkBoolSort()
			depthMust(t, err)
			var pins []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(manager, pins)) }()
			own := func(value ast.ASTHandle, valueErr error) ast.ASTHandle {
				t.Helper()
				depthMust(t, valueErr)
				ref, refErr := manager.NewRef(value)
				depthMust(t, refErr)
				pins = append(pins, ref)
				return value
			}
			terms := make([]ast.ASTHandle, 16)
			for i := range terms {
				terms[i] = own(manager.MkFreshConst("equality-expansion-partial", integer))
			}
			cube := make([]ast.ASTHandle, 0, len(terms))
			for i := 1; i < len(terms); i++ {
				cube = append(cube, own(manager.MkEq(terms[i-1], terms[i])))
			}
			cube = append(cube, own(manager.MkFreshConst("equality-expansion-remainder", boolean)))
			want := len(terms)*(len(terms)-1)/2 + 1
			baseline := manager.Stats().LiveReferences
			manager.Budget().FailAfter(stop)
			result, dirty, cause := transformer.expandEqualityClasses(cube)
			manager.Budget().FailAfter(-1)
			if cause == nil {
				if !dirty || len(result) != want {
					t.Fatal("wrong successful equality expansion", stop, dirty, len(result))
				}
				depthMust(t, releaseNormalizeRefs(manager, result))
				succeeded = true
			} else {
				failures++
				if result != nil {
					t.Fatal("failed equality expansion published refs", stop, len(result), cause)
				}
				var receipt memory.ConstructionFailure
				if errors.As(cause, &receipt) {
					depthMust(t, receipt.Cleanup())
					depthMust(t, receipt.Cleanup())
				}
			}
			if got := manager.Stats().LiveReferences; got != baseline {
				t.Fatal("equality expansion changed ownership after failure", stop, got, baseline)
			}
		})
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing equality expansion failure/success frontier", failures, succeeded)
	}
}

func TestEqualityGeneralizationCleanupOwnsUnpublishedLemma(t *testing.T) {
	manager, _ := newSpacerTestServices(t)
	boolean, err := manager.MkBoolSort()
	depthMust(t, err)
	temporary, err := manager.MkFreshConst("equality-generalization-temporary", boolean)
	depthMust(t, err)
	temporaryRef, err := manager.NewRef(temporary)
	depthMust(t, err)
	stale := temporaryRef
	depthMust(t, manager.DelRef(&temporaryRef))
	body, err := manager.MkTrue()
	depthMust(t, err)
	bodyRef, err := manager.NewRef(body)
	depthMust(t, err)
	lemma := &Lemma{manager: manager, body: bodyRef}
	owner := &equalityGeneralizationOwner{manager: manager, refs: []ast.ASTRef{stale}, lemma: lemma}
	failure := finishEqualityGeneralization(owner, nil, lemma)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || owner.lemma != lemma || lemma.closed {
		t.Fatal("cleanup refusal did not preserve unpublished lemma", failure)
	}
	message, exposed, original := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
	owner.refs[0] = ast.ASTRef{}
	// A lemma without a body is the lazy mk_expr_core state: its cleanup
	// releases the remaining roots and completes.
	depthMust(t, manager.DelRef(&lemma.body))
	if err = receipt.Cleanup(); err != nil || owner.lemma != nil || !lemma.closed {
		t.Fatal("consumed lemma roots did not advance the cleanup frontier", err, owner.lemma, lemma.closed)
	}
	depthMust(t, receipt.Cleanup())
	depthMust(t, receipt.Cleanup())
	if owner.lemma != nil || !lemma.closed || receipt.CleanupCause() != nil || receipt.OriginalCause() != original || receipt.Error() != message || receipt.Unwrap() != exposed {
		t.Fatal("unpublished lemma cleanup did not finish")
	}
}
