package rel

// Source: src/muz/rel/dl_instruction.cpp perform() of instr_join,
// instr_project_rename, instr_union, instr_filter_equal,
// instr_filter_identical, instr_filter_interpreted,
// instr_filter_interpreted_and_project, instr_join_project,
// instr_select_equal_and_project and instr_filter_by_negation for
// relation_base registers: the functor comes from relation_manager::mk_*_fn
// and a missing functor is the "trying to perform unsupported ..."
// default_exception. The register bookkeeping around them (empty sources,
// fast_empty results) stays in the instructions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/z3exception"

	"github.com/glycerine/g3/pkg/ast"
)

// relationOperands returns the relation_base view of the operands of an
// operation. ok is false when every operand is a table_relation's table
// (the table operations apply). Mixing a table_relation with another
// plugin's relation needs the table_relation plugin's relation-level
// operations and the product fallbacks, which this port does not have.
func relationOperands(tables ...FiniteTable) ([]RelationBase, bool, error) {
	any, all := false, true
	result := make([]RelationBase, len(tables))
	for i, table := range tables {
		if table == nil {
			continue
		}
		r, ok := AsRelation(table)
		if ok {
			any = true
			result[i] = r
		} else {
			all = false
		}
	}
	if !any {
		return nil, false, nil
	}
	if !all {
		kinds := make([]string, 0, len(tables))
		for _, table := range tables {
			if table != nil {
				kinds = append(kinds, relationPluginName(table))
			}
		}
		return nil, true, unsupportedRelationOperation("mixed", kinds...)
	}
	return result, true, nil
}

func relationJoin(r1, r2 RelationBase, left, right []int) (FiniteTable, error) {
	fn, err := r1.RelationPlugin().Manager().MkRelationJoinFn(r1, r2, left, right, true)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, unsupportedRelationOperation("join", r1.RelationPlugin().Name(), r2.RelationPlugin().Name())
	}
	return fn(r1, r2)
}

func relationJoinProject(r1, r2 RelationBase, left, right, removed []int) (FiniteTable, error) {
	fn, err := r1.RelationPlugin().Manager().MkRelationJoinProjectFn(r1, r2, left, right, removed, true)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, unsupportedRelationOperation("join-project", r1.RelationPlugin().Name(), r2.RelationPlugin().Name())
	}
	return fn(r1, r2)
}

func relationProject(r RelationBase, removed []int) (FiniteTable, error) {
	fn, err := r.RelationPlugin().Manager().MkRelationProjectFn(r, removed)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, unsupportedRelationOperationOne("project", r.RelationPlugin().Name())
	}
	return fn(r)
}

func relationRename(r RelationBase, cycle []int) (FiniteTable, error) {
	fn, err := r.RelationPlugin().Manager().MkRelationRenameFn(r, cycle)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, unsupportedRelationOperationOne("rename", r.RelationPlugin().Name())
	}
	return fn(r)
}

// relationElement is the relation_element of a table element in the
// column's sort (relation_manager::table_to_relation); the caller owns it.
func relationElement(r RelationBase, value Element, column int) (ast.ASTRef, error) {
	sorts := r.RelationSignature()
	if column < 0 || column >= len(sorts) {
		return ast.ASTRef{}, ErrInstruction
	}
	return r.RelationPlugin().Manager().DecodeRelationElement(sorts[column], value)
}

func relationFilterEqual(r RelationBase, value Element, column int) (result error) {
	m := r.RelationPlugin().Manager()
	ref, err := relationElement(r, value, column)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.manager.DelRef(&ref)) }()
	handle, err := m.manager.RefGet(ref)
	if err != nil {
		return err
	}
	fn, err := m.MkRelationFilterEqualFn(r, handle, column)
	if err != nil {
		return err
	}
	if fn == nil {
		return unsupportedRelationOperationOne("filter_equal", r.RelationPlugin().Name())
	}
	return fn(r)
}

func relationFilterIdentical(r RelationBase, columns []int) error {
	fn, err := r.RelationPlugin().Manager().MkRelationFilterIdenticalFn(r, columns)
	if err != nil {
		return err
	}
	if fn == nil {
		return unsupportedRelationOperationOne("filter_identical", r.RelationPlugin().Name())
	}
	return fn(r)
}

func relationSelectEqualProject(r RelationBase, value Element, column int) (_ FiniteTable, result error) {
	m := r.RelationPlugin().Manager()
	ref, err := relationElement(r, value, column)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, m.manager.DelRef(&ref)) }()
	handle, err := m.manager.RefGet(ref)
	if err != nil {
		return nil, err
	}
	fn, err := m.MkRelationSelectEqualAndProjectFn(r, handle, column)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, unsupportedRelationOperationOne("select_equal_and_project", r.RelationPlugin().Name())
	}
	return fn(r)
}

func relationUnion(target, source, delta RelationBase, widen bool) error {
	m := target.RelationPlugin().Manager()
	var fn RelationUnionFn
	var err error
	if widen {
		fn, err = m.MkRelationWidenFn(target, source, delta)
	} else {
		fn, err = m.MkRelationUnionFn(target, source, delta)
	}
	if err != nil {
		return err
	}
	if fn == nil {
		// instr_union reports widening as a union too.
		op := "union"
		if delta != nil {
			// "<tgt>, <src> and <delta>"
			return unsupportedRelationOperation(op, target.RelationPlugin().Name()+", "+source.RelationPlugin().Name(), delta.RelationPlugin().Name())
		}
		return unsupportedRelationOperation(op, target.RelationPlugin().Name(), source.RelationPlugin().Name())
	}
	return fn(target, source, delta)
}

func relationFilterNegation(target, negative RelationBase, targetColumns, negativeColumns []int) error {
	fn, err := target.RelationPlugin().Manager().MkRelationFilterByNegationFn(target, negative, targetColumns, negativeColumns)
	if err != nil {
		return err
	}
	if fn == nil {
		return z3exception.NewDefaultException("trying to perform unsupported filter_by_negation on relations of kinds " + target.RelationPlugin().Name() + " and " + negative.RelationPlugin().Name())
	}
	return fn(target, negative)
}

func relationFilterInterpreted(r RelationBase, condition ast.ASTHandle) error {
	fn, err := r.RelationPlugin().Manager().MkRelationFilterInterpretedFn(r, condition)
	if err != nil {
		return err
	}
	if fn == nil {
		return unsupportedRelationOperationOne("filter_interpreted", r.RelationPlugin().Name())
	}
	return fn(r)
}

func relationFilterInterpretedAndProject(r RelationBase, condition ast.ASTHandle, removed []int) (FiniteTable, error) {
	fn, err := r.RelationPlugin().Manager().MkRelationFilterInterpretedAndProjectFn(r, condition, removed)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, unsupportedRelationOperationOne("filter_interpreted_and_project", r.RelationPlugin().Name())
	}
	return fn(r)
}

// relationRegisterEmpty is ctx.reg(r)->fast_empty() for a result about to
// be placed into a register.
func relationRegisterEmpty(table FiniteTable) (bool, bool) {
	if r, ok := AsRelation(table); ok {
		return r.FastEmpty(), true
	}
	return false, false
}

// unsupportedRelationOperationOne is dl_instruction.cpp's "trying to
// perform unsupported <op> operation on a relation of kind <k>".
func unsupportedRelationOperationOne(op, kind string) error {
	return z3exception.NewDefaultException("trying to perform unsupported " + op + " operation on a relation of kind " + kind)
}

// finiteEmptySignature reports whether a register value has the empty
// signature.
func finiteEmptySignature(t FiniteTable) (bool, error) {
	if r, ok := AsRelation(t); ok {
		return len(r.RelationSignature()) == 0, nil
	}
	sig, err := t.Signature()
	return sig.Len() == 0, err
}

// finiteEmpty is relation_base::empty() of a register value.
func finiteEmpty(t FiniteTable) (bool, error) {
	if r, ok := AsRelation(t); ok {
		return r.Empty()
	}
	n, err := t.Len()
	return n == 0, err
}

// mixedEmptySignatureJoin is relation_manager::mk_join_fn's
// empty_signature_relation_join_fn for a table_relation joined with
// another plugin's relation when one of them has the empty signature
// (neither plugin's mk_join_fn applies to the pair). ok is false when
// neither operand has the empty signature.
func mixedEmptySignatureJoin(first, second FiniteTable) (_ FiniteTable, ok bool, err error) {
	e1, err := finiteEmptySignature(first)
	if err != nil {
		return nil, true, err
	}
	nullary, other := first, second
	if !e1 {
		e2, err := finiteEmptySignature(second)
		if err != nil || !e2 {
			return nil, err != nil, err
		}
		nullary, other = second, first
	}
	empty, err := finiteEmpty(nullary)
	if err != nil {
		return nil, true, err
	}
	if empty {
		// r.get_manager().mk_empty_relation(r.get_signature(), r.get_kind())
		result, err := emptyFiniteLike(other)
		return result, true, err
	}
	result, err := other.CloneFinite()
	return result, true, err
}
