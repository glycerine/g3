package tactic

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type signedSquareRoot struct {
	square   int32
	negative bool
}

// Compare signs first and then squared magnitudes. Every input magnitude is
// either a square root of an int32 or the absolute value of an int32.
type rootMagnitude struct {
	squared  int64
	negative bool
}

func squareRootValue(t candidateTerm, roots []signedSquareRoot) rootMagnitude {
	if t.kind == candidateConstant {
		return roots[t.index].magnitude()
	}
	x := int64(t.index)
	if t.kind == candidateSquare {
		x = int64(roots[t.index].square)
	}
	return rootMagnitude{x * x, x < 0}
}

func (v signedSquareRoot) magnitude() rootMagnitude {
	return rootMagnitude{int64(v.square), v.negative}
}

func (x rootMagnitude) less(y rootMagnitude, strict bool) bool {
	if x.negative != y.negative {
		return x.negative
	}
	if x.squared == y.squared {
		return !strict
	}
	return (x.squared < y.squared) != x.negative
}

type rootOrderBound struct {
	right  int32
	strict bool
}

// When every real variable satisfies x*x = c, its ordered domain consists
// of at most two points. Start at each minimum. For x <= y (or x < y), a
// violation forces y up in every solution. Each variable can rise only once.
func (t *NLSATTactic) trySquareRoots(goal *basetactic.Goal, result *basetactic.TacticResult, p *integerCandidate) (bool, error) {
	roots := make([]signedSquareRoot, len(p.variables))
	for i, v := range p.variables {
		if v.numeral.integer {
			return false, nil
		}
		roots[i].square = -1
	}
	for i, c := range p.comparisons {
		if i%64 == 0 && t.manager.Limit.CancelFlag() || c.operation == 0 {
			return false, nil
		}
		left, right := c.left, c.right
		if left.kind == candidateInteger {
			left, right = right, left
		}
		if c.operation == candidateEqual && left.kind == candidateSquare && right.kind == candidateInteger {
			v := &roots[left.index]
			if right.index < 0 || v.square >= 0 && v.square != right.index {
				return t.squareRootConflict(goal, result)
			}
			v.square, v.negative = right.index, right.index > 0
		}
	}
	for _, v := range roots {
		if v.square < 0 {
			return false, nil
		}
	}
	switch p.assignSquareRoots(roots) {
	case lbool.Undef:
		return false, nil
	case lbool.False:
		return t.squareRootConflict(goal, result)
	}
	if t.manager.Limit.CancelFlag() {
		return false, nil
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return false, err
	}
	if !models {
		return finishSatisfiableCandidate(goal, result)
	}
	return t.publishSquareRoots(goal, result, p, roots)
}

func (p *integerCandidate) assignSquareRoots(roots []signedSquareRoot) lbool.LBool {
	var outgoing [][]rootOrderBound
	var queue []int32
	// Ground or unary bounds can be read immediately. Only an order between
	// two direct variable values can propagate a sign change to a neighbor.
	constrain := func(left, right candidateTerm, strict bool) bool {
		if left.kind == candidateConstant && right.kind == candidateConstant {
			if outgoing == nil {
				outgoing = make([][]rootOrderBound, len(roots))
			}
			if len(outgoing[left.index]) == 0 {
				queue = append(queue, left.index)
			}
			outgoing[left.index] = append(outgoing[left.index], rootOrderBound{right.index, strict})
			return true
		}
		x := squareRootValue(left, roots)
		if x.less(squareRootValue(right, roots), strict) {
			return true
		}
		if right.kind != candidateConstant || !roots[right.index].negative {
			return false
		}
		roots[right.index].negative = false
		return x.less(squareRootValue(right, roots), strict)
	}
	for i, c := range p.comparisons {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return lbool.Undef
		}
		l, r := c.left, c.right
		switch c.operation {
		case candidateEqual:
			if !constrain(l, r, false) || !constrain(r, l, false) {
				return lbool.False
			}
		case candidateLT, candidateLE:
			if !constrain(l, r, c.operation == candidateLT) {
				return lbool.False
			}
		case candidateGT, candidateGE:
			if !constrain(r, l, c.operation == candidateGT) {
				return lbool.False
			}
		default:
			return lbool.Undef
		}
	}
	for i := 0; i < len(queue); i++ {
		from := queue[i]
		for j, edge := range outgoing[from] {
			if j%64 == 0 && p.manager.Limit.CancelFlag() {
				return lbool.Undef
			}
			x := roots[from]
			y := &roots[edge.right]
			if x.magnitude().less(y.magnitude(), edge.strict) {
				continue
			}
			if !y.negative {
				return lbool.False
			}
			y.negative = false
			if len(outgoing[edge.right]) != 0 {
				queue = append(queue, edge.right)
			}
		}
	}
	// A later increase may violate an upper bound. Check the complete
	// original system before publishing the least assignment.
	for i, c := range p.comparisons {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return lbool.Undef
		}
		x, y := squareRootValue(c.left, roots), squareRootValue(c.right, roots)
		switch c.operation {
		case candidateEqual:
			if x != y {
				return lbool.False
			}
		case candidateLE, candidateLT:
			if !x.less(y, c.operation == candidateLT) {
				return lbool.False
			}
		case candidateGE, candidateGT:
			if !y.less(x, c.operation == candidateGT) {
				return lbool.False
			}
		}
	}
	return lbool.True
}

func (t *NLSATTactic) squareRootConflict(goal *basetactic.Goal, result *basetactic.TacticResult) (bool, error) {
	cores, err := goal.UnsatCoresEnabled()
	if err != nil || cores || t.manager.Limit.CancelFlag() {
		return false, err
	}
	f, err := t.manager.MkFalse()
	if err != nil {
		return false, err
	}
	if err := goal.Assert(f); err != nil {
		return false, err
	}
	if err := goal.IncDepth(); err != nil {
		return false, err
	}
	return true, result.AddGoal(goal)
}

func (t *NLSATTactic) publishSquareRoots(goal *basetactic.Goal, result *basetactic.TacticResult, p *integerCandidate, roots []signedSquareRoot) (_ bool, err error) {
	m := t.manager
	md, err := model.NewModel(m)
	if err != nil {
		return false, err
	}
	var held basetactic.ModelRef
	if err = held.Set(md.Core()); err != nil {
		return false, errors.Join(err, md.Close())
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	values := make(map[signedSquareRoot]ast.ASTHandle)
	for i, root := range roots {
		if i%64 == 0 && m.Limit.CancelFlag() {
			return false, nil
		}
		h, ok := values[root]
		if !ok {
			h, err = m.ArithmeticSquareRootNumeral(root.square, root.negative)
			if err != nil {
				return false, err
			}
			values[root] = h
		}
		if err = md.Core().RegisterConstant(p.variables[i].declaration, h); err != nil {
			return false, err
		}
	}
	if m.Limit.CancelFlag() {
		return false, nil
	}
	return publishCandidateModel(goal, result, md)
}
