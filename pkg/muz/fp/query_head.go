package fp

// Source: src/muz/base/dl_rule.cpp rule_manager::mk_query free-variable
// collection, reversed query-head domain, and descending de Bruijn arguments.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (e *PropositionalEngine) makeQueryHead(query ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, error) {
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	sort, err := e.manager.GetSort(query)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if sort != boolean {
		return ast.ASTHandle{}, ast.ASTHandle{}, fmt.Errorf("%w: query is not Boolean", ErrPropositionalHorn)
	}
	used, err := ast.NewASTUsedVars(e.manager)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = used.Process(query); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	count := used.MaxFoundVarIndexPlusOne()
	if !used.UsesAllVars(count) {
		return ast.ASTHandle{}, ast.ASTHandle{}, fmt.Errorf("%w: query has a gap in free variables", ErrPropositionalHorn)
	}
	domain := make([]ast.ASTHandle, count)
	arguments := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		domain[i], err = used.Get(count - i - 1)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		arguments[i], err = e.manager.MkVar(count-i-1, domain[i])
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
	}
	prefix, err := e.manager.Symbols.Intern("query")
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	declaration, err := e.manager.MkFreshFuncDecl(prefix, usymbol.Symbol{}, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	head, err := e.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	return declaration, head, nil
}
