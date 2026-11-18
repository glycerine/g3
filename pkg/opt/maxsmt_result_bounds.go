package opt

// Source: src/opt/maxsmt.cpp get_lower/get_upper at optimal termination.

func finalizeSoftBounds(result *ArithmeticResult, index int) error {
	value := &result.Values[index]
	if err := result.Q.Copy(&value.SoftLower, &value.SoftCost); err != nil {
		return err
	}
	if err := result.Q.Copy(&value.SoftUpper, &value.SoftCost); err != nil {
		return err
	}
	value.SoftBoundsValid = true
	return nil
}
