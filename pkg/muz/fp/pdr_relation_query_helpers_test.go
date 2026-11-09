package fp

import "github.com/glycerine/g3/pkg/ast"

// pdrQueryPredicate is the PDR predicate that answers a relation query: the
// mk_query predicate query!N for a queried relation, else the derived
// predicate itself.
func (e *PropositionalEngine) pdrQueryPredicate(declaration ast.ASTHandle) ast.ASTHandle {
	return e.pdrPredicate(e.pdrQueryTarget(declaration))
}
