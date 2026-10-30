package api

// Source: src/api/api_rcf.cpp:269-291.

func (c *SMTContext) RCFString(value *RCFNumber) (string, error) {
	service, err := c.checkRCF(value)
	if err != nil {
		return "", err
	}
	return service.String(&value.value)
}

func (c *SMTContext) RCFDecimalString(value *RCFNumber, precision uint32) (string, error) {
	service, err := c.checkRCF(value)
	if err != nil {
		return "", err
	}
	return service.DecimalString(&value.value, precision, false)
}
