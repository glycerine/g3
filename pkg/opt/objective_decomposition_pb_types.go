package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type pbSum struct {
	terms   []ast.ASTHandle
	weights []big.Rat
	offset  big.Rat
}

func (s *pbSum) insert(term ast.ASTHandle, coefficient *big.Rat) {
	if coefficient.Sign() == 0 {
		return
	}
	s.terms = append(s.terms, term)
	s.weights = append(s.weights, *new(big.Rat).Set(coefficient))
}

func ratNeg(v *big.Rat) *big.Rat    { return new(big.Rat).Neg(v) }
func ratMul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
