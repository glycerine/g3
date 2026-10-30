package api

// Source: src/api/api_rcf.cpp:140-181.

func (c *SMTContext) rcfBinary(a, b *RCFNumber, operation uint8) (*RCFNumber, error) {
	service, err := c.checkRCF(a, b)
	if err != nil {
		return nil, err
	}
	switch operation {
	case 0:
		value, operationErr := service.Add(&a.value, &b.value)
		return c.wrapRCF(service, value, operationErr)
	case 1:
		value, operationErr := service.Sub(&a.value, &b.value)
		return c.wrapRCF(service, value, operationErr)
	case 2:
		value, operationErr := service.Mul(&a.value, &b.value)
		return c.wrapRCF(service, value, operationErr)
	case 3:
		zero, err := service.IsZero(&b.value)
		if err != nil {
			return nil, err
		}
		if zero {
			return nil, ErrRCFDivisionByZero
		}
		value, operationErr := service.Div(&a.value, &b.value)
		return c.wrapRCF(service, value, operationErr)
	default:
		return nil, ErrRCFNumberOwner
	}
}

func (c *SMTContext) RCFAdd(a, b *RCFNumber) (*RCFNumber, error) { return c.rcfBinary(a, b, 0) }
func (c *SMTContext) RCFSub(a, b *RCFNumber) (*RCFNumber, error) { return c.rcfBinary(a, b, 1) }
func (c *SMTContext) RCFMul(a, b *RCFNumber) (*RCFNumber, error) { return c.rcfBinary(a, b, 2) }
func (c *SMTContext) RCFDiv(a, b *RCFNumber) (*RCFNumber, error) { return c.rcfBinary(a, b, 3) }
