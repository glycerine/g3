package spacer

// Source: unsat_core_plugin_farkas_lemma_optimized in
// src/muz/spacer/spacer_unsat_core_plugin.cpp and .h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// farkasTerm's coefficient is a rational of the AST manager's rational
// manager; a term in m_linear_combinations owns it.
type farkasTerm struct {
	literal     ast.ASTHandle
	coefficient numeral.Rational
}

type UnsatCoreOptimizedFarkasPlugin struct {
	learner      *UnsatCoreLearner
	farkas       *pdr.FarkasLearner
	context      *uparams.ParamsContext
	globals      *uparams.GlobalParamsService
	combinations [][]farkasTerm
	closed       bool
}

func NewUnsatCoreOptimizedFarkasPlugin(learner *UnsatCoreLearner, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*UnsatCoreOptimizedFarkasPlugin, error) {
	if learner == nil || learner.open() != nil || context == nil || globals == nil {
		return nil, ErrUnsatCoreLearnerOwner
	}
	farkas, err := pdr.NewFarkasLearner(learner.manager, context, globals)
	if err != nil {
		return nil, err
	}
	return &UnsatCoreOptimizedFarkasPlugin{learner: learner, farkas: farkas, context: context, globals: globals}, nil
}

func (p *UnsatCoreOptimizedFarkasPlugin) open() error {
	if p == nil || p.closed || p.learner == nil || p.farkas == nil {
		return ErrUnsatCoreLearnerOwner
	}
	return p.learner.open()
}

func (p *UnsatCoreOptimizedFarkasPlugin) ComputePartialCore(step ast.ASTHandle) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	facts, coefficients, matched, err := p.farkas.FarkasPremises(step)
	defer func() { resultErr = errors.Join(resultErr, p.farkas.DelCoefficients(coefficients)) }()
	if err != nil || !matched || p.learner.IsClosed(step) {
		return err
	}
	combination := make([]farkasTerm, 0, len(facts))
	defer func() { resultErr = errors.Join(resultErr, p.delTerms(combination)) }()
	canClose := true
	for i := range facts {
		parent, err := p.learner.manager.GetProofParent(step, uint32(i))
		if err != nil {
			return err
		}
		if !p.learner.IsBOpen(parent) {
			continue
		}
		pure, err := p.learner.OnlyContainsSymbolsB(facts[i])
		if err != nil {
			return err
		}
		if pure && !p.learner.IsHMarked(parent) {
			combination = append(combination, farkasTerm{literal: facts[i], coefficient: coefficients[i]})
			coefficients[i] = numeral.NewRational()
		} else {
			canClose = false
		}
	}
	if !canClose {
		return nil
	}
	if err = p.learner.SetClosed(step, true); err != nil {
		return err
	}
	if len(combination) != 0 {
		p.combinations = append(p.combinations, combination)
		combination = nil
	}
	return nil
}

// delTerms frees the terms' coefficients.
func (p *UnsatCoreOptimizedFarkasPlugin) delTerms(terms []farkasTerm) (err error) {
	q := p.learner.manager.NumeralService().Manager
	for i := range terms {
		err = errors.Join(err, q.Del(&terms[i].coefficient))
	}
	return err
}

// delCombinations frees m_linear_combinations.
func (p *UnsatCoreOptimizedFarkasPlugin) delCombinations() (err error) {
	for _, combination := range p.combinations {
		err = errors.Join(err, p.delTerms(combination))
	}
	p.combinations = nil
	return err
}

func (p *UnsatCoreOptimizedFarkasPlugin) combine(terms []farkasTerm) (resultErr error) {
	literals := make([]ast.ASTHandle, len(terms))
	coefficients := make([]*numeral.Rational, len(terms))
	for i := range terms {
		literals[i] = terms[i].literal
		coefficients[i] = &terms[i].coefficient
	}
	// compute_linear_combination: a local smt::farkas_util, no split literals.
	util, err := smtcore.NewSMTFarkasUtil(p.learner.manager, p.context, p.globals)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, util.Close()) }()
	for i := range literals {
		if err = util.Add(coefficients[i], literals[i]); err != nil {
			return err
		}
	}
	combined, err := p.learner.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.learner.manager.DelRef(&combined)) }()
	if err = util.Get(combined); err != nil {
		return err
	}
	complement, err := p.learner.manager.RefGet(combined)
	if err != nil {
		return err
	}
	negated, err := p.learner.manager.MkNotSimplified(complement) // ast_util mk_not strips the negation
	if err != nil {
		return err
	}
	return p.learner.AddLemmaToCore(negated)
}

func (p *UnsatCoreOptimizedFarkasPlugin) Finalize() (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if len(p.combinations) == 0 {
		return nil
	}
	basis := make([]ast.ASTHandle, 0)
	indices := make(map[ast.ASTHandle]uint32)
	for _, combination := range p.combinations {
		for _, term := range combination {
			if _, found := indices[term.literal]; !found {
				indices[term.literal] = uint32(len(basis))
				basis = append(basis, term.literal)
			}
		}
	}
	matrix := newSpacerMatrix(p.learner.manager.NumeralService().Manager, uint32(len(p.combinations)), uint32(len(basis)))
	defer func() { resultErr = errors.Join(resultErr, matrix.del()) }()
	for row, combination := range p.combinations {
		for i := range combination {
			if err := matrix.set(uint32(row), indices[combination[i].literal], &combination[i].coefficient); err != nil {
				return err
			}
		}
	}
	rank, err := matrix.gaussianElimination()
	if err != nil {
		return err
	}
	for row := uint32(0); row < rank; row++ {
		// The terms borrow the matrix's rationals.
		terms := make([]farkasTerm, 0, len(basis))
		for column, literal := range basis {
			coefficient, err := matrix.get(row, uint32(column))
			if err != nil {
				return err
			}
			if !coefficient.IsZero() {
				terms = append(terms, farkasTerm{literal: literal, coefficient: *coefficient})
			}
		}
		if len(terms) == 0 {
			return ast.ErrASTDomain
		}
		if err = p.combine(terms); err != nil {
			return err
		}
	}
	return p.delCombinations()
}

func (p *UnsatCoreOptimizedFarkasPlugin) Close() error {
	if p == nil {
		return ErrUnsatCoreLearnerOwner
	}
	if p.closed {
		return nil
	}
	err := p.delCombinations()
	p.closed = true
	err = errors.Join(err, p.farkas.Close())
	p.farkas = nil
	p.learner = nil
	return err
}
