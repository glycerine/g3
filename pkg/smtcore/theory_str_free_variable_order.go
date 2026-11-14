package smtcore

// Source: src/smt/theory_str.cpp get_var_in_eqc and process_free_var's
// representative/leaf/standalone selection before tester generation.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTStringTheory) stringClassVariables(expression ast.ASTHandle) []ast.ASTHandle {
	var variables []ast.ASTHandle
	for _, member := range t.stringClass(expression) {
		if _, variable := t.variableSet[member]; variable {
			variables = append(variables, member)
		}
	}
	stringSortHandles(variables)
	return variables
}

// The two source emission loops call the same tester generator. Return their
// combined visitation order while keeping the actual selection: deduplicate
// registered variables in private classes, then leaf variables before alone
// variables with unknown length. The source obj_hashtable sets are iterated in
// core_hashtable table order (ast.ObjMap); the std::map<expr*,int> input keeps
// Go handle order.
func (t *SMTStringTheory) stringFreeVariableOrder(candidates map[ast.ASTHandle]int) ([]ast.ASTHandle, error) {
	m := t.state.manager
	// obj_hashtable<expr> eqcRepSet, leafVarSet and aloneVars[-1]: iterated in
	// core_hashtable table order.
	representatives := ast.NewObjMap(m)
	for _, candidate := range stringOrderedCounts(candidates) {
		if _, regex := t.regexVariables[candidate]; regex {
			continue
		}
		duplicate := false
		for _, member := range t.stringClassVariables(candidate) {
			seen, err := representatives.Contains(member)
			if err != nil {
				return nil, err
			}
			if seen {
				duplicate = true
				break
			}
		}
		if !duplicate {
			if err := representatives.Insert(candidate, [2]ast.ASTHandle{}); err != nil {
				return nil, err
			}
		}
	}
	leaf, alone := ast.NewObjMap(m), ast.NewObjMap(m)
	for _, entry := range representatives.Entries() {
		variable := entry.Key
		// input_var_in_len has one source insertion, guarded by String sort
		// and is_length simultaneously. Actual length terms have Int sort,
		// so that cache is always empty for retained well-typed consumers.
		if t.state.context.enodes.get(variable) == nil {
			if err := t.state.Internalize(variable, false); err != nil {
				return nil, err
			}
		}
		node := t.state.context.enodes.get(variable)
		if node == nil {
			return nil, ast.ErrASTDomain
		}
		standalone := true
		for _, parent := range node.Parents() {
			concat, err := t.state.manager.IsSequenceConcat(parent.Owner())
			if err != nil {
				return nil, err
			}
			if concat {
				standalone = false
				break
			}
		}
		if standalone {
			_, known, err := t.lengthValue(variable)
			if err != nil {
				return nil, err
			}
			if !known {
				if err := alone.Insert(variable, [2]ast.ASTHandle{}); err != nil {
					return nil, err
				}
				continue
			}
		}
		if err := leaf.Insert(variable, [2]ast.ASTHandle{}); err != nil {
			return nil, err
		}
	}
	var ordered []ast.ASTHandle
	for _, entry := range leaf.Entries() {
		ordered = append(ordered, entry.Key)
	}
	for _, entry := range alone.Entries() {
		ordered = append(ordered, entry.Key)
	}
	return ordered, nil
}
