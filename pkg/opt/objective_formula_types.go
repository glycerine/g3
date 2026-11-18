package opt

// Source: src/opt/opt_context.cpp to_fmls/from_fmls objective markers.

import (
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
)

type FormulaObjectiveKind uint8

const (
	FormulaMaximize FormulaObjectiveKind = iota + 1
	FormulaMinimize
	FormulaMaxSAT
)

type FormulaObjective struct {
	Kind      FormulaObjectiveKind
	Term      ast.ASTHandle
	SoftTerms []ast.ASTHandle
	Weights   []numeral.Rational
	ID        string
}

type DecodedFormulaObjective struct {
	Kind         FormulaObjectiveKind
	Term         ast.ASTHandle
	OriginalTerm ast.ASTHandle
	SoftTerms    []ast.ASTHandle
	Weights      []numeral.Rational
	Offset       numeral.Rational
	Negate       bool
	ID           string
	// Index is the objective's position; from_fmls updates only the
	// objectives whose function applications survive simplification.
	Index int
	// HardPosition counts the hard formulas before this objective's
	// formula: from_fmls pushes its purify definitions after them.
	HardPosition int
}

type DecodedFormulaSet struct {
	Hard       []ast.ASTHandle
	Objectives []DecodedFormulaObjective
}
