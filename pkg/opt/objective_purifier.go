package opt

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type ObjectivePurifier struct {
	manager *ast.ASTManager
	arith   ast.FamilyID
	filter  *tactic.FilterModelConverter
	refs    []ast.ASTRef
	stats   ObjectivePurificationStats
	closed  bool
	// detached: the filter (fm) was handed to m_model_converter, whose
	// reference releases it.
	detached bool
}

func NewObjectivePurifier(manager *ast.ASTManager) (*ObjectivePurifier, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	arith, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	filter, err := tactic.NewFilterModel(manager)
	if err != nil {
		return nil, err
	}
	return &ObjectivePurifier{manager: manager, arith: arith, filter: filter}, nil
}

func (p *ObjectivePurifier) open() error {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	if p.filter == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (p *ObjectivePurifier) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := p.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.refs = append(p.refs, ref)
	return h, nil
}

func (p *ObjectivePurifier) Statistics() (ObjectivePurificationStats, error) {
	if err := p.open(); err != nil {
		return ObjectivePurificationStats{}, err
	}
	return p.stats, nil
}

func (p *ObjectivePurifier) Close() (result error) {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.filter != nil && p.detached {
		p.filter = nil
	} else if p.filter != nil {
		result = p.filter.Close()
		if result == nil {
			p.filter = nil
		}
	}
	for i := len(p.refs) - 1; i >= 0; i-- {
		if p.refs[i] == (ast.ASTRef{}) {
			continue
		}
		err := p.manager.DelRef(&p.refs[i])
		result = errors.Join(result, err)
		if err != nil {
			break
		}
	}
	if result == nil {
		p.refs = nil
		p.closed = true
	}
	return result
}

// DetachFilter hands one reference to the filter model converter (fm) to
// the caller: purify's `m_model_converter = concat(m_model_converter, fm)`.
// It returns nil when purification introduced no fresh declaration.
func (p *ObjectivePurifier) DetachFilter() (tactic.ModelConverter, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if p.stats.FreshDeclarations == 0 {
		return nil, nil
	}
	if err := p.filter.IncRef(); err != nil {
		return nil, err
	}
	p.detached = true
	return p.filter, nil
}
