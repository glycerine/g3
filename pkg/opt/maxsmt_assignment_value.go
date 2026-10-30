package opt

// Source: src/opt/maxsmt.cpp init/update_assignment/get_assignment.

import "math/big"

func setSoftObjectiveValue(result *ArithmeticResult, index int, cost, satisfied big.Rat, assignment []bool) {
	result.Values[index] = ObjectiveValue{
		SoftCost: cost, SoftSatisfied: satisfied,
		SoftAssignment: append([]bool(nil), assignment...), Soft: true,
	}
}
