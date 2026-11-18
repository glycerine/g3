package qe

// Source: src/qe/qe_arith_plugin.cpp bounds_proc.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type ArithmeticBoundsProcessor struct {
	self           *ArithmeticBoundsProcessor
	util           *ArithmeticQETermUtil
	manager        *ast.ASTManager
	marked         map[ast.ASTHandle]bool
	bounds         []*ArithmeticBound
	divisibilities []*ArithmeticDivisibility
	// divZ and divD are m_div_z and m_d: the common Cooper residue, made on
	// the first div_z call and shared by every branch of this (x, fml).
	divZ   ast.ASTRef
	divD   unumeral.Rational // m_d
	closed bool
}

func NewArithmeticBoundsProcessor(util *ArithmeticQETermUtil) (*ArithmeticBoundsProcessor, error) {
	if util == nil {
		return nil, ast.ErrASTHandle
	}
	if err := util.open(); err != nil {
		return nil, err
	}
	p := &ArithmeticBoundsProcessor{util: util, manager: util.manager, marked: make(map[ast.ASTHandle]bool), divD: unumeral.NewRational()}
	p.self = p
	return p, nil
}

func (p *ArithmeticBoundsProcessor) open() error {
	if p == nil || p.self != p || p.util == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	return p.util.open()
}

func (p *ArithmeticBoundsProcessor) Reset() (result error) {
	if err := p.open(); err != nil {
		return err
	}
	for i := len(p.bounds) - 1; i >= 0; i-- {
		result = errors.Join(result, p.bounds[i].Close())
	}
	for i := len(p.divisibilities) - 1; i >= 0; i-- {
		result = errors.Join(result, p.divisibilities[i].Close())
	}
	if p.divZ != (ast.ASTRef{}) {
		result = errors.Join(result, p.manager.DelRef(&p.divZ))
	}
	result = errors.Join(result, p.util.q().Del(&p.divD))
	p.bounds = nil
	p.divisibilities = nil
	p.marked = make(map[ast.ASTHandle]bool)
	return result
}

func (p *ArithmeticBoundsProcessor) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	err := p.Reset()
	if err == nil {
		p.closed = true
	}
	return err
}

func (p *ArithmeticBoundsProcessor) Bounds(strict bool, direction ArithmeticBoundDirection) []*ArithmeticBound {
	if p == nil || p.closed {
		return nil
	}
	result := make([]*ArithmeticBound, 0, len(p.bounds))
	for _, bound := range p.bounds {
		if bound.Strict == strict && bound.Direction == direction {
			result = append(result, bound)
		}
	}
	return result
}

func (p *ArithmeticBoundsProcessor) Divisibilities(nested bool) []*ArithmeticDivisibility {
	if p == nil || p.closed {
		return nil
	}
	result := make([]*ArithmeticDivisibility, 0, len(p.divisibilities))
	for _, constraint := range p.divisibilities {
		if constraint.Nested == nested {
			result = append(result, constraint)
		}
	}
	return result
}

func (p *ArithmeticBoundsProcessor) Add(variable *ast.ContainsApp, atom ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if variable == nil {
		return false, ast.ErrASTHandle
	}
	if p.marked[atom] {
		return true, nil
	}
	if matched, err := p.addNonStrict(variable, atom); err != nil || matched {
		if matched {
			p.marked[atom] = true
		}
		return matched, err
	}
	if matched, err := p.addStrict(variable, atom); err != nil || matched {
		if matched {
			p.marked[atom] = true
		}
		return matched, err
	}
	if matched, err := p.addDivides(variable, atom); err != nil || matched {
		if matched {
			p.marked[atom] = true
		}
		return matched, err
	}
	matched, err := p.addNestedDivides(variable, atom)
	if matched {
		p.marked[atom] = true
	}
	return matched, err
}

// commonDivisor is bounds_proc::div_z: the residue z = (bv2int z_bv) with
// 0 <= z < d for d the lcm of all divisors, made once. ok is false when
// there are no divisibility constraints or d is one.
// d receives m_d (a rational of the manager's NumeralService().Manager).
func (p *ArithmeticBoundsProcessor) commonDivisor(d *unumeral.Rational) (z ast.ASTHandle, ok bool, err error) {
	if err = p.open(); err != nil {
		return z, false, err
	}
	q := p.util.q()
	if p.divZ != (ast.ASTRef{}) {
		if z, err = p.manager.RefGet(p.divZ); err != nil {
			return z, false, err
		}
		return z, true, q.Copy(d, &p.divD)
	}
	if len(p.divisibilities) == 0 {
		return z, false, nil
	}
	if err = cooperLCM(q, p.divisibilities, &p.divD); err != nil {
		return z, false, err
	}
	if one, cause := q.IsOne(&p.divD); cause != nil || one {
		return z, false, cause
	}
	if z, err = p.util.mkBoundedVar(&p.divD); err != nil {
		return z, false, err
	}
	if p.divZ, err = p.manager.NewRef(z); err != nil {
		return z, false, err
	}
	return z, true, q.Copy(d, &p.divD)
}
