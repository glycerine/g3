package qe

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func truncateLiteVector(m *ast.ASTManager, v *[]ast.ASTRef, n int) error {
	for len(*v) > n {
		i := len(*v) - 1
		if (*v)[i] != (ast.ASTRef{}) {
			if err := m.DelRef(&(*v)[i]); err != nil {
				return err
			}
		}
		*v = (*v)[:i]
	}
	return nil
}
func (l *Lite) expressions(v []ast.ASTRef) ([]ast.ASTHandle, error) {
	hs := make([]ast.ASTHandle, len(v))
	for i, ref := range v {
		h, err := l.manager.RefGet(ref)
		if err != nil {
			return nil, err
		}
		hs[i] = h
	}
	return hs, nil
}
func (l *Lite) replaceVector(c *liteCall, v *[]ast.ASTRef, hs []ast.ASTHandle) error {
	next, err := c.vector(hs)
	if err != nil {
		return err
	}
	if err = truncateLiteVector(l.manager, v, 0); err != nil {
		return err
	}
	*v = *next
	*next = nil
	return nil
}
func (l *Lite) uniqueNonGround(v []ast.ASTRef) (int, bool, error) {
	if len(v) <= 1 {
		return 0, false, nil
	}
	index := -1
	for i, ref := range v {
		h, err := l.manager.RefGet(ref)
		if err != nil {
			return 0, false, err
		}
		ground, err := l.manager.IsGround(h)
		if err != nil {
			return 0, false, err
		}
		if !ground {
			if index >= 0 {
				return 0, false, nil
			}
			index = i
		}
	}
	return index, index >= 0, nil
}
func (l *Lite) traceVector(line int, v []ast.ASTRef) error {
	tags := l.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe_lite", "operator()", "src/qe/qe_lite.cpp", line, func() bool { return true }, func(out io.Writer) error {
		writer := traceBodyWriter{out}
		for _, ref := range v {
			h, err := l.manager.RefGet(ref)
			if err != nil {
				return err
			}
			if err = writeExpression(l.manager, l.config, writer, h); err != nil {
				return err
			}
			_, _ = io.WriteString(writer, "\n")
		}
		return nil
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// EliminateConjunctions selects indices in the set when indexOfBound is true,
// or outside it otherwise. The resized slice and every live ref remain owned
// by the caller, including completed vector writes on error.
func (l *Lite) EliminateConjunctions(indices []uint32, indexOfBound bool, forms *[]ast.ASTRef) (err error) {
	c, err := l.begin()
	if err != nil {
		return err
	}
	defer l.finish(c, &err)
	if forms == nil {
		return ast.ErrASTHandle
	}
	return l.eliminateConjunctions(c, indices, indexOfBound, forms)
}
func (l *Lite) eliminateConjunctions(c *liteCall, indices []uint32, indexOfBound bool, forms *[]ast.ASTRef) error {
	hs, err := l.expressions(*forms)
	if err != nil {
		return err
	}
	hs, err = l.manager.FlattenAnd(hs)
	if err != nil {
		return err
	}
	if err = l.replaceVector(c, forms, hs); err != nil {
		return err
	}
	index, unique, err := l.uniqueNonGround(*forms)
	if err != nil {
		return err
	}
	if unique {
		h, err := l.manager.RefGet((*forms)[index])
		if err != nil {
			return err
		}
		// expr_ref fml is scoped to this block and released on return,
		// after fmls[index] = fml.
		tmp, err := l.manager.NewRef(h)
		if err != nil {
			return err
		}
		if err = l.eliminateExpression(c, indices, indexOfBound, tmp); err == nil {
			if h, err = l.manager.RefGet(tmp); err == nil {
				err = l.manager.SetRef((*forms)[index], h)
			}
		}
		return errors.Join(err, l.manager.DelRef(&tmp))
	}
	if err = l.traceVector(2464, *forms); err != nil {
		return err
	}
	v, err := NewVariableSetTest(l.manager, indices, indexOfBound)
	if err != nil {
		return err
	}
	c.selectors = append(c.selectors, v)
	if err = l.der.SetVariableTest(v); err != nil {
		return err
	}
	if err = l.fm.SetVariableTest(v); err != nil {
		return err
	}
	if err = l.array.SetVariableTest(v); err != nil {
		return err
	}
	if err = l.der.EliminateConjunctions(forms); err != nil {
		return err
	}
	if err = l.fm.Apply(forms); err != nil {
		return err
	}
	if l.useArray {
		if err = l.array.Apply(*forms); err != nil {
			return err
		}
	}
	return l.traceVector(2475, *forms)
}

// EliminateIndices is the source expression overload: distribute the phase
// pipeline over flattened disjuncts, then rebuild with configured Boolean rules.
func (l *Lite) EliminateIndices(indices []uint32, indexOfBound bool, formula ast.ASTRef) (err error) {
	c, err := l.begin()
	if err != nil {
		return err
	}
	defer l.finish(c, &err)
	return l.eliminateExpression(c, indices, indexOfBound, formula)
}
func (l *Lite) eliminateExpression(c *liteCall, indices []uint32, indexOfBound bool, formula ast.ASTRef) (err error) {
	m := l.manager
	h, err := m.RefGet(formula)
	if err != nil {
		return err
	}
	hs, err := m.FlattenOr([]ast.ASTHandle{h})
	if err != nil {
		return err
	}
	// expr_ref_vector disjs is a local: ~ref_vector releases it in forward
	// order when the function returns.
	disjs := new([]ast.ASTRef)
	defer func() { err = errors.Join(err, releaseLiteVectorForward(m, disjs)) }()
	if err = appendLiteVector(m, disjs, hs); err != nil {
		return err
	}
	for i := range *disjs {
		if err = l.eliminateDisjunct(c, indices, indexOfBound, formula, (*disjs)[i]); err != nil {
			return err
		}
	}
	args, err := l.expressions(*disjs)
	if err != nil {
		return err
	}
	boolean, err := c.boolean()
	if err != nil {
		return err
	}
	return boolean.MkOr(args, formula)
}

// eliminateDisjunct is one loop body of the expression overload; its
// expr_ref_vector conjs dies at the end of the iteration, after disjs[i] = fml.
func (l *Lite) eliminateDisjunct(c *liteCall, indices []uint32, indexOfBound bool, formula, disj ast.ASTRef) (err error) {
	m := l.manager
	h, err := m.RefGet(disj)
	if err != nil {
		return err
	}
	conjs := new([]ast.ASTRef)
	defer func() { err = errors.Join(err, releaseLiteVectorForward(m, conjs)) }()
	if err = appendLiteVector(m, conjs, []ast.ASTHandle{h}); err != nil {
		return err
	}
	if err = l.eliminateConjunctions(c, indices, indexOfBound, conjs); err != nil {
		return err
	}
	args, err := l.expressions(*conjs)
	if err != nil {
		return err
	}
	boolean, err := c.boolean()
	if err != nil {
		return err
	}
	if err = boolean.MkAnd(args, formula); err != nil {
		return err
	}
	h, err = m.RefGet(formula)
	if err != nil {
		return err
	}
	return m.SetRef(disj, h)
}

// appendLiteVector pushes new references onto a caller-owned vector.
func appendLiteVector(m *ast.ASTManager, v *[]ast.ASTRef, hs []ast.ASTHandle) error {
	for _, h := range hs {
		ref, err := m.NewRef(h)
		if err != nil {
			return err
		}
		*v = append(*v, ref)
	}
	return nil
}

// releaseLiteVectorForward is ~ref_vector: dec_ref from first to last.
func releaseLiteVectorForward(m *ast.ASTManager, v *[]ast.ASTRef) (result error) {
	for i := range *v {
		if (*v)[i] != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&(*v)[i]))
		}
	}
	*v = (*v)[:0]
	return result
}
