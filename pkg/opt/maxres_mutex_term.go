package opt

// Source: src/opt/maxsmt.cpp weighted soft map consumed by MaxRes.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// maxResWeightedTerm is one new_soft entry; weight is owned by the term.
type maxResWeightedTerm struct {
	expression ast.ASTHandle
	weight     numeral.Rational
}
