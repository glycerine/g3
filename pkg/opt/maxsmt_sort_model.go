package opt

// Source: src/opt/sortmax.cpp update_assignment/is_true.

import "errors"

func (e *sortMaxEngine) updateResult(result *ArithmeticResult) (int, error) {
	cost, satisfied, assignment, err := e.solver.evaluateSoftDetailed(e.objective, result.Model)
	if err != nil {
		return 0, err
	}
	setSoftObjectiveValue(result, e.index, cost, satisfied, assignment)
	if !satisfied.IsInt() || !satisfied.Num().IsInt64() {
		return 0, errors.New("opt: invalid SortMax objective value")
	}
	count := satisfied.Num().Int64()
	if count < 0 || count > int64(len(e.outputs)) {
		return 0, errors.New("opt: invalid SortMax objective value")
	}
	return int(count), nil
}
