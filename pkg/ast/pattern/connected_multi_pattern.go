package pattern

import "github.com/glycerine/g3/pkg/ast"

// A multi-pattern with disjoint groups of variables can enumerate a Cartesian
// product of matches. A further term from the filtered pool can join two or
// more groups before the matcher proposes a substitution. The ordinary
// pattern remains available for a later matching round.
func (c *patternCandidates) connectingTerm(selected, candidates []ast.ASTHandle) (ast.ASTHandle, error) {
	const maxTerms = 12
	if len(selected) < 2 || len(selected) > maxTerms || c.numBindings > 32 || c.numBindings < 3 || len(candidates) > 128 {
		return ast.ASTHandle{}, nil
	}
	variables := func(term ast.ASTHandle) uint32 {
		var mask uint32
		for variable := range c.info[term].freeVars {
			mask |= uint32(1) << variable
		}
		return mask
	}
	var groups [maxTerms]uint32
	count := 0
	for _, term := range selected {
		mask := variables(term)
		for i := 0; i < count; {
			if groups[i]&mask == 0 {
				i++
				continue
			}
			mask |= groups[i]
			count--
			groups[i] = groups[count]
		}
		groups[count] = mask
		count++
	}
	if count < 2 {
		return ast.ASTHandle{}, nil
	}
	// Resolve equal scores by the number of candidate terms with the same
	// declaration. A single relation mentioned many ways can match a large
	// number of tuples. A further tie retains the existing candidate order.
	var best ast.ASTHandle
	maxGroups := 1
	minUses := len(candidates) + 1
	var uses map[ast.ASTHandle]int
	for _, term := range candidates {
		mask := variables(term)
		joined := 0
		for _, group := range groups[:count] {
			if group&mask != 0 {
				joined++
			}
		}
		if joined < 2 || joined < maxGroups {
			continue
		}
		if uses == nil {
			uses = make(map[ast.ASTHandle]int, len(candidates))
			for _, h := range candidates {
				decl, err := c.manager.GetDecl(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				uses[decl]++
			}
		}
		decl, err := c.manager.GetDecl(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		freq := uses[decl]
		if joined > maxGroups || freq < minUses {
			best, maxGroups, minUses = term, joined, freq
		}
	}
	return best, nil
}
