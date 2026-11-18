package arith

// Source: src/tactic/arith/fm_tactic.cpp:1000-1086.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// FMBuildConstraint is the retained add_constraint tail after arithmetic AST
// parsing has produced a concrete linear polynomial.
func FMBuildConstraint(store *FMConstraintStore, variables *FMVariableTable, literals []int32, xs []uint32, coefficients []unumeral.Rational, constant *unumeral.Rational, strict bool, dependency ast.ASTDependency) (*FMConstraint, error) {
	if store == nil || variables == nil || len(xs) != len(coefficients) {
		return nil, ast.ErrASTDomain
	}
	for _, x := range xs {
		if int(x) >= len(variables.Integer) {
			return nil, ast.ErrASTDomain
		}
	}
	c, err := store.New(literals, xs, coefficients, constant, strict, dependency)
	if err != nil {
		return nil, err
	}
	if err = c.MakeInteger(); err != nil {
		return nil, errors.Join(err, store.Delete(c))
	}
	if c.Strict && variables.AllInteger(c) {
		c.Strict = false
		if err = store.q().Dec(&c.Constant); err != nil {
			return nil, errors.Join(err, store.Delete(c))
		}
	}
	return c, nil
}
