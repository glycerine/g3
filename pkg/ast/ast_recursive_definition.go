package ast

// Source: src/ast/ast.h ast_manager::is_rec_fun_def/rec_fun_qid and
// src/ast/ast.cpp reserved recursive-definition identifier.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

func (m *ASTManager) RecursiveFunctionQID() (usymbol.Symbol, error) {
	if _, err := m.ProofsEnabled(); err != nil {
		return usymbol.Symbol{}, err
	}
	return m.Symbols.Intern(":rec-fun")
}

func (m *ASTManager) IsRecursiveFunctionDefinition(q ASTHandle) (bool, error) {
	qid, _, err := m.GetQuantifierIDs(q)
	if err != nil || qid.IsNull() || qid.IsNumerical() {
		return false, err
	}
	name, present, err := m.Symbols.BareString(qid)
	return present && name == ":rec-fun", err
}

// NoteRecursiveFunctionDefinitions records that a recursive function was
// defined. The model evaluator's AND/OR short-circuit (a deliberate deviation
// that avoids z3 4.7's unbounded unfolding of dead recursive branches) is
// enabled only then, so other evaluations keep the source traversal and
// caching order.
func (m *ASTManager) NoteRecursiveFunctionDefinitions() { m.recursiveDefinitions = true }

// HasRecursiveFunctionDefinitions reports NoteRecursiveFunctionDefinitions.
func (m *ASTManager) HasRecursiveFunctionDefinitions() bool {
	return m != nil && m.recursiveDefinitions
}
