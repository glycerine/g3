package fp

// Source: src/muz/rel/dl_compiler.cpp aligned one-tail rule lowering and
// src/muz/rel/dl_instruction.cpp LOAD, UNION, and STORE execution.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalDatalogIdentityRule struct {
	head ast.ASTHandle
	tail ast.ASTHandle
}

func sameExternalIntervalIdentityRule(left, right *externalIntervalDatalogIdentityRule) bool {
	return left != nil && right != nil && left.head == right.head && left.tail == right.tail
}

func externalIntervalIdentitySpecsEqual(head, tail *predicateState) bool {
	if head == nil || tail == nil || !isExternalIntervalRepresentation(head.requestedRepresentations) ||
		!isExternalIntervalRepresentation(tail.requestedRepresentations) ||
		!externalIntervalKindsEqual(head.requestedRepresentations, tail.requestedRepresentations) ||
		len(head.sorts) == 0 || len(head.sorts) != len(tail.sorts) {
		return false
	}
	for i := range head.sorts {
		if head.sorts[i] != tail.sorts[i] {
			return false
		}
	}
	return true
}

// externalIntervalIdentityRule validates the complete rule fragment without
// invoking relation callbacks. Zero rules keeps the rule-free activation; the
// only admitted rule is one acyclic, aligned, unique-variable identity edge.
func (e *PropositionalEngine) externalIntervalIdentityRule() (_ *externalIntervalDatalogIdentityRule, resultErr error) {
	count, err := e.set.Len()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	if count != 1 {
		return nil, ErrExternalIntervalDatalogFragment
	}
	rule, err := e.set.Rule(0)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	if total != 1 || positive != 1 || uninterpreted != 1 {
		return nil, ErrExternalIntervalDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	tail, negated, err := rule.Tail(0)
	if err != nil {
		return nil, err
	}
	if negated {
		return nil, ErrExternalIntervalDatalogFragment
	}
	headDeclaration, err := e.manager.GetDecl(head)
	if err != nil {
		return nil, err
	}
	tailDeclaration, err := e.manager.GetDecl(tail)
	if err != nil {
		return nil, err
	}
	if headDeclaration == tailDeclaration || !externalIntervalIdentitySpecsEqual(e.predicates[headDeclaration], e.predicates[tailDeclaration]) {
		return nil, ErrExternalIntervalDatalogFragment
	}
	headCount, err := e.manager.GetNumArgs(head)
	if err != nil {
		return nil, err
	}
	tailCount, err := e.manager.GetNumArgs(tail)
	if err != nil {
		return nil, err
	}
	headState := e.predicates[headDeclaration]
	if headCount == 0 || headCount != tailCount || int(headCount) != len(headState.sorts) {
		return nil, ErrExternalIntervalDatalogFragment
	}
	seen := make(map[uint32]bool, headCount)
	for column := uint32(0); column < headCount; column++ {
		tailArgument, cause := e.manager.GetArg(tail, column)
		if cause != nil {
			return nil, cause
		}
		headArgument, cause := e.manager.GetArg(head, column)
		if cause != nil {
			return nil, cause
		}
		tailKind, cause := e.manager.GetKind(tailArgument)
		if cause != nil {
			return nil, cause
		}
		headKind, cause := e.manager.GetKind(headArgument)
		if cause != nil {
			return nil, cause
		}
		if tailKind != ast.ASTVar || headKind != ast.ASTVar {
			return nil, ErrExternalIntervalDatalogFragment
		}
		tailVariable, cause := e.manager.GetVarIndex(tailArgument)
		if cause != nil {
			return nil, cause
		}
		headVariable, cause := e.manager.GetVarIndex(headArgument)
		if cause != nil {
			return nil, cause
		}
		if tailVariable != headVariable || seen[tailVariable] {
			return nil, ErrExternalIntervalDatalogFragment
		}
		seen[tailVariable] = true
		tailSort, cause := e.manager.GetSort(tailArgument)
		if cause != nil {
			return nil, cause
		}
		headSort, cause := e.manager.GetSort(headArgument)
		if cause != nil {
			return nil, cause
		}
		if tailSort != headState.sorts[column] || headSort != headState.sorts[column] {
			return nil, ErrExternalIntervalDatalogFragment
		}
	}
	return &externalIntervalDatalogIdentityRule{head: headDeclaration, tail: tailDeclaration}, nil
}

type externalIntervalDatalogIdentityContinuation struct {
	registers *externalIntervalDatalogRegisters
	nested    memory.ConstructionFailure
	runtime   *memory.Runtime
}

func (c *externalIntervalDatalogIdentityContinuation) resume() error {
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	if c.registers == nil {
		return nil
	}
	if err := c.registers.store(); err != nil {
		var owner *externalIntervalDatalogRegisterFailure
		if errors.As(err, &owner) && owner.owner == c.registers {
			c.nested = owner
			c.registers = nil
		}
		return err
	}
	c.registers = nil
	return nil
}

type externalIntervalDatalogIdentityFailure struct {
	original, cleanup error
	reported          error
	text              string
	owner             *externalIntervalDatalogIdentityContinuation
}

var _ memory.ConstructionFailure = (*externalIntervalDatalogIdentityFailure)(nil)

func (f *externalIntervalDatalogIdentityFailure) Error() string        { return f.text }
func (f *externalIntervalDatalogIdentityFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalDatalogIdentityFailure) OriginalCause() error { return f.original }
func (f *externalIntervalDatalogIdentityFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalDatalogIdentityFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalDatalogIdentityFailure) Cleanup() error {
	f.cleanup = f.owner.resume()
	return f.cleanup
}

func newExternalIntervalDatalogIdentityFailure(cause error, registers *externalIntervalDatalogRegisters, nested memory.ConstructionFailure) *externalIntervalDatalogIdentityFailure {
	return &externalIntervalDatalogIdentityFailure{
		original: cause, cleanup: cause, reported: cause, text: cause.Error(),
		owner: &externalIntervalDatalogIdentityContinuation{
			registers: registers,
			nested:    nested,
			runtime:   registers.runtime,
		},
	}
}

func (a *externalIntervalDatalogActivation) resumeExternalIntervalIdentity() (bool, error) {
	if a.pending == nil {
		return false, nil
	}
	if err := a.pending.Cleanup(); err != nil {
		return false, err
	}
	completed := a.pendingCompletesRule && (sameExternalIntervalIdentityRule(a.pendingIdentity, a.identity) || sameExternalIntervalGroundRule(a.pendingGround, a.ground))
	a.pending = nil
	a.pendingCompletesRule = false
	a.pendingIdentity = nil
	a.pendingGround = nil
	return completed, nil
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalIdentity() (resultErr error) {
	if err := a.open(); err != nil {
		return err
	}
	completed, err := a.resumeExternalIntervalIdentity()
	if err != nil {
		return err
	}
	if completed {
		return nil
	}
	if a.identity == nil {
		if a.ground != nil {
			return a.executeExternalIntervalGround()
		}
		return nil
	}
	registers, err := a.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{a.identity.head, a.identity.tail})
	if err != nil {
		var pending memory.ConstructionFailure
		if errors.As(err, &pending) {
			a.pending = pending
			a.pendingCompletesRule = false
			a.pendingIdentity = nil
			a.pendingGround = nil
		}
		return err
	}
	rollback := func(cause error) error {
		finished := finishExternalIntervalDatalogRegisterFailure(cause, registers)
		var pending memory.ConstructionFailure
		if errors.As(finished, &pending) {
			a.pending = pending
			a.pendingCompletesRule = false
			a.pendingIdentity = nil
			a.pendingGround = nil
		}
		registers = nil
		return finished
	}
	source, err := registers.relation(a.identity.tail)
	if err != nil {
		return rollback(err)
	}
	if source != nil {
		target, cause := registers.ensureRelation(a.identity.head)
		if cause != nil {
			return rollback(cause)
		}
		if cause = target.Union(source, nil, false); cause != nil {
			if rel.ExternalAssignmentCommitted(cause) {
				var nested memory.ConstructionFailure
				if !errors.As(cause, &nested) {
					return rollback(fmt.Errorf("%w: committed union has no cleanup owner", ErrExternalIntervalDatalogFragment))
				}
				failure := newExternalIntervalDatalogIdentityFailure(cause, registers, nested)
				a.pending = failure
				a.pendingCompletesRule = true
				a.pendingIdentity = a.identity
				a.pendingGround = nil
				registers = nil
				return failure
			}
			return rollback(cause)
		}
	}
	if err = registers.store(); err != nil {
		var pending *externalIntervalDatalogRegisterFailure
		if errors.As(err, &pending) && pending.owner == registers {
			a.pending = pending
			a.pendingCompletesRule = true
			a.pendingIdentity = a.identity
			a.pendingGround = nil
			registers = nil
			return err
		}
		failure := newExternalIntervalDatalogIdentityFailure(err, registers, nil)
		a.pending = failure
		a.pendingCompletesRule = true
		a.pendingIdentity = a.identity
		a.pendingGround = nil
		registers = nil
		return failure
	}
	registers = nil
	return nil
}
