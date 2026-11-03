package fp

// Source: src/muz/rel/dl_compiler.cpp two-tail rule lowering and
// src/muz/rel/dl_instruction.cpp LOAD, JOIN, UNION, and STORE execution.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

// externalIntervalDatalogCartesianRule is the exact acyclic rule
// H(x,y) :- P(x), Q(y). The left and right declarations preserve tail order,
// which is also the result column order of the source Cartesian JOIN.
type externalIntervalDatalogCartesianRule struct {
	head  ast.ASTHandle
	left  ast.ASTHandle
	right ast.ASTHandle
}

func sameExternalIntervalCartesianRule(left, right *externalIntervalDatalogCartesianRule) bool {
	return left != nil && right != nil && left.head == right.head &&
		left.left == right.left && left.right == right.right
}

func (e *PropositionalEngine) externalIntervalCartesianRule(rule *base.Rule) (*externalIntervalDatalogCartesianRule, error) {
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	if positive != 2 || uninterpreted != 2 {
		return nil, ErrExternalIntervalDatalogFragment
	}

	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	first, firstNegated, err := rule.Tail(0)
	if err != nil {
		return nil, err
	}
	second, secondNegated, err := rule.Tail(1)
	if err != nil {
		return nil, err
	}
	if firstNegated || secondNegated {
		return nil, ErrExternalIntervalDatalogFragment
	}

	headDeclaration, err := e.manager.GetDecl(head)
	if err != nil {
		return nil, err
	}
	firstDeclaration, err := e.manager.GetDecl(first)
	if err != nil {
		return nil, err
	}
	secondDeclaration, err := e.manager.GetDecl(second)
	if err != nil {
		return nil, err
	}
	if headDeclaration == firstDeclaration || headDeclaration == secondDeclaration || firstDeclaration == secondDeclaration ||
		e.auxiliary[headDeclaration] || e.auxiliary[firstDeclaration] || e.auxiliary[secondDeclaration] {
		return nil, ErrExternalIntervalDatalogFragment
	}

	headCount, err := e.manager.GetNumArgs(head)
	if err != nil {
		return nil, err
	}
	firstCount, err := e.manager.GetNumArgs(first)
	if err != nil {
		return nil, err
	}
	secondCount, err := e.manager.GetNumArgs(second)
	if err != nil {
		return nil, err
	}
	if headCount != 2 || firstCount != 1 || secondCount != 1 {
		return nil, ErrExternalIntervalDatalogFragment
	}

	firstArgument, err := e.manager.GetArg(first, 0)
	if err != nil {
		return nil, err
	}
	secondArgument, err := e.manager.GetArg(second, 0)
	if err != nil {
		return nil, err
	}
	headLeftArgument, err := e.manager.GetArg(head, 0)
	if err != nil {
		return nil, err
	}
	headRightArgument, err := e.manager.GetArg(head, 1)
	if err != nil {
		return nil, err
	}
	arguments := []ast.ASTHandle{firstArgument, secondArgument, headLeftArgument, headRightArgument}
	for _, argument := range arguments {
		kind, cause := e.manager.GetKind(argument)
		if cause != nil {
			return nil, cause
		}
		if kind != ast.ASTVar {
			return nil, ErrExternalIntervalDatalogFragment
		}
	}
	firstVariable, err := e.manager.GetVarIndex(firstArgument)
	if err != nil {
		return nil, err
	}
	secondVariable, err := e.manager.GetVarIndex(secondArgument)
	if err != nil {
		return nil, err
	}
	headLeftVariable, err := e.manager.GetVarIndex(headLeftArgument)
	if err != nil {
		return nil, err
	}
	headRightVariable, err := e.manager.GetVarIndex(headRightArgument)
	if err != nil {
		return nil, err
	}
	if firstVariable == secondVariable || headLeftVariable == headRightVariable {
		return nil, ErrExternalIntervalDatalogFragment
	}
	// Rule normalization stores the two conjuncts in reverse source order.
	// Admit only H(x,y) :- P(x), Q(y): the normalized first tail must supply
	// the second head column and the normalized second tail the first.
	if firstVariable != headRightVariable || secondVariable != headLeftVariable {
		return nil, ErrExternalIntervalDatalogFragment
	}
	leftDeclaration, rightDeclaration := secondDeclaration, firstDeclaration
	leftArgument, rightArgument := secondArgument, firstArgument

	headState := e.predicates[headDeclaration]
	leftState := e.predicates[leftDeclaration]
	rightState := e.predicates[rightDeclaration]
	if headState == nil || leftState == nil || rightState == nil ||
		!isExternalIntervalRepresentation(headState.requestedRepresentations) ||
		!isExternalIntervalRepresentation(leftState.requestedRepresentations) ||
		!isExternalIntervalRepresentation(rightState.requestedRepresentations) ||
		len(headState.sorts) != 2 || len(leftState.sorts) != 1 || len(rightState.sorts) != 1 ||
		headState.sorts[0] != leftState.sorts[0] || headState.sorts[1] != rightState.sorts[0] {
		return nil, ErrExternalIntervalDatalogFragment
	}
	for column, argument := range []ast.ASTHandle{leftArgument, rightArgument, headLeftArgument, headRightArgument} {
		sort, cause := e.manager.GetSort(argument)
		if cause != nil {
			return nil, cause
		}
		want := headState.sorts[column%2]
		if sort != want {
			return nil, ErrExternalIntervalDatalogFragment
		}
	}
	return &externalIntervalDatalogCartesianRule{
		head: headDeclaration, left: leftDeclaration, right: rightDeclaration,
	}, nil
}

// externalIntervalDatalogCartesianContinuation owns every temporary created
// after LOAD. A committed UNION is cleaned before STORE, and STORE publication
// completes before the joined temporary is closed. Every frontier is retryable
// without replaying JOIN or UNION.
type externalIntervalDatalogCartesianContinuation struct {
	registers *externalIntervalDatalogRegisters
	temporary *rel.ExternalIntervalProduct
	nested    memory.ConstructionFailure
	runtime   *memory.Runtime
	publish   bool
}

func (c *externalIntervalDatalogCartesianContinuation) resume() error {
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	if c.publish && c.registers != nil {
		if err := c.registers.store(); err != nil {
			var owner *externalIntervalDatalogRegisterFailure
			if errors.As(err, &owner) && owner.owner == c.registers {
				c.nested = owner
				c.registers = nil
			}
			return err
		}
		c.registers = nil
	}
	if c.temporary != nil {
		if err := c.temporary.Close(); err != nil {
			return err
		}
		c.temporary = nil
	}
	if !c.publish && c.registers != nil {
		if err := c.registers.Close(); err != nil {
			return err
		}
		c.registers = nil
	}
	return nil
}

type externalIntervalDatalogCartesianFailure struct {
	original, cleanup error
	reported          error
	text              string
	owner             *externalIntervalDatalogCartesianContinuation
}

var _ memory.ConstructionFailure = (*externalIntervalDatalogCartesianFailure)(nil)

func (f *externalIntervalDatalogCartesianFailure) Error() string        { return f.text }
func (f *externalIntervalDatalogCartesianFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalDatalogCartesianFailure) OriginalCause() error { return f.original }
func (f *externalIntervalDatalogCartesianFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalDatalogCartesianFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalDatalogCartesianFailure) Cleanup() error {
	f.cleanup = f.owner.resume()
	return f.cleanup
}

func newExternalIntervalDatalogCartesianFailure(cause error, owner *externalIntervalDatalogCartesianContinuation) *externalIntervalDatalogCartesianFailure {
	return &externalIntervalDatalogCartesianFailure{
		original: cause, cleanup: cause, reported: cause, text: cause.Error(), owner: owner,
	}
}

func finishExternalIntervalDatalogCartesianFailure(cause error, owner *externalIntervalDatalogCartesianContinuation) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.resume()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalDatalogCartesianFailure{
		original: original, cleanup: cleanup, reported: reported,
		text: reported.Error(), owner: owner,
	}
}

func (a *externalIntervalDatalogActivation) setCartesianPending(failure memory.ConstructionFailure, completes bool) {
	a.pending = failure
	a.pendingCompletesRule = completes
	a.pendingIdentity = nil
	a.pendingGround = nil
	a.pendingCartesian = nil
	if completes {
		a.pendingCartesian = a.cartesian
	}
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalCartesian() error {
	registers, err := a.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{
		a.cartesian.head, a.cartesian.left, a.cartesian.right,
	})
	if err != nil {
		var pending memory.ConstructionFailure
		if errors.As(err, &pending) {
			a.setCartesianPending(pending, false)
		}
		return err
	}
	owner := &externalIntervalDatalogCartesianContinuation{
		registers: registers,
		runtime:   registers.runtime,
	}
	rollback := func(cause error) error {
		finished := finishExternalIntervalDatalogCartesianFailure(cause, owner)
		var pending memory.ConstructionFailure
		if errors.As(finished, &pending) {
			a.setCartesianPending(pending, false)
		}
		owner = nil
		return finished
	}

	left, err := registers.relation(a.cartesian.left)
	if err != nil {
		return rollback(err)
	}
	right, err := registers.relation(a.cartesian.right)
	if err != nil {
		return rollback(err)
	}
	if left != nil && right != nil {
		joined, cause := rel.JoinExternalIntervalProducts(left, right, nil, nil)
		if cause != nil {
			return rollback(cause)
		}
		owner.temporary = joined
		target, cause := registers.ensureRelation(a.cartesian.head)
		if cause != nil {
			return rollback(cause)
		}
		if cause = target.Union(joined, nil, false); cause != nil {
			if rel.ExternalAssignmentCommitted(cause) {
				var nested memory.ConstructionFailure
				if !errors.As(cause, &nested) {
					return rollback(fmt.Errorf("%w: committed union has no cleanup owner", ErrExternalIntervalDatalogFragment))
				}
				owner.nested = nested
				owner.publish = true
				failure := newExternalIntervalDatalogCartesianFailure(cause, owner)
				a.setCartesianPending(failure, true)
				return failure
			}
			return rollback(cause)
		}
	}
	owner.publish = true
	if err = owner.resume(); err != nil {
		failure := newExternalIntervalDatalogCartesianFailure(err, owner)
		a.setCartesianPending(failure, true)
		return failure
	}
	return nil
}
