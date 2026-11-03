package fp

// Source: dl_compiler.cpp one-tail binding, FILTER_IDENTICAL and head assembly.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalDatalogUnaryTransform struct {
	identical [][]uint32
	removed   []uint32
	renames   [][]uint32
}

func sameExternalIntervalUnaryTransform(left, right *externalIntervalDatalogUnaryTransform) bool {
	if left == nil || right == nil {
		return left == right
	}
	if !externalIntervalColumnsEqual(left.removed, right.removed) || len(left.identical) != len(right.identical) || len(left.renames) != len(right.renames) {
		return false
	}
	for i := range left.identical {
		if !externalIntervalColumnsEqual(left.identical[i], right.identical[i]) {
			return false
		}
	}
	for i := range left.renames {
		if !externalIntervalColumnsEqual(left.renames[i], right.renames[i]) {
			return false
		}
	}
	return true
}

func (e *PropositionalEngine) externalIntervalUnaryPlan(rule *base.Rule) (headDecl, tailDecl ast.ASTHandle, transform *externalIntervalDatalogUnaryTransform, err error) {
	fail := func(cause error) (ast.ASTHandle, ast.ASTHandle, *externalIntervalDatalogUnaryTransform, error) {
		return ast.ASTHandle{}, ast.ASTHandle{}, nil, cause
	}
	total, err := rule.TailSize()
	if err != nil {
		return fail(err)
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return fail(err)
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return fail(err)
	}
	if total != 1 || positive != 1 || uninterpreted != 1 {
		return fail(ErrExternalIntervalDatalogFragment)
	}
	head, err := rule.Head()
	if err != nil {
		return fail(err)
	}
	tail, negated, err := rule.Tail(0)
	if err != nil {
		return fail(err)
	}
	if negated {
		return fail(ErrExternalIntervalDatalogFragment)
	}
	variables := make([][]uint32, 2)
	sorts := make([][]ast.ASTHandle, 2)
	for index, application := range []ast.ASTHandle{head, tail} {
		declaration, cause := e.manager.GetDecl(application)
		if cause != nil {
			return fail(cause)
		}
		state := e.predicates[declaration]
		if state == nil || e.auxiliary[declaration] || !isExternalIntervalRepresentation(state.requestedRepresentations) {
			return fail(ErrExternalIntervalDatalogFragment)
		}
		count, cause := e.manager.GetNumArgs(application)
		if cause != nil {
			return fail(cause)
		}
		if int(count) != len(state.sorts) || index == 1 && count == 0 {
			return fail(ErrExternalIntervalDatalogFragment)
		}
		if index == 0 {
			headDecl = declaration
		} else {
			tailDecl = declaration
		}
		sorts[index] = state.sorts
		seen := make(map[uint32]bool)
		for column := uint32(0); column < count; column++ {
			argument, cause := e.manager.GetArg(application, column)
			if cause != nil {
				return fail(cause)
			}
			kind, cause := e.manager.GetKind(argument)
			if cause != nil {
				return fail(cause)
			}
			if kind != ast.ASTVar {
				return fail(ErrExternalIntervalDatalogFragment)
			}
			variable, cause := e.manager.GetVarIndex(argument)
			if cause != nil {
				return fail(cause)
			}
			sort, cause := e.manager.GetSort(argument)
			if cause != nil {
				return fail(cause)
			}
			if sort != state.sorts[column] || index == 0 && seen[variable] {
				return fail(ErrExternalIntervalDatalogFragment)
			}
			seen[variable] = true
			variables[index] = append(variables[index], variable)
		}
	}
	if headDecl == tailDecl {
		return fail(ErrExternalIntervalDatalogFragment)
	}
	columns := make(map[uint32][]uint32)
	for column, variable := range variables[1] {
		previous := columns[variable]
		if len(previous) != 0 && sorts[1][previous[0]] != sorts[1][column] {
			return fail(ErrExternalIntervalDatalogFragment)
		}
		columns[variable] = append(previous, uint32(column))
	}
	plan := &externalIntervalDatalogUnaryTransform{}
	order := make([]uint32, 0, len(columns))
	for variable := range columns {
		order = append(order, variable)
	}
	// Source norm_vars compacts these variable-only rules. The unsigned map
	// therefore visits dense variable IDs in numeric order, not column order.
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	for _, variable := range order {
		if group := columns[variable]; len(group) > 1 {
			plan.identical = append(plan.identical, group)
		}
	}
	wanted := make(map[uint32]bool)
	for column, variable := range variables[0] {
		group := columns[variable]
		if len(group) == 0 || sorts[1][group[len(group)-1]] != sorts[0][column] {
			return fail(ErrExternalIntervalDatalogFragment)
		}
		wanted[variable] = true
	}
	surviving := make(map[uint32]uint32)
	for column, variable := range variables[1] {
		group := columns[variable]
		if !wanted[variable] || group[len(group)-1] != uint32(column) {
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
		// As in two-tail assembly, transpositions implement the compiler's
		// permutation with the external declaration's opposite cycle direction.
		for i := 0; i+1 < len(cycle); i++ {
			plan.renames = append(plan.renames, []uint32{cycle[i], cycle[i+1]})
		}
	}
	if len(plan.identical)+len(plan.removed)+len(plan.renames) != 0 {
		transform = plan
	}
	return headDecl, tailDecl, transform, nil
}

func (a *externalIntervalDatalogActivation) setUnaryPending(failure memory.ConstructionFailure, plan *externalIntervalDatalogIdentityRule, completes bool) {
	a.pending, a.pendingCompletesRule = failure, completes
	a.pendingIdentity, a.pendingGround, a.pendingCartesian = nil, nil, nil
	if completes {
		a.pendingIdentity = plan
	}
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalUnary(plan *externalIntervalDatalogIdentityRule, transform *externalIntervalDatalogUnaryTransform) error {
	registers, err := a.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{plan.head, plan.tail})
	if err != nil {
		var pending memory.ConstructionFailure
		if errors.As(err, &pending) {
			a.setUnaryPending(pending, plan, false)
		}
		return err
	}
	owner := &externalIntervalDatalogCartesianContinuation{registers: registers, runtime: registers.runtime}
	rollback := func(cause error) error {
		failure := finishExternalIntervalDatalogCartesianFailure(cause, owner)
		var pending memory.ConstructionFailure
		if errors.As(failure, &pending) {
			a.setUnaryPending(pending, plan, false)
		}
		return failure
	}
	finish := func() error {
		owner.publish = true
		if err := owner.resume(); err != nil {
			failure := newExternalIntervalDatalogCartesianFailure(err, owner)
			a.setUnaryPending(failure, plan, true)
			return failure
		}
		return nil
	}
	current, err := registers.relation(plan.tail)
	if err != nil {
		return rollback(err)
	}
	if current == nil {
		return finish()
	}
	if len(transform.identical) != 0 {
		owner.temporary, err = current.Clone()
		if err != nil {
			return rollback(err)
		}
		current = owner.temporary
		for _, columns := range transform.identical {
			if err := current.FilterIdentical(columns); err != nil {
				return rollback(err)
			}
			empty, err := current.Empty()
			if err != nil {
				return rollback(err)
			}
			if empty {
				return finish()
			}
		}
	}
	if len(transform.removed) != 0 {
		next, err := current.Project(transform.removed)
		if err != nil {
			return rollback(err)
		}
		if err := owner.replaceTemporary(next); err != nil {
			return rollback(err)
		}
		current = next
	}
	for _, cycle := range transform.renames {
		next, err := current.Rename(cycle)
		if err != nil {
			return rollback(err)
		}
		if err := owner.replaceTemporary(next); err != nil {
			return rollback(err)
		}
		current = next
	}
	target, err := registers.ensureRelation(plan.head)
	if err != nil {
		return rollback(err)
	}
	if err := target.Union(current, nil, false); err != nil {
		if !rel.ExternalAssignmentCommitted(err) {
			return rollback(err)
		}
		var nested memory.ConstructionFailure
		if !errors.As(err, &nested) {
			return rollback(fmt.Errorf("%w: committed union has no cleanup owner", ErrExternalIntervalDatalogFragment))
		}
		owner.nested, owner.publish = nested, true
		failure := newExternalIntervalDatalogCartesianFailure(err, owner)
		a.setUnaryPending(failure, plan, true)
		return failure
	}
	return finish()
}
