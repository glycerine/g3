package opt

import (
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
)

type pbSum struct {
	terms   []ast.ASTHandle
	weights []numeral.Rational
	offset  numeral.Rational
}
