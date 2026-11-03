package fp

// Source: src/muz/rel/dl_compiler.cpp zero-tail head assembly and
// src/muz/rel/dl_instruction.cpp LOAD, TOTAL, MK_UNARY_SINGLETON, JOIN, UNION,
// DEALLOC, and STORE.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalDatalogGroundRule struct {
	head            ast.ASTHandle
	application     ast.ASTHandle
	values          []ast.ASTHandle
	applicationRoot ast.ASTRef
}

func (r *externalIntervalDatalogGroundRule) close(manager *ast.ASTManager) error {
	if r == nil || r.applicationRoot == (ast.ASTRef{}) {
		return nil
	}
	return manager.DelRef(&r.applicationRoot)
}

func sameExternalIntervalGroundRule(left, right *externalIntervalDatalogGroundRule) bool {
	if left == nil || right == nil || left.head != right.head || len(left.values) != len(right.values) {
		return false
	}
	for i, value := range left.values {
		if value != right.values[i] {
			return false
		}
	}
	return true
}

func (e *PropositionalEngine) externalIntervalGroundRule(rule *base.Rule) (*externalIntervalDatalogGroundRule, error) {
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	if positive != 0 || uninterpreted != 0 {
		return nil, ErrExternalIntervalDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	declaration, err := e.manager.GetDecl(head)
	if err != nil {
		return nil, err
	}
	state := e.predicates[declaration]
	if state == nil || !isExternalIntervalRepresentation(state.requestedRepresentations) {
		return nil, ErrExternalIntervalDatalogFragment
	}
	count, err := e.manager.GetNumArgs(head)
	if err != nil {
		return nil, err
	}
	if int(count) != len(state.sorts) {
		return nil, ErrExternalIntervalDatalogFragment
	}
	values := make([]ast.ASTHandle, count)
	for i := range values {
		value, cause := e.manager.GetArg(head, uint32(i))
		if cause != nil {
			return nil, cause
		}
		sort, cause := e.manager.GetSort(value)
		if cause != nil {
			return nil, cause
		}
		_, _, numeral, cause := e.manager.ArithmeticNumeralText(value)
		if cause != nil {
			return nil, cause
		}
		if !numeral || sort != state.sorts[i] {
			return nil, ErrExternalIntervalDatalogFragment
		}
		values[i] = value
	}
	return &externalIntervalDatalogGroundRule{head: declaration, application: head, values: values}, nil
}

// externalIntervalRulePlans validates the complete retained one-rule fragment
// before relation callbacks. The live plans are the aligned identity edge, a
// zero-tail arithmetic numeral fact, and two positive variable-only tails with
// a bound, distinct-variable head.
func (e *PropositionalEngine) externalIntervalRulePlans() (_ *externalIntervalDatalogIdentityRule, _ *externalIntervalDatalogGroundRule, _ *externalIntervalDatalogCartesianRule, resultErr error) {
	count, err := e.set.Len()
	if err != nil {
		return nil, nil, nil, err
	}
	if count == 0 {
		return nil, nil, nil, nil
	}
	if count != 1 {
		return nil, nil, nil, ErrExternalIntervalDatalogFragment
	}
	rule, err := e.set.Rule(0)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	total, err := rule.TailSize()
	if err != nil {
		return nil, nil, nil, err
	}
	if total == 0 {
		ground, cause := e.externalIntervalGroundRule(rule)
		return nil, ground, nil, cause
	}
	if total == 1 {
		identity, cause := e.externalIntervalIdentityRule()
		return identity, nil, nil, cause
	}
	if total == 2 {
		cartesian, cause := e.externalIntervalCartesianRule(rule)
		return nil, nil, cartesian, cause
	}
	return nil, nil, nil, ErrExternalIntervalDatalogFragment
}

func (a *externalIntervalDatalogActivation) installExternalIntervalRulePlans(identity *externalIntervalDatalogIdentityRule, ground *externalIntervalDatalogGroundRule, cartesian *externalIntervalDatalogCartesianRule) error {
	if a.program != nil {
		return ErrExternalIntervalDatalogFragment
	}
	selected := 0
	if identity != nil {
		selected++
	}
	if ground != nil {
		selected++
	}
	if cartesian != nil {
		selected++
	}
	if selected > 1 {
		return ErrExternalIntervalDatalogFragment
	}
	if a.ground != nil {
		if !sameExternalIntervalGroundRule(a.ground, ground) || identity != nil || cartesian != nil {
			return ErrExternalIntervalDatalogFragment
		}
		ground = a.ground
	}
	if a.identity != nil && identity != nil {
		if !sameExternalIntervalIdentityRule(a.identity, identity) {
			return ErrExternalIntervalDatalogFragment
		}
		identity = a.identity
	}
	if a.cartesian != nil && cartesian != nil && !sameExternalIntervalCartesianRule(a.cartesian, cartesian) {
		return ErrExternalIntervalDatalogFragment
	}
	if a.identity != nil && (ground != nil || cartesian != nil) ||
		a.ground != nil && (identity != nil || cartesian != nil) ||
		a.cartesian != nil && (identity != nil || ground != nil) {
		return ErrExternalIntervalDatalogFragment
	}
	if ground != nil && ground.applicationRoot == (ast.ASTRef{}) {
		root, err := a.manager.NewRef(ground.application)
		if err != nil {
			return err
		}
		ground.applicationRoot = root
	}
	if identity == nil {
		if err := a.identity.close(a.manager); err != nil {
			return err
		}
	}
	a.identity = identity
	a.ground = ground
	a.cartesian = cartesian
	return identity.pin(a.manager)
}

type externalIntervalDatalogGroundContinuation struct {
	registers *externalIntervalDatalogRegisters
	temporary *rel.ExternalIntervalProduct
	tuple     *externalIntervalDatalogGroundTuple
	nested    memory.ConstructionFailure
	runtime   *memory.Runtime
	publish   bool
}

func (c *externalIntervalDatalogGroundContinuation) resume() error {
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	// A joined head register is explicitly deallocated by the source rule
	// code before termination STORE; cached unary singletons outlive STORE.
	if c.tuple != nil {
		if err := c.tuple.closeJoined(); err != nil {
			return err
		}
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
	if c.tuple != nil {
		if err := c.tuple.closeSingletons(); err != nil {
			return err
		}
		c.tuple = nil
	}
	if !c.publish && c.registers != nil {
		if err := c.registers.Close(); err != nil {
			return err
		}
		c.registers = nil
	}
	return nil
}

type externalIntervalDatalogGroundFailure struct {
	original, cleanup error
	reported          error
	text              string
	owner             *externalIntervalDatalogGroundContinuation
}

var _ memory.ConstructionFailure = (*externalIntervalDatalogGroundFailure)(nil)

func (f *externalIntervalDatalogGroundFailure) Error() string        { return f.text }
func (f *externalIntervalDatalogGroundFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalDatalogGroundFailure) OriginalCause() error { return f.original }
func (f *externalIntervalDatalogGroundFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalDatalogGroundFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalDatalogGroundFailure) Cleanup() error {
	f.cleanup = f.owner.resume()
	return f.cleanup
}

func newExternalIntervalDatalogGroundFailure(cause error, owner *externalIntervalDatalogGroundContinuation) *externalIntervalDatalogGroundFailure {
	return &externalIntervalDatalogGroundFailure{
		original: cause, cleanup: cause, reported: cause, text: cause.Error(), owner: owner,
	}
}

func finishExternalIntervalDatalogGroundFailure(cause error, owner *externalIntervalDatalogGroundContinuation) error {
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
	return &externalIntervalDatalogGroundFailure{
		original: original, cleanup: cleanup, reported: reported,
		text: reported.Error(), owner: owner,
	}
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalGround() error {
	registers, err := a.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{a.ground.head})
	if err != nil {
		var pending memory.ConstructionFailure
		if errors.As(err, &pending) {
			a.pending = pending
			a.pendingCompletesRule = false
			a.pendingIdentity = nil
			a.pendingGround = nil
			a.pendingCartesian = nil
		}
		return err
	}
	owner := &externalIntervalDatalogGroundContinuation{
		registers: registers,
		runtime:   registers.runtime,
	}
	rollback := func(cause error) error {
		finished := finishExternalIntervalDatalogGroundFailure(cause, owner)
		var pending memory.ConstructionFailure
		if errors.As(finished, &pending) {
			a.pending = pending
			a.pendingCompletesRule = false
			a.pendingIdentity = nil
			a.pendingGround = nil
			a.pendingCartesian = nil
		}
		owner = nil
		return finished
	}
	entry := a.entries[a.ground.head]
	if entry == nil {
		return rollback(ErrExternalIntervalDatalogFragment)
	}
	var temporary *rel.ExternalIntervalProduct
	var selected bool
	if len(a.ground.values) > 1 {
		owner.tuple = &externalIntervalDatalogGroundTuple{}
		if a.program != nil {
			err = owner.tuple.assembleCached(registers, a.ground.head, entry.sorts, a.ground.values)
		} else {
			err = owner.tuple.assemble(a.plugin, entry.kinds, entry.sorts, a.ground.values)
		}
		if err != nil {
			return rollback(err)
		}
		temporary = owner.tuple.current
		selected = temporary != nil
	} else if a.program != nil {
		if len(a.ground.values) == 0 {
			temporary, err = registers.nullaryTotal(a.ground.head)
		} else {
			temporary, err = registers.unarySingleton(a.ground.head, entry.sorts[0], a.ground.values[0])
		}
		selected = temporary != nil
	} else if len(a.ground.values) == 0 {
		temporary, selected, err = rel.NewSelectedExternalIntervalNullaryTotal(a.plugin, entry.kinds)
	} else {
		temporary, selected, err = rel.NewSelectedExternalIntervalProduct(a.plugin, entry.sorts, entry.kinds)
	}
	if err != nil {
		return rollback(err)
	}
	if !selected || temporary == nil {
		return rollback(ErrExternalIntervalDatalogFragment)
	}
	if owner.tuple == nil && a.program == nil {
		owner.temporary = temporary
	}
	if len(a.ground.values) == 1 && a.program == nil {
		if err = temporary.AddFact(a.ground.values); err != nil {
			return rollback(err)
		}
	}
	target, err := registers.ensureRelation(a.ground.head)
	if err != nil {
		return rollback(err)
	}
	if err = target.Union(temporary, nil, false); err != nil {
		if rel.ExternalAssignmentCommitted(err) {
			var nested memory.ConstructionFailure
			if !errors.As(err, &nested) {
				return rollback(fmt.Errorf("%w: committed union has no cleanup owner", ErrExternalIntervalDatalogFragment))
			}
			owner.nested = nested
			owner.publish = true
			failure := newExternalIntervalDatalogGroundFailure(err, owner)
			a.pending = failure
			a.pendingCompletesRule = true
			a.pendingIdentity = nil
			a.pendingGround = a.ground
			a.pendingCartesian = nil
			return failure
		}
		return rollback(err)
	}
	owner.publish = true
	if err = owner.resume(); err != nil {
		failure := newExternalIntervalDatalogGroundFailure(err, owner)
		a.pending = failure
		a.pendingCompletesRule = true
		a.pendingIdentity = nil
		a.pendingGround = a.ground
		a.pendingCartesian = nil
		return failure
	}
	return nil
}
