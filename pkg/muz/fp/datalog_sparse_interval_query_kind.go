package fp

import "github.com/glycerine/g3/pkg/ast"

// rule_manager::mk_query chooses the first predicate in the flattened body;
// its requested kind is inherited by the fresh query head. Result precision
// belongs to the concrete relation produced during evaluation, not this
// declaration metadata.
func (e *PropositionalEngine) inheritSparseIntervalQueryKind(query, head ast.ASTHandle) error {
	pending := []ast.ASTHandle{query}
	for len(pending) != 0 {
		current := pending[0]
		pending = pending[1:]
		kind, err := e.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind == ast.ASTQuantifier {
			body, err := e.manager.GetQuantifierBody(current)
			if err != nil {
				return err
			}
			pending = append([]ast.ASTHandle{body}, pending...)
			continue
		}
		if kind != ast.ASTApp {
			continue
		}
		operator, err := e.manager.GetDeclKind(current)
		if err != nil {
			return err
		}
		if operator == ast.ASTOpAnd {
			count, err := e.manager.GetNumArgs(current)
			if err != nil {
				return err
			}
			children := make([]ast.ASTHandle, count)
			for i := range children {
				children[i], err = e.manager.GetArg(current, uint32(i))
				if err != nil {
					return err
				}
			}
			pending = append(children, pending...)
			continue
		}
		declaration, err := e.manager.GetDecl(current)
		if err != nil {
			return err
		}
		state := e.predicates[declaration]
		if state == nil {
			continue
		}
		e.predicates[head].requestedRepresentations = append([]string(nil), state.requestedRepresentations...)
		return nil
	}
	return nil
}
