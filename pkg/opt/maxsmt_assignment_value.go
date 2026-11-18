package opt

// Source: src/opt/maxsmt.cpp init/update_assignment/get_assignment.

import "github.com/glycerine/g3/pkg/util/numeral"

// setSoftObjectiveValue copies cost and satisfied (borrowed) into the
// result's rationals.
func setSoftObjectiveValue(result *ArithmeticResult, index int, cost, satisfied *numeral.Rational, assignment []bool) error {
	value := &result.Values[index]
	if err := result.Q.Copy(&value.SoftCost, cost); err != nil {
		return err
	}
	if err := result.Q.Copy(&value.SoftSatisfied, satisfied); err != nil {
		return err
	}
	value.SoftAssignment = append([]bool(nil), assignment...)
	value.Soft = true
	return nil
}
