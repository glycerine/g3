package rel

// Source: src/muz/rel/dl_instruction.h instruction::m_fn_cache, find_fn and
// store_fn; dl_instruction.cpp instr_join, instr_filter_equal,
// instr_filter_identical, instr_union, instr_project_rename,
// instr_join_project, instr_select_equal_and_project and
// instr_filter_by_negation, which make their relation functor once per
// combination of operand kinds (relation_base::get_kind()) and reuse it.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// relationFnCache is an instruction's m_fn_cache for the relation
// functors (one instruction makes one kind of functor).
type relationFnCache struct {
	join     map[[2]int]RelationJoinFn
	unions   map[[3]int]RelationUnionFn
	negation map[[2]int]RelationIntersectionFilterFn
	mutator  map[int]RelationMutatorFn
	trans    map[int]RelationTransformerFn
	// value is the instruction's m_value: the relation element of a
	// filter_equal / select_equal_and_project, made once and held by the
	// instruction (and its cached functor) until the instruction closes.
	value        ast.ASTRef
	valueManager *ast.ASTManager
}

// noDeltaKind marks find_fn(r_tgt, r_src) (no delta) in the union key.
const noDeltaKind = -2

func relationKind(r RelationBase) int {
	return r.RelationPlugin().Manager().RelationKindOf(r)
}

func (c *relationFnCache) join2(r1, r2 RelationBase, left, right, removed []int, project bool) (FiniteTable, error) {
	key := [2]int{relationKind(r1), relationKind(r2)}
	fn := c.join[key]
	if fn == nil {
		m := r1.RelationPlugin().Manager()
		var err error
		if project {
			fn, err = m.MkRelationJoinProjectFn(r1, r2, left, right, removed, true)
		} else {
			fn, err = m.MkRelationJoinFn(r1, r2, left, right, true)
		}
		if err != nil {
			return nil, err
		}
		if fn == nil {
			op := "join"
			if project {
				op = "join-project"
			}
			return nil, unsupportedRelationOperation(op, r1.RelationPlugin().Name(), r2.RelationPlugin().Name())
		}
		if c.join == nil {
			c.join = make(map[[2]int]RelationJoinFn)
		}
		c.join[key] = fn
	}
	return unwrapResult(fn(r1, r2))
}

func (c *relationFnCache) transform(r RelationBase, op string, mk func() (RelationTransformerFn, error)) (FiniteTable, error) {
	key := relationKind(r)
	fn := c.trans[key]
	if fn == nil {
		var err error
		if fn, err = mk(); err != nil {
			return nil, err
		}
		if fn == nil {
			return nil, unsupportedRelationOperationOne(op, r.RelationPlugin().Name())
		}
		if c.trans == nil {
			c.trans = make(map[int]RelationTransformerFn)
		}
		c.trans[key] = fn
	}
	return unwrapResult(fn(r))
}

func (c *relationFnCache) mutate(r RelationBase, op string, mk func() (RelationMutatorFn, error)) error {
	key := relationKind(r)
	fn := c.mutator[key]
	if fn == nil {
		var err error
		if fn, err = mk(); err != nil {
			return err
		}
		if fn == nil {
			return unsupportedRelationOperationOne(op, r.RelationPlugin().Name())
		}
		if c.mutator == nil {
			c.mutator = make(map[int]RelationMutatorFn)
		}
		c.mutator[key] = fn
	}
	return fn(r)
}

// valueHandle is m_value for the relation r's column sort, made at the
// first relation perform.
func (c *relationFnCache) valueHandle(r RelationBase, value Element, column int) (ast.ASTHandle, error) {
	m := r.RelationPlugin().Manager()
	if c.valueManager == nil {
		ref, err := relationElement(r, value, column)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		c.value, c.valueManager = ref, m.manager
	}
	return c.valueManager.RefGet(c.value)
}

func (c *relationFnCache) filterEqual(r RelationBase, value Element, column int) error {
	if c.mutator[relationKind(r)] == nil {
		if _, err := c.valueHandle(r, value, column); err != nil {
			return err
		}
	}
	return c.mutate(r, "filter_equal", func() (RelationMutatorFn, error) {
		h, err := c.valueManager.RefGet(c.value)
		if err != nil {
			return nil, err
		}
		return r.RelationPlugin().Manager().MkRelationFilterEqualFn(r, h, column)
	})
}

func (c *relationFnCache) selectEqualProject(r RelationBase, value Element, column int) (FiniteTable, error) {
	if c.trans[relationKind(r)] == nil {
		if _, err := c.valueHandle(r, value, column); err != nil {
			return nil, err
		}
	}
	return c.transform(r, "select_equal_and_project", func() (RelationTransformerFn, error) {
		h, err := c.valueManager.RefGet(c.value)
		if err != nil {
			return nil, err
		}
		return r.RelationPlugin().Manager().MkRelationSelectEqualAndProjectFn(r, h, column)
	})
}

func (c *relationFnCache) union(target, source, delta RelationBase, widen bool) error {
	key := [3]int{relationKind(target), relationKind(source), noDeltaKind}
	if delta != nil {
		key[2] = relationKind(delta)
	}
	fn := c.unions[key]
	if fn == nil {
		m := target.RelationPlugin().Manager()
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
			if delta != nil {
				return unsupportedRelationOperation("union", target.RelationPlugin().Name()+", "+source.RelationPlugin().Name(), delta.RelationPlugin().Name())
			}
			return unsupportedRelationOperation("union", target.RelationPlugin().Name(), source.RelationPlugin().Name())
		}
		if c.unions == nil {
			c.unions = make(map[[3]int]RelationUnionFn)
		}
		c.unions[key] = fn
	}
	return fn(target, source, delta)
}

func (c *relationFnCache) filterNegation(target, negative RelationBase, targetColumns, negativeColumns []int) error {
	key := [2]int{relationKind(target), relationKind(negative)}
	fn := c.negation[key]
	if fn == nil {
		var err error
		if fn, err = target.RelationPlugin().Manager().MkRelationFilterByNegationFn(target, negative, targetColumns, negativeColumns); err != nil {
			return err
		}
		if fn == nil {
			return unsupportedRelationNegation(target, negative)
		}
		if c.negation == nil {
			c.negation = make(map[[2]int]RelationIntersectionFilterFn)
		}
		c.negation[key] = fn
	}
	return fn(target, negative)
}

// close releases m_value (the instruction's destructor).
func (c *relationFnCache) close() error {
	c.join, c.unions, c.negation, c.mutator, c.trans = nil, nil, nil, nil, nil
	if c.valueManager == nil {
		return nil
	}
	m := c.valueManager
	c.valueManager = nil
	return errors.Join(m.DelRef(&c.value))
}
