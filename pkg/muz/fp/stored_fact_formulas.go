package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// StoredFactFormulas returns owning ground applications for tuples currently
// held by concrete Datalog relation tables. Public source rule enumeration
// deliberately omits these; the explicit Go replay exporter uses them.
func (e *PropositionalEngine) StoredFactFormulas() (result []ast.ASTRef, resultErr error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(e.manager, result))
			result = nil
		}
	}()
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil || state.table == nil {
			continue
		}
		facts, err := state.table.Facts()
		if err != nil {
			return nil, err
		}
		for _, fact := range facts {
			values, err := e.relations.DecodeFact(state.sorts, fact)
			if err != nil {
				return nil, err
			}
			handles := make([]ast.ASTHandle, len(values))
			for i := range values {
				handles[i], err = e.manager.RefGet(values[i])
				if err != nil {
					break
				}
			}
			var application ast.ASTHandle
			if err == nil {
				application, err = e.manager.MkApp(declaration, handles)
			}
			var reference ast.ASTRef
			if err == nil {
				reference, err = e.manager.NewRef(application)
			}
			err = errors.Join(err, closeASTRefs(e.manager, values))
			if err != nil {
				if reference != (ast.ASTRef{}) {
					err = errors.Join(err, e.manager.DelRef(&reference))
				}
				return nil, err
			}
			result = append(result, reference)
		}
	}
	return result, nil
}
