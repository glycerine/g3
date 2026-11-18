package opt

// Source: src/opt/opt_context.cpp is_maxsat/from_fmls.

import (
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
)

// DecomposedObjective is the internal MaxSAT-shaped form accepted by the
// retained optimization backends. Value = Offset - penalty when Negate is
// true, and Value = Offset + penalty otherwise.
type DecomposedObjective struct {
	SourceKind   FormulaObjectiveKind
	Term         ast.ASTHandle
	OriginalTerm ast.ASTHandle
	SoftTerms    []ast.ASTHandle
	Weights      []numeral.Rational
	Offset       numeral.Rational
	Negate       bool
	ID           string
	Converted    bool
}

type ObjectiveDecompositionStats struct {
	Objectives       int
	ConvertedPBSums  int
	ConvertedBVTerms int
	RetainedMaxSAT   int
	Unconverted      int
	GeneratedTerms   int
}
