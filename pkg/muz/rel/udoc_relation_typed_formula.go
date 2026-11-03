package rel

// Source: src/muz/rel/udoc_relation.cpp udoc_relation::to_formula(tbv/doc).

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func typedUDocBitVectorNumeral(m *ast.ASTManager, number *big.Int, width uint32) (_ ast.ASTHandle, resultErr error) {
	service := m.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	if err = service.SetString(&value, number.String()); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.BitVectorNumeral(&value, width)
}

func (r *TypedUDocRelation) cubeFormula(cube *TBV) (ast.ASTHandle, error) {
	m := r.factory.manager
	conjunction := make([]ast.ASTHandle, 0)
	for column, sort := range r.sorts {
		low, limit := r.relation.offsets[column], r.relation.offsets[column+1]
		for bit := low; bit < limit; {
			value, err := r.relation.manager.vectors.Get(cube, bit)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if value == TBitX {
				bit++
				continue
			}
			start := bit
			number := new(big.Int)
			for bit < limit {
				value, err = r.relation.manager.vectors.Get(cube, bit)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if value == TBitX {
					break
				}
				if value == TBit1 {
					number.SetBit(number, int(bit-start), 1)
				}
				bit++
			}
			variable, err := m.MkVar(uint32(column), sort)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			bitVector, err := m.IsBitVectorSort(sort)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			var term, numeral ast.ASTHandle
			if bitVector {
				width := bit - start
				numeral, err = typedUDocBitVectorNumeral(m, number, width)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				term = variable
				if start != low || bit != limit {
					term, err = m.MkBitVectorExtract(bit-1-low, start-low, variable)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
			} else {
				// Source only supports whole-column numerals for finite-domain
				// sorts; partial finite bit-ranges are a BV-only operation.
				if start != low || bit != limit || !number.IsUint64() {
					return ast.ASTHandle{}, ErrDOC
				}
				numeral, err = m.MkDatalogNumeral(number.Uint64(), sort)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				term = variable
			}
			equal, err := m.MkEq(term, numeral)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			conjunction = append(conjunction, equal)
		}
	}
	return m.MkAndArray(conjunction)
}

func (r *TypedUDocRelation) documentFormula(document *Doc) (ast.ASTHandle, error) {
	positive, err := r.cubeFormula(document.positive)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conjunction := []ast.ASTHandle{positive}
	for _, negative := range document.negative.values {
		formula, err := r.cubeFormula(negative)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		formula, err = r.factory.manager.MkNotSimplified(formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		conjunction = append(conjunction, formula)
	}
	return r.factory.manager.MkAndArray(conjunction)
}

// ToFormula returns an owning formula over original column-sort variables.
func (r *TypedUDocRelation) ToFormula() (ast.ASTRef, error) {
	if err := r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	disjunction := make([]ast.ASTHandle, 0, len(r.relation.elems.values))
	for _, document := range r.relation.elems.values {
		formula, err := r.documentFormula(document)
		if err != nil {
			return ast.ASTRef{}, err
		}
		disjunction = append(disjunction, formula)
	}
	formula, err := r.factory.manager.MkOrArray(disjunction)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return r.factory.manager.NewRef(formula)
}
