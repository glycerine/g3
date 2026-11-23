package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
)

type enumerationRefutation struct {
	premises order.EnumerationPremises
	kernel   *orderProofState
}

func (p *enumerationRefutation) release() error {
	if p != nil && p.kernel != nil {
		if err := p.kernel.release(); err != nil {
			return err
		}
		p.kernel = nil
	}
	return nil
}

func (p *enumerationRefutation) check(m *ast.ASTManager, forms []ast.ASTHandle) (bool, error) {
	if len(forms) < 2 || m.TraceStream != nil || m.Limit.CancelFlag() {
		return false, nil
	}
	if !p.premises.Matches(m, forms[:len(forms)-1]) {
		if err := p.release(); err != nil {
			return false, err
		}
	}
	problem, err := p.premises.Project(m, forms)
	if err != nil || problem == nil {
		return false, err
	}
	if p.kernel != nil && p.kernel.needsNewDeclarations(*problem) {
		if err := p.release(); err != nil {
			return false, err
		}
	}
	if p.kernel == nil {
		p.kernel = &orderProofState{finiteEnumeration: true}
		if err := p.kernel.start(m); err != nil {
			return false, errors.Join(err, p.release())
		}
	}
	// The private search budget also covers assertion preprocessing. A
	// failed proof gives the caller its ordinary solver and concrete model.
	proved, err := p.kernel.check(m, *problem, 6_000_000, true)
	if err != nil || !proved {
		return false, errors.Join(err, p.release())
	}
	return true, nil
}
