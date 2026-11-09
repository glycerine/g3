package rel

import (
	"errors"
	"slices"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

func (r *TableRelation) acceptsInterpretedFact(rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle, fact Fact) (accepted bool, resultErr error) {
	bindings := make([]ast.ASTHandle, tableRelationBindingSize(sorts))
	references := make([]ast.ASTRef, 0, len(sorts)+len(conditions))
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, r.manager.manager.DelRef(&references[index]))
		}
	}()
	for _, variable := range sortedRelVariables(sorts) {
		sort := sorts[variable]
		reference, err := r.manager.DecodeValue(sort, fact[columns[variable]])
		if err != nil {
			return false, err
		}
		references = append(references, reference)
		handle, err := r.manager.manager.RefGet(reference)
		if err != nil {
			return false, err
		}
		bindings[tableRelationBindingIndex(len(bindings), variable)] = handle
	}
	for _, condition := range conditions {
		out, err := r.manager.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return false, err
		}
		references = append(references, out)
		if err = rewriter.RewriteWithBindings(condition, bindings, out); err != nil {
			return false, err
		}
		rewritten, err := r.manager.manager.RefGet(out)
		if err != nil {
			return false, err
		}
		if r.manager.manager.IsFalse(rewritten) {
			return false, nil
		}
		if !r.manager.manager.IsTrue(rewritten) {
			return false, ErrTableRelation
		}
	}
	return true, nil
}

// sortedRelVariables lists variable indexes ascending, as the source walks
// its per-variable vectors (Go map order would be random).
func sortedRelVariables(sorts map[uint32]ast.ASTHandle) []uint32 {
	keys := make([]uint32, 0, len(sorts))
	for variable := range sorts {
		keys = append(keys, variable)
	}
	slices.Sort(keys)
	return keys
}

// handlesByID sorts handles by AST id (deterministic; Go map order would be
// random).
func handlesByID(m *ast.ASTManager, keys []ast.ASTHandle) []ast.ASTHandle {
	ids := make(map[ast.ASTHandle]uint32, len(keys))
	for _, key := range keys {
		id, _ := m.GetASTID(key)
		ids[key] = id
	}
	slices.SortFunc(keys, func(a, b ast.ASTHandle) int { return int(ids[a]) - int(ids[b]) })
	return keys
}
