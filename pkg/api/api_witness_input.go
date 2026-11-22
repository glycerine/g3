package api

import "github.com/glycerine/g3/pkg/ast"

// Translation visits shared subterms and quantifier annotations. Bound this
// initial traversal as well as the later ground search. The binder depth is
// part of the key because a shared de Bruijn variable can be free in one root
// and bound in another.
func (s *witnessClauseSet) acceptsInput(parent *ast.ASTManager, assertions []ast.ASTHandle) (bool, error) {
	type entry struct {
		term  ast.ASTHandle
		depth uint32
	}
	var stack []entry
	for _, root := range assertions {
		depth, err := parent.GetDepth(root)
		if err != nil || depth > 192 {
			return false, err
		}
		stack = append(stack, entry{term: root})
	}
	seen := make(map[entry]bool)
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[v] {
			continue
		}
		seen[v] = true
		if len(seen) > 80_000 || !s.charge(1) {
			return false, s.failure()
		}
		kind, _, args, err := parent.NodeShape(v.term)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTVar:
			i, err := parent.GetVarIndex(v.term)
			if err != nil || i >= v.depth {
				return false, err
			}
		case ast.ASTApp:
			if args > 8192 || len(stack)+int(args) > 80_000 {
				return false, nil
			}
			for i := uint32(0); i < args; i++ {
				x, err := parent.GetArg(v.term, i)
				if err != nil {
					return false, err
				}
				stack = append(stack, entry{x, v.depth})
			}
		case ast.ASTQuantifier:
			n, err := parent.GetNumDeclarations(v.term)
			if err != nil || n == 0 || n > maxWitnessBinders {
				return false, err
			}
			level := v.depth + n
			body, err := parent.GetQuantifierBody(v.term)
			if err != nil {
				return false, err
			}
			stack = append(stack, entry{body, level})
			np, err := parent.GetNumPatterns(v.term)
			if err != nil || np > 256 {
				return false, err
			}
			for i := uint32(0); i < np; i++ {
				p, err := parent.GetPattern(v.term, i)
				if err != nil {
					return false, err
				}
				stack = append(stack, entry{p, level})
			}
			nn, err := parent.GetNumNoPatterns(v.term)
			if err != nil || nn > 256 {
				return false, err
			}
			for i := uint32(0); i < nn; i++ {
				p, err := parent.GetNoPattern(v.term, i)
				if err != nil {
					return false, err
				}
				stack = append(stack, entry{p, level})
			}
		default:
			return false, nil
		}
	}
	return true, nil
}
