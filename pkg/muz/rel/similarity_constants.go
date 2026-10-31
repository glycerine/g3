package rel

// Source: dl_mk_similarity_compressor.cpp constant-position collection,
// stable-constant removal and repeated-column parent detection.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type similarityConstant struct {
	tail     int
	argument uint32
	parent   int
}

func similarityRuleAtom(rule *base.Rule, tail int) (ast.ASTHandle, error) {
	if tail < 0 {
		return rule.Head()
	}
	h, _, e := rule.Tail(tail)
	return h, e
}

func similarityConstantPositions(rule *base.Rule) ([]similarityConstant, error) {
	m, e := rule.ASTManager()
	if e != nil {
		return nil, e
	}
	n, e := rule.PositiveTailSize()
	if e != nil {
		return nil, e
	}
	var result []similarityConstant
	for i := -1; i < n; i++ {
		atom, e := similarityRuleAtom(rule, i)
		if e != nil {
			return nil, e
		}
		arity, e := m.GetNumArgs(atom)
		if e != nil {
			return nil, e
		}
		for j := uint32(0); j < arity; j++ {
			arg, e := m.GetArg(atom, j)
			if e != nil {
				return nil, e
			}
			kind, e := m.GetKind(arg)
			if e != nil {
				return nil, e
			}
			if kind != ast.ASTVar {
				result = append(result, similarityConstant{tail: i, argument: j, parent: -1})
			}
		}
	}
	return result, nil
}

func similarityConstantValue(m *ast.ASTManager, rule *base.Rule, position similarityConstant) (ast.ASTHandle, error) {
	atom, e := similarityRuleAtom(rule, position.tail)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.GetArg(atom, position.argument)
}

// similarityVaryingConstants removes stable columns, then selects the first
// earlier equal column of the same sort as parent. Rows and results borrow ASTs.
func similarityVaryingConstants(rules []*base.Rule) ([]similarityConstant, error) {
	if len(rules) < 2 {
		return nil, ast.ErrASTDomain
	}
	m, e := rules[0].ASTManager()
	if e != nil {
		return nil, e
	}
	positions, e := similarityConstantPositions(rules[0])
	if e != nil {
		return nil, e
	}
	first := make([]ast.ASTHandle, len(positions))
	stable := make([]bool, len(positions))
	for i, p := range positions {
		first[i], e = similarityConstantValue(m, rules[0], p)
		if e != nil {
			return nil, e
		}
		stable[i] = true
	}
	for _, rule := range rules[1:] {
		cmp, e := similarityRoughCompare(rules[0], rule)
		if e != nil {
			return nil, e
		}
		if cmp != 0 {
			return nil, ast.ErrASTDomain
		}
		for i, p := range positions {
			v, e := similarityConstantValue(m, rule, p)
			if e != nil {
				return nil, e
			}
			if v != first[i] {
				stable[i] = false
			}
		}
	}
	var varying []similarityConstant
	for i, p := range positions {
		if !stable[i] {
			varying = append(varying, p)
		}
	}
	// Source considers each earlier column, including one that itself has a
	// parent, and chooses the lowest index still equal in every rule.
	for i := 1; i < len(varying); i++ {
		a, e := similarityConstantValue(m, rules[0], varying[i])
		if e != nil {
			return nil, e
		}
		as, e := m.GetSort(a)
		if e != nil {
			return nil, e
		}
		for j := 0; j < i; j++ {
			b, e := similarityConstantValue(m, rules[0], varying[j])
			if e != nil {
				return nil, e
			}
			bs, e := m.GetSort(b)
			if e != nil {
				return nil, e
			}
			if a != b || as != bs {
				continue
			}
			equal := true
			for _, rule := range rules[1:] {
				x, e := similarityConstantValue(m, rule, varying[i])
				if e != nil {
					return nil, e
				}
				y, e := similarityConstantValue(m, rule, varying[j])
				if e != nil {
					return nil, e
				}
				if x != y {
					equal = false
					break
				}
			}
			if equal {
				varying[i].parent = j
				break
			}
		}
	}
	return varying, nil
}
