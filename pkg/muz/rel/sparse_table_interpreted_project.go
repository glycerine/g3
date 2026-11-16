package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpretedAndProject follows the relation manager's default
// transform: clone, apply the interpreted filter, then project the clone.
func (t *SparseTable) FilterInterpretedAndProject(manager *RelationManager, sorts []ast.ASTHandle, rewriter *astrewriter.TheoryRewriter, condition ast.ASTHandle, removed []int) (_ *SparseTable, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	filtered, err := t.Clone()
	if err != nil {
		return nil, err
	}
	// default_table_filter_interpreted_and_project_fn never deallocates its
	// clone t2, so no sparse recycle line is printed for it.
	defer func() { resultErr = errors.Join(resultErr, filtered.release()) }()
	if err = filtered.FilterInterpreted(manager, sorts, rewriter, condition); err != nil {
		return nil, err
	}
	return filtered.Project(removed)
}
