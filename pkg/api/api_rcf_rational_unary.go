package api

// Source: src/api/api_rcf.cpp:183-215.

func (c *SMTContext) RCFNeg(value *RCFNumber) (*RCFNumber, error) {
	service, err := c.checkRCF(value)
	if err != nil {
		return nil, err
	}
	result, operationErr := service.Neg(&value.value)
	return c.wrapRCF(service, result, operationErr)
}

func (c *SMTContext) RCFInv(value *RCFNumber) (*RCFNumber, error) {
	service, err := c.checkRCF(value)
	if err != nil {
		return nil, err
	}
	zero, err := service.IsZero(&value.value)
	if err != nil || zero {
		if zero {
			return nil, ErrRCFDivisionByZero
		}
		return nil, err
	}
	one, err := service.One()
	if err != nil {
		return nil, err
	}
	defer service.CleanupValue(&one)
	result, operationErr := service.Div(&one, &value.value)
	return c.wrapRCF(service, result, operationErr)
}

func (c *SMTContext) RCFPower(value *RCFNumber, exponent uint32) (*RCFNumber, error) {
	service, err := c.checkRCF(value)
	if err != nil {
		return nil, err
	}
	result, operationErr := service.Power(&value.value, exponent)
	return c.wrapRCF(service, result, operationErr)
}
