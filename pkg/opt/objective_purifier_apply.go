package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// Purify runs Begin and PurifyObjective over every objective.
func (p *ObjectivePurifier) Purify(hard []ast.ASTHandle, objectives []DecomposedObjective) (PurifiedObjectiveSet, error) {
	if err := p.Begin(hard); err != nil {
		return PurifiedObjectiveSet{}, err
	}
	for i := range objectives {
		if _, err := p.PurifyObjective(objectives[i]); err != nil {
			return PurifiedObjectiveSet{}, err
		}
	}
	return p.Result(), nil
}

// Begin is from_fmls pushing the hard formulas onto m_hard_constraints.
func (p *ObjectivePurifier) Begin(hard []ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	p.set = PurifiedObjectiveSet{Hard: make([]ast.ASTHandle, 0, len(hard))}
	for _, formula := range hard {
		if _, err := p.holdInto(&p.hardRefs, formula); err != nil {
			return err
		}
		p.set.Hard = append(p.set.Hard, formula)
	}
	return nil
}

// PurifyObjective is from_fmls's per-objective step: mk_atomic on the soft
// terms, purify on an arithmetic term. Definitions join the hard formulas.
func (p *ObjectivePurifier) PurifyObjective(source DecomposedObjective) (DecomposedObjective, error) {
	if err := p.open(); err != nil {
		return DecomposedObjective{}, err
	}
	p.objectiveRefs = append(p.objectiveRefs, purifiedObjectiveRefs{})
	target := DecomposedObjective{SourceKind: source.SourceKind, Offset: *new(big.Rat).Set(&source.Offset), Negate: source.Negate, ID: source.ID, Converted: source.Converted, Weights: cloneRationals(source.Weights)}
	if !source.OriginalTerm.IsNull() {
		if _, err := p.holdTerm(source.OriginalTerm); err != nil {
			return DecomposedObjective{}, err
		}
		target.OriginalTerm = source.OriginalTerm
	}
	if !source.Term.IsNull() {
		term, err := p.arithmeticTerm(source.Term, &p.set.Hard)
		if err != nil {
			return DecomposedObjective{}, err
		}
		if _, err = p.holdTerm(term); err != nil {
			return DecomposedObjective{}, err
		}
		target.Term = term
	}
	target.SoftTerms = make([]ast.ASTHandle, len(source.SoftTerms))
	for j, term := range source.SoftTerms {
		atomic, err := p.atomic(term, &p.set.Hard)
		if err != nil {
			return DecomposedObjective{}, err
		}
		if _, err = p.holdInto(&p.objectiveRefs[len(p.objectiveRefs)-1].terms, atomic); err != nil {
			return DecomposedObjective{}, err
		}
		target.SoftTerms[j] = atomic
	}
	p.set.Objectives = append(p.set.Objectives, target)
	return target, nil
}

// Result is the purified hard formulas and objectives so far.
func (p *ObjectivePurifier) Result() PurifiedObjectiveSet { return p.set }
