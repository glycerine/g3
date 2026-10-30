package spacer

// Source: src/muz/spacer/spacer_generalizers.cpp lemma_eq_generalizer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *PredicateTransformer) expandEqualityClasses(cube []ast.ASTHandle) (result []ast.ASTRef, dirty bool, resultErr error) {
	equivalence, err := ast.NewExprEquivClass(p.manager)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, equivalence.Close()) }()
	remainder, err := p.manager.FactorEqualities(cube, equivalence)
	if err != nil {
		return nil, false, err
	}
	equalities, dirty, err := p.manager.EquivToExprFull(equivalence)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, result))
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, equalities))
			result = nil
		}
	}()
	result, err = refsFromHandles(p.manager, remainder)
	if err != nil {
		return nil, false, err
	}
	result = append(result, equalities...)
	equalities = nil
	return result, dirty, nil
}

func (p *PredicateTransformer) generalizeEqualityClasses(obligation *ProofObligation, lemma *Lemma) (_ *Lemma, resultErr error) {
	cube, err := lemma.Cube()
	if err != nil {
		return lemma, err
	}
	expanded, dirty, err := p.expandEqualityClasses(cube)
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, expanded)) }()
	if err != nil || !dirty {
		return lemma, err
	}
	level, err := lemma.Level()
	if err != nil {
		return lemma, err
	}
	handles := make([]ast.ASTHandle, len(expanded))
	for i, ref := range expanded {
		handles[i], err = p.manager.RefGet(ref)
		if err != nil {
			return lemma, err
		}
	}
	return NewLemmaFromProofObligation(obligation, handles, level)
}
