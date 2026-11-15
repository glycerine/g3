package qe

import "github.com/glycerine/g3/pkg/ast"

// realBranch ports arith_plugin::project_real. z3 4.7.1 bug kept out of g3:
// project_real picks the bound family from raw counts
// (le + lt < ge + gt), but assign and subst read the branch in the family
// get_bound_sizes picks, which doubles the non-strict counts of a real, so
// the two disagree and qe-sat with projection_mode false never terminates
// (qe_arith_plugin.cpp:1920-1955; AUDIT.md qe_01-F2;
// test_vectors/z3_bug_qe_sat_project_real_direction.smt2). g3 uses the
// assign/subst family here.
func (c *arithmeticModelProjectCall) realBranch(entry *arithmeticPluginBounds, target ast.ASTHandle) (uint32, error) {
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
	nonValue, nonIndex, nonFound, err := c.findMinMax(non, policy.Lower)
	if err != nil {
		return 0, err
	}
	strictValue, strictIndex, strictFound, err := c.findMinMax(strict, policy.Lower)
	if err != nil {
		return 0, err
	}
	if !nonFound && !strictFound {
		return 0, nil
	}
	if strictFound && (!nonFound || strictValue.Cmp(nonValue) <= 0) {
		return 1 + 2*uint32(len(non)) + strictIndex, nil
	}
	variableValue, err := c.numeral(target)
	if err != nil {
		return 0, err
	}
	branch := 1 + 2*nonIndex
	if variableValue.Cmp(nonValue) != 0 {
		branch++
	}
	return branch, nil
}
