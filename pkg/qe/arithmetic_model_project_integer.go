package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c *arithmeticModelProjectCall) integerBranch(entry *arithmeticPluginBounds, target ast.ASTHandle) (uint32, error) {
	policy, err := c.plugin.branchPolicy(entry, target)
	if err != nil {
		return 0, err
	}
	direction := ArithmeticUpperBound
	if policy.Lower {
		direction = ArithmeticLowerBound
	}
	q := c.plugin.util.q()
	bound := unumeral.NewRational()
	index, found, err := c.findMinMax(entry.processor.Bounds(false, direction), policy.Lower, &bound)
	if err = errors.Join(err, q.Del(&bound)); err != nil || !found {
		return 0, err
	}
	return 1 + index, nil
}
