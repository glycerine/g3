package fp

// Source: src/muz/base/dl_rule.cpp rule_manager::mk_query free-variable compaction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// hoistQueryExistentials exposes the existential query variables to the
// synthetic rule head, as rule_manager::mk_query does before compaction.
func (e *PropositionalEngine) hoistQueryExistentials(query ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	hasQuantifiers, err := e.manager.HasQuantifiers(query)
	if err != nil {
		return result, err
	}
	if !hasQuantifiers {
		return e.manager.NewRef(query)
	}
	hoister, err := rewriter.NewDefaultQuantifierHoister(e.manager)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, hoister.Close())
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	hoisted, err := hoister.PullQuantifierIndexed(false, query, true, true)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, hoisted.Close()) }()
	return e.manager.CopyRef(hoisted.Formula)
}

// compactQueryVariables removes unused free-variable indices while preserving
// the order and sorts of the variables that occur in the query.
func (e *PropositionalEngine) compactQueryVariables(query ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	used, err := ast.NewASTUsedVars(e.manager)
	if err != nil {
		return result, err
	}
	if err = used.Process(query); err != nil {
		return result, err
	}
	count := used.MaxFoundVarIndexPlusOne()
	if used.UsesAllVars(count) {
		return e.manager.NewRef(query)
	}
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return result, err
	}
	bindings := make([]ast.ASTHandle, count)
	var next uint32
	for i := uint32(0); i < count; i++ {
		sort := used.Contains(i)
		index := uint32(0)
		if sort.IsNull() {
			sort = boolean
		} else {
			index = next
			next++
		}
		bindings[i], err = e.manager.MkVar(index, sort)
		if err != nil {
			return result, err
		}
	}
	substitution, err := rewriter.NewVarSubstitution(e.manager, false)
	defer func() {
		if substitution != nil {
			resultErr = errors.Join(resultErr, substitution.Close())
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	if err != nil {
		return result, err
	}
	result, err = e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = substitution.Substitute(query, bindings, result); err != nil {
		return result, err
	}
	return result, nil
}
