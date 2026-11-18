package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// realBranch ports arith_plugin::project_real. z3 4.7.1 bug kept out of g3:
// project_real picks the bound family from raw counts
// (le + lt < ge + gt), but assign and subst read the branch in the family
// get_bound_sizes picks, which doubles the non-strict counts of a real, so
// the two disagree and qe-sat with projection_mode false never terminates
// (qe_arith_plugin.cpp:1920-1955; AUDIT.md qe_01-F2;
// test_vectors/z3_bug_qe_sat_project_real_direction.smt2). g3 uses the
// assign/subst family here.
func (c *arithmeticModelProjectCall) realBranch(entry *arithmeticPluginBounds, target ast.ASTHandle) (_ uint32, resultErr error) {
	policy, err := c.plugin.branchPolicy(entry, target)
	if err != nil {
		return 0, err
	}
	direction := ArithmeticUpperBound
	if policy.Lower {
		direction = ArithmeticLowerBound
	}
	non := entry.processor.Bounds(false, direction)
	strict := entry.processor.Bounds(true, direction)
	q := c.plugin.util.q()
	nonValue, strictValue, variableValue := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() {
		resultErr = errors.Join(resultErr, q.Del(&nonValue), q.Del(&strictValue), q.Del(&variableValue))
	}()
	nonIndex, nonFound, err := c.findMinMax(non, policy.Lower, &nonValue)
	if err != nil {
		return 0, err
	}
	strictIndex, strictFound, err := c.findMinMax(strict, policy.Lower, &strictValue)
	if err != nil {
		return 0, err
	}
	if !nonFound && !strictFound {
		return 0, nil
	}
	if strictFound {
		useStrict := !nonFound
		if !useStrict {
			cmp, err := q.Compare(&strictValue, &nonValue)
			if err != nil {
				return 0, err
			}
			useStrict = cmp <= 0
		}
		if useStrict {
			return 1 + 2*uint32(len(non)) + strictIndex, nil
		}
	}
	if err = c.numeral(target, &variableValue); err != nil {
		return 0, err
	}
	branch := 1 + 2*nonIndex
	equal, err := q.Equal(&variableValue, &nonValue)
	if err != nil {
		return 0, err
	}
	if !equal {
		branch++
	}
	return branch, nil
}
