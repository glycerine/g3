package qe

import "github.com/glycerine/g3/pkg/ast"

// arithmeticPluginBounds is an m_bounds_cache entry; the trail owns its
// (x, fml) key.
type arithmeticPluginBounds struct {
	processor *ArithmeticBoundsProcessor
}

func (e *arithmeticPluginBounds) close(m *ast.ASTManager) error {
	if e == nil {
		return nil
	}
	return e.processor.Close()
}

func (p *ArithmeticPlugin) entry(variable *ast.ContainsApp, formula ast.ASTHandle) (*arithmeticPluginBounds, ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, ast.ASTHandle{}, err
	}
	if variable == nil {
		return nil, ast.ASTHandle{}, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	entry := p.bounds[arithmeticPluginKey{target, formula}]
	if entry == nil {
		return nil, target, ast.ErrASTDomain
	}
	return entry, target, nil
}

func (p *ArithmeticPlugin) insertEntry(target, formula ast.ASTHandle, processor *ArithmeticBoundsProcessor) (resultErr error) {
	// m_trail.push_back(x); m_trail.push_back(fml);
	// m_bounds_cache.insert(x, fml, bounds).
	if err := p.pushTrail(target, formula); err != nil {
		return err
	}
	hash, err := p.boundsTable.PairHash(target, formula)
	if err != nil {
		return err
	}
	p.boundsTable.InsertPairKeys(target, formula, hash)
	p.bounds[arithmeticPluginKey{target, formula}] = &arithmeticPluginBounds{processor: processor}
	return nil
}

// pushTrail is m_trail.push_back for each expression.
func (p *ArithmeticPlugin) pushTrail(expressions ...ast.ASTHandle) error {
	for _, e := range expressions {
		ref, err := p.manager.NewRef(e)
		if err != nil {
			return err
		}
		p.trail = append(p.trail, ref)
	}
	return nil
}
