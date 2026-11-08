package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// The helpers below reproduce the former eager AddRule state (queued raw
// rule plus its Go-only eager clauses) for tests of internal rule-set users.

func (e *PropositionalEngine) addRuleEager(formula ast.ASTHandle, name string) error {
	if err := e.AddRule(formula, name); err != nil {
		return err
	}
	return e.materializePendingRules()
}

func (e *PropositionalEngine) addRuleWithBoundEager(formula ast.ASTHandle, name string, bound uint32) error {
	if err := e.AddRuleWithBound(formula, name, bound); err != nil {
		return err
	}
	return e.materializePendingRules()
}

func (e *PropositionalEngine) addRuleWithSymbolEager(formula ast.ASTHandle, name symbol.Symbol) error {
	if err := e.AddRuleWithSymbol(formula, name); err != nil {
		return err
	}
	return e.materializePendingRules()
}

func (e *PropositionalEngine) addRuleWithSymbolAndBoundEager(formula ast.ASTHandle, name symbol.Symbol, bound uint32) error {
	if err := e.AddRuleWithSymbolAndBound(formula, name, bound); err != nil {
		return err
	}
	return e.materializePendingRules()
}
