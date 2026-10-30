package api

// Source: src/api/api_rcf.cpp:36-44,140-267.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

func (c *SMTContext) checkRCF(values ...*RCFNumber) (*unumeral.RationalService, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	var service *unumeral.RationalService
	for _, value := range values {
		if err := value.open(); err != nil || value.context != c.smtContextState {
			return nil, ErrRCFNumberOwner
		}
		if service == nil {
			service = value.service
		} else if service != value.service {
			return nil, ErrRCFNumberOwner
		}
	}
	if service == nil {
		return nil, ErrRCFNumberOwner
	}
	return service, nil
}

func (c *SMTContext) wrapRCF(service *unumeral.RationalService, value unumeral.OwnedRational, operationErr error) (*RCFNumber, error) {
	if operationErr != nil {
		return nil, operationErr
	}
	if err := c.open(); err != nil {
		_ = service.CleanupValue(&value)
		return nil, err
	}
	return &RCFNumber{context: c.smtContextState, service: service, value: value}, nil
}
