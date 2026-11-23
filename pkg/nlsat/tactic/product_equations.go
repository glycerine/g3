package tactic

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type productEquation struct {
	other int32
	value int32
}

type productSystem struct {
	candidate *integerCandidate
	edges     [][]productEquation
	signs     []int8 // a suggested sign from comparison with zero
}

type rationalRadical struct {
	numerator, denominator int32
	negative               bool
}

// In a connected system of nonzero products x*y=k, each variable is either
// a*t or a/t for a rational a and the component's first variable t. An odd
// cycle fixes t². An even cycle checks a rational equality. A square x*x=k
// is a cycle of length one.
func (t *NLSATTactic) tryProductEquations(goal *basetactic.Goal, result *basetactic.TacticResult, forms []ast.ASTHandle, p *integerCandidate) (bool, error) {
	for _, v := range p.variables {
		if v.numeral.integer {
			return false, nil
		}
	}
	s := productSystem{candidate: p, edges: make([][]productEquation, len(p.variables)), signs: make([]int8, len(p.variables))}
	for i, cmp := range p.comparisons {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return false, nil
		}
		if cmp.operation == 0 {
			// The scalar comparison table leaves products of different variables
			// unclassified. Accept only a top-level nonzero product equation.
			ok, err := s.readEquation(forms[i])
			if err != nil || !ok {
				return false, err
			}
			continue
		}
		l, r := cmp.left, cmp.right
		variableOnLeft := l.kind != candidateInteger
		if !variableOnLeft {
			l, r = r, l
		}
		if r.kind != candidateInteger {
			continue
		}
		if l.kind == candidateSquare && cmp.operation == candidateEqual {
			if r.index < 0 {
				return t.squareRootConflict(goal, result)
			}
			if r.index == 0 {
				return false, nil
			}
			s.edges[l.index] = append(s.edges[l.index], productEquation{l.index, r.index})
		}
		if l.kind == candidateConstant && r.index == 0 && cmp.operation != candidateEqual {
			positive := cmp.operation == candidateGT || cmp.operation == candidateGE
			if positive == variableOnLeft {
				s.signs[l.index] = 1
			} else {
				s.signs[l.index] = -1
			}
		}
	}
	roots := make([]rationalRadical, len(p.variables))
	switch s.solve(roots) {
	case lbool.False:
		return t.squareRootConflict(goal, result)
	case lbool.Undef:
		return false, nil
	}
	for i, cmp := range p.comparisons {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return false, nil
		}
		// solve checked every product equality, including all back edges.
		if cmp.operation != 0 && !productComparisonHolds(cmp, roots) {
			return false, nil
		}
	}
	if p.manager.Limit.CancelFlag() {
		return false, nil
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return false, err
	}
	if !models {
		return finishSatisfiableCandidate(goal, result)
	}
	return t.publishSquareRootModel(goal, result, p, func(i int) rationalRadical { return roots[i] })
}

func (s *productSystem) readEquation(form ast.ASTHandle) (bool, error) {
	p := s.candidate
	m := p.manager
	head, err := m.NodeHeader(form)
	if err != nil || head.Family != m.BasicFamilyID() || head.DeclKind != ast.ASTOpEq || head.NumArgs != 2 {
		return false, err
	}
	l, err := m.GetArg(form, 0)
	if err != nil {
		return false, err
	}
	r, err := m.GetArg(form, 1)
	if err != nil {
		return false, err
	}
	if p.terms[l].kind == candidateInteger {
		l, r = r, l
	}
	target := p.terms[r]
	if target.kind != candidateInteger || target.index == 0 {
		return false, nil
	}
	head, err = m.NodeHeader(l)
	if err != nil || head.Family != p.arithmetic || head.DeclKind != candidateMul || head.NumArgs != 2 {
		return false, err
	}
	x, err := m.GetArg(l, 0)
	if err != nil {
		return false, err
	}
	y, err := m.GetArg(l, 1)
	if err != nil {
		return false, err
	}
	a, b := p.terms[x], p.terms[y]
	if a.kind != candidateConstant || b.kind != candidateConstant {
		return false, nil
	}
	s.edges[a.index] = append(s.edges[a.index], productEquation{b.index, target.index})
	s.edges[b.index] = append(s.edges[b.index], productEquation{a.index, target.index})
	return true, nil
}

// A nil coefficient marks an unvisited variable. All rationals are local to
// this invocation. The size bound limits the cost of an unsuccessful attempt.
type productCoordinate struct {
	factor     *big.Rat
	reciprocal bool
}

func (s *productSystem) solve(roots []rationalRadical) lbool.LBool {
	coordinates := make([]productCoordinate, len(roots))
	var queue []int32
	for seed := range roots {
		if coordinates[seed].factor != nil {
			continue
		}
		coordinates[seed].factor = new(big.Rat).SetInt64(1)
		queue = append(queue[:0], int32(seed))
		var square *big.Rat
		var sign int8
		for head := 0; head < len(queue); head++ {
			i := queue[head]
			if head%64 == 0 && s.candidate.manager.Limit.CancelFlag() {
				return lbool.Undef
			}
			from := coordinates[i]
			if want := s.signs[i]; want != 0 {
				if from.factor.Sign() < 0 {
					want = -want
				}
				if sign != 0 && sign != want {
					// All vertices in this component have a nonzero incident product.
					return lbool.False
				}
				sign = want
			}
			for j, edge := range s.edges[i] {
				if j%256 == 0 && s.candidate.manager.Limit.CancelFlag() {
					return lbool.Undef
				}
				next := new(big.Rat).SetInt64(int64(edge.value))
				next.Quo(next, from.factor)
				if next.Num().BitLen() > 64 || next.Denom().BitLen() > 64 {
					return lbool.Undef
				}
				to := coordinates[edge.other]
				if to.factor == nil {
					coordinates[edge.other] = productCoordinate{factor: next, reciprocal: !from.reciprocal}
					queue = append(queue, edge.other)
					continue
				}
				if to.reciprocal != from.reciprocal {
					if to.factor.Cmp(next) != 0 {
						return lbool.False
					}
					continue
				}
				if from.reciprocal {
					next.Quo(to.factor, next)
				} else {
					next.Quo(next, to.factor)
				}
				if next.Sign() <= 0 || square != nil && square.Cmp(next) != 0 {
					return lbool.False
				}
				square = next
			}
		}
		if square == nil {
			square = new(big.Rat).SetInt64(1)
		}
		for j, i := range queue {
			if j%64 == 0 && s.candidate.manager.Limit.CancelFlag() {
				return lbool.Undef
			}
			c := coordinates[i]
			var magnitude big.Rat
			magnitude.Mul(c.factor, c.factor)
			if c.reciprocal {
				magnitude.Quo(&magnitude, square)
			} else {
				magnitude.Mul(&magnitude, square)
			}
			if magnitude.Num().BitLen() > 31 || magnitude.Denom().BitLen() > 31 {
				return lbool.Undef
			}
			roots[i] = rationalRadical{
				numerator: int32(magnitude.Num().Int64()), denominator: int32(magnitude.Denom().Int64()),
				negative: (c.factor.Sign() < 0) != (sign < 0),
			}
		}
	}
	return lbool.True
}

// Compare signs and squared magnitudes to check the original scalar bounds.
// A failed bound may have another witness for a component with a free scale.
func productComparisonHolds(c candidateComparison, roots []rationalRadical) bool {
	var left, right big.Rat
	leftNegative := productTermMagnitude(c.left, roots, &left)
	rightNegative := productTermMagnitude(c.right, roots, &right)
	order := left.Cmp(&right)
	switch {
	case leftNegative && rightNegative:
		order = -order
	case leftNegative:
		order = -1
	case rightNegative:
		order = 1
	}
	switch c.operation {
	case candidateEqual:
		return order == 0
	case candidateLT:
		return order < 0
	case candidateLE:
		return order <= 0
	case candidateGT:
		return order > 0
	case candidateGE:
		return order >= 0
	}
	return false
}

func productTermMagnitude(t candidateTerm, roots []rationalRadical, out *big.Rat) (negative bool) {
	if t.kind == candidateInteger {
		n := int64(t.index)
		out.SetInt64(n * n)
		return n < 0
	}
	v := roots[t.index]
	out.SetFrac64(int64(v.numerator), int64(v.denominator))
	if t.kind == candidateSquare {
		out.Mul(out, out)
		return false
	}
	return v.negative
}
