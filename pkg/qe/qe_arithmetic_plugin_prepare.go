package qe

import "github.com/glycerine/g3/pkg/ast"

// Prepare retains update_bounds: only atoms containing x are classified and a
// single unrecognized atom rejects the arithmetic plugin for this formula.
func (p *ArithmeticPlugin) Prepare(variable *ast.ContainsApp, formula ast.ASTHandle, positive, negative []ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if variable == nil {
		return false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	key := arithmeticPluginKey{target, formula}
	if p.bounds[key] != nil {
		return true, nil
	}
	processor, err := NewArithmeticBoundsProcessor(p.util)
	if err != nil {
		return false, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = processor.Close()
		}
	}()
	for polarity, atoms := range [][]ast.ASTHandle{positive, negative} {
		matched, cause := p.updateBounds(processor, variable, atoms, polarity == 0)
		if cause != nil {
			return false, cause
		}
		if !matched {
			return false, nil
		}
	}
	if err = p.insertEntry(target, formula, processor); err != nil {
		return false, err
	}
	ok = true
	return true, nil
}

func joinArithmeticErrors(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

// updateBounds is update_bounds(bounds, contains_x, fml, tbl, is_pos): a
// negative atom is bounded as tmp = m.mk_not(e), the app_ref local to the
// call.
func (p *ArithmeticPlugin) updateBounds(processor *ArithmeticBoundsProcessor, variable *ast.ContainsApp, atoms []ast.ASTHandle, positive bool) (matched bool, err error) {
	m := p.manager
	var tmp ast.ASTRef
	defer func() {
		if tmp != (ast.ASTRef{}) {
			err = joinArithmeticErrors(err, m.DelRef(&tmp))
		}
	}()
	for _, atom := range atoms {
		contains, cause := variable.Check(atom)
		if cause != nil {
			return false, cause
		}
		if !contains {
			continue
		}
		e := atom
		if !positive {
			negated, cause := m.MkNot(atom)
			if cause != nil {
				return false, cause
			}
			if tmp == (ast.ASTRef{}) {
				tmp, cause = m.NewRef(negated)
			} else {
				cause = m.SetRef(tmp, negated)
			}
			if cause != nil {
				return false, cause
			}
			e = negated
		}
		ok, cause := processor.Add(variable, e)
		if cause != nil || !ok {
			return false, cause
		}
	}
	return true, nil
}
