package api

import "github.com/glycerine/g3/pkg/ast"

const (
	witnessPositive uint8 = 1 << iota
	witnessNegative
	witnessEither = witnessPositive | witnessNegative
)

// A quantified Boolean term can occur under either polarity. The condition of
// an if-then-else and arguments of equality or an uninterpreted function can
// affect the formula in both directions.
func witnessChildPolarity(m *ast.ASTManager, header ast.ASTNodeHeader, parent uint8, arg uint32) uint8 {
	if header.Family != m.BasicFamilyID() {
		return witnessEither
	}
	switch header.DeclKind {
	case ast.ASTOpAnd, ast.ASTOpOr:
		return parent
	case ast.ASTOpIte:
		if arg != 0 {
			return parent
		}
	case ast.ASTOpNot, ast.ASTOpImplies:
		if arg != 0 {
			return parent
		}
		return parent>>1 | (parent&witnessPositive)<<1
	}
	return witnessEither
}

type witnessInstancePlan struct {
	assertions    []ast.ASTHandle
	source        []ast.ASTHandle // Full input to validate when this plan uses a subset.
	sorts         []ast.ASTHandle
	width, rounds int
	selectedOnly  bool // restrict this attempt to the connected premise subset
}

// Look for several quantifiers ranging over the same domain in the final
// assertion. A negated universal supplies a witness for the other quantifiers.
// A negative existential can exclude that witness. Two positive universals
// alone warrant a smaller, single-round attempt.
// These are admission heuristics. The proof itself checks ground formulas.
func planWitnessInstances(m *ast.ASTManager, assertions []ast.ASTHandle) (*witnessInstancePlan, error) {
	if len(assertions) < 2 || len(assertions) > 8192 {
		return nil, nil
	}
	goal := assertions[len(assertions)-1]
	has, err := m.HasQuantifiers(goal)
	if err != nil || !has {
		return nil, err
	}
	type visit struct {
		term     ast.ASTHandle
		polarity uint8
	}
	type occurrences struct {
		universals, existentials uint8
		positiveCount            int
	}
	stack := []visit{{goal, witnessPositive}}
	seen := make(map[ast.ASTHandle]uint8)
	bySort := make(map[ast.ASTHandle]occurrences)
	var sorts []ast.ASTHandle
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		pol := v.polarity &^ seen[v.term]
		if pol == 0 {
			continue
		}
		seen[v.term] |= pol
		if len(seen) > 192 || m.ResourceLimit().CancelFlag() {
			return nil, nil
		}
		head, err := m.NodeHeader(v.term)
		if err != nil {
			return nil, err
		}
		if head.Kind == ast.ASTQuantifier {
			n, err := m.GetNumDeclarations(v.term)
			if err != nil {
				return nil, err
			}
			if n != 1 {
				continue
			}
			st, err := m.GetDeclarationSort(v.term, 0)
			if err != nil {
				return nil, err
			}
			all, err := m.IsForall(v.term)
			if err != nil {
				return nil, err
			}
			row, ok := bySort[st]
			if !ok {
				sorts = append(sorts, st)
			}
			if all {
				row.universals |= pol
				if pol&witnessPositive != 0 {
					row.positiveCount++
				}
			} else {
				row.existentials |= pol
			}
			bySort[st] = row
			continue
		}
		if head.Kind != ast.ASTApp || head.NumArgs > 192 {
			return nil, nil
		}
		for i := uint32(0); i < head.NumArgs; i++ {
			arg, err := m.GetArg(v.term, i)
			if err != nil {
				return nil, err
			}
			nested, err := m.HasQuantifiers(arg)
			if err != nil {
				return nil, err
			}
			if nested {
				stack = append(stack, visit{arg, witnessChildPolarity(m, head, pol, i)})
			}
		}
	}
	p := witnessInstancePlan{assertions: assertions, width: 3, rounds: 1}
	for _, st := range sorts {
		row := bySort[st]
		switch {
		case row.universals == witnessEither && row.existentials&witnessNegative != 0:
			p.width, p.rounds = 6, 3
		case row.universals&witnessNegative != 0 && row.existentials&witnessNegative != 0:
			p.width = max(p.width, 6)
		case row.positiveCount >= 2 && row.universals == witnessPositive && row.existentials == 0:
		default:
			continue
		}
		p.sorts = append(p.sorts, st)
	}
	if len(p.sorts) == 0 {
		return nil, nil
	}
	return &p, nil
}
