package fp

// Source: src/muz/rel/dl_compiler.cpp two-tail rule lowering and
// src/muz/rel/dl_instruction.cpp LOAD, JOIN_PROJECT, RENAME, UNION, and STORE.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

// externalIntervalDatalogCartesianRule lowers two positive variable-only
// tails with distinct columns to a head of distinct bound variables. It also
// covers the original Cartesian rule as the no-key/no-projection case.
type externalIntervalDatalogCartesianRule struct {
	head         ast.ASTHandle
	left         ast.ASTHandle
	right        ast.ASTHandle
	leftColumns  []uint32
	rightColumns []uint32
	removed      []uint32
	renames      [][]uint32
}

func sameExternalIntervalCartesianRule(left, right *externalIntervalDatalogCartesianRule) bool {
	if left == nil || right == nil || left.head != right.head || left.left != right.left || left.right != right.right ||
		!externalIntervalColumnsEqual(left.leftColumns, right.leftColumns) ||
		!externalIntervalColumnsEqual(left.rightColumns, right.rightColumns) ||
		!externalIntervalColumnsEqual(left.removed, right.removed) || len(left.renames) != len(right.renames) {
		return false
	}
	for i := range left.renames {
		if !externalIntervalColumnsEqual(left.renames[i], right.renames[i]) {
			return false
		}
	}
	return true
}

func externalIntervalColumnsEqual(left, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for i, column := range left {
		if column != right[i] {
			return false
		}
	}
	return true
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
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	if positive != 2 || uninterpreted != 2 || total != 2 {
		return nil, ErrExternalIntervalDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	// The source compiler consumes normalized rule tail order directly.
	left, leftNegated, err := rule.Tail(0)
	if err != nil {
		return nil, err
	}
	right, rightNegated, err := rule.Tail(1)
	if err != nil {
		return nil, err
	}
	if leftNegated || rightNegated {
		return nil, ErrExternalIntervalDatalogFragment
	}
	plan := &externalIntervalDatalogCartesianRule{}
	applications := []ast.ASTHandle{head, left, right}
	declarations := []*ast.ASTHandle{&plan.head, &plan.left, &plan.right}
	variables := make([][]uint32, 3)
	sorts := make([][]ast.ASTHandle, 3)
	for index, application := range applications {
		declaration, cause := e.manager.GetDecl(application)
		if cause != nil {
			return nil, cause
		}
		state := e.predicates[declaration]
		if state == nil || e.auxiliary[declaration] || !isExternalIntervalRepresentation(state.requestedRepresentations) {
			return nil, ErrExternalIntervalDatalogFragment
		}
		count, cause := e.manager.GetNumArgs(application)
		if cause != nil {
			return nil, cause
		}
		if int(count) != len(state.sorts) || index != 0 && count == 0 {
			return nil, ErrExternalIntervalDatalogFragment
		}
		*declarations[index] = declaration
		sorts[index] = state.sorts
		variables[index] = make([]uint32, count)
		seen := make(map[uint32]bool)
		for column := uint32(0); column < count; column++ {
			argument, cause := e.manager.GetArg(application, column)
			if cause != nil {
				return nil, cause
			}
			kind, cause := e.manager.GetKind(argument)
			if cause != nil {
				return nil, cause
			}
			if kind != ast.ASTVar {
				return nil, ErrExternalIntervalDatalogFragment
			}
			variable, cause := e.manager.GetVarIndex(argument)
			if cause != nil {
				return nil, cause
			}
			sort, cause := e.manager.GetSort(argument)
			if cause != nil {
				return nil, cause
			}
			if seen[variable] || sort != state.sorts[column] {
				return nil, ErrExternalIntervalDatalogFragment
			}
			seen[variable] = true
			variables[index][column] = variable
		}
	}
	if plan.head == plan.left || plan.head == plan.right || plan.left == plan.right {
		return nil, ErrExternalIntervalDatalogFragment
	}
	leftVariables, rightVariables := variables[1], variables[2]
	for first, variable := range leftVariables {
		for second, other := range rightVariables {
			if variable == other {
				if sorts[1][first] != sorts[2][second] {
					return nil, ErrExternalIntervalDatalogFragment
				}
				plan.leftColumns = append(plan.leftColumns, uint32(first))
				plan.rightColumns = append(plan.rightColumns, uint32(second))
			}
		}
	}
	joinedVariables := append(append([]uint32(nil), leftVariables...), rightVariables...)
	joinedSorts := append(append([]ast.ASTHandle(nil), sorts[1]...), sorts[2]...)
	last := make(map[uint32]uint32)
	for column, variable := range joinedVariables {
		last[variable] = uint32(column)
	}
	wanted := make(map[uint32]bool)
	for column, variable := range variables[0] {
		source, found := last[variable]
		if !found || joinedSorts[source] != sorts[0][column] {
			return nil, ErrExternalIntervalDatalogFragment
		}
		wanted[variable] = true
	}
	// Source get_local_indexes_for_projection discards unused variables and
	// the earliest extra copies, leaving the last occurrence for each head.
	surviving := make(map[uint32]uint32)
	for column, variable := range joinedVariables {
		if !wanted[variable] || last[variable] != uint32(column) {
			plan.removed = append(plan.removed, uint32(column))
		} else {
			surviving[variable] = uint32(len(surviving))
		}
	}
	permutation := make([]uint32, len(variables[0]))
	for column, variable := range variables[0] {
		permutation[column] = surviving[variable]
	}
	for column := range permutation {
		if permutation[column] == uint32(column) {
			continue
		}
		var cycle []uint32
		next := uint32(column)
		for {
			cycle = append(cycle, next)
			previous := next
			next = permutation[previous]
			permutation[previous] = previous
			if next == uint32(column) {
				break
			}
		}
		// The compiler's source permutation rotates left, while the source
		// datalog declaration and vector-relation rename rotate right. Those
		// disagree for cycles longer than two. Lower the compiler permutation
		// to adjacent transpositions, whose signatures, external callbacks and
		// interval values agree, instead of changing either shared primitive.
		for i := 0; i+1 < len(cycle); i++ {
			plan.renames = append(plan.renames, []uint32{cycle[i], cycle[i+1]})
		}
	}
	return plan, nil
}

// externalIntervalDatalogCartesianContinuation owns every temporary created
// after LOAD. A committed UNION and its temporary are cleaned before STORE.
// Every frontier is retryable without replaying JOIN or UNION.
type externalIntervalDatalogCartesianContinuation struct {
	registers *externalIntervalDatalogRegisters
	temporary *rel.ExternalIntervalProduct
	retiring  *rel.ExternalIntervalProduct
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
	if c.temporary != nil {
		if err := c.temporary.Close(); err != nil {
			return err
		}
		c.temporary = nil
	}
	if c.retiring != nil {
		if err := c.retiring.Close(); err != nil {
			return err
		}
		c.retiring = nil
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
	if !c.publish && c.registers != nil {
		if err := c.registers.Close(); err != nil {
			return err
		}
		c.registers = nil
	}
	return nil
}

func (c *externalIntervalDatalogCartesianContinuation) replaceTemporary(next *rel.ExternalIntervalProduct) error {
	c.retiring, c.temporary = c.temporary, next
	if c.retiring != nil {
		if err := c.retiring.Close(); err != nil {
			return err
		}
		c.retiring = nil
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
		joined, cause := rel.JoinExternalIntervalProducts(left, right, a.cartesian.leftColumns, a.cartesian.rightColumns)
		if cause != nil {
			return rollback(cause)
		}
		owner.temporary = joined
		if len(a.cartesian.removed) != 0 {
			projected, cause := owner.temporary.Project(a.cartesian.removed)
			if cause != nil {
				return rollback(cause)
			}
			if cause = owner.replaceTemporary(projected); cause != nil {
				return rollback(cause)
			}
		}
		// instr_join_project canonicalizes an empty result before any head
		// assembly. In particular, disjoint interval keys must not reach UNION.
		empty, cause := owner.temporary.Empty()
		if cause != nil {
			return rollback(cause)
		}
		if empty {
			owner.publish = true
			if cause = owner.resume(); cause != nil {
				failure := newExternalIntervalDatalogCartesianFailure(cause, owner)
				a.setCartesianPending(failure, true)
				return failure
			}
			return nil
		}
		for _, cycle := range a.cartesian.renames {
			renamed, cause := owner.temporary.Rename(cycle)
			if cause != nil {
				return rollback(cause)
			}
			if cause = owner.replaceTemporary(renamed); cause != nil {
				return rollback(cause)
			}
		}
		target, cause := registers.ensureRelation(a.cartesian.head)
		if cause != nil {
			return rollback(cause)
		}
		if cause = target.Union(owner.temporary, nil, false); cause != nil {
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
