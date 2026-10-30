package api

// Source: src/api/api_rcf.cpp:36-75. This unit is the dependency-closed
// rational specialization; transcendental and infinitesimal extensions remain
// gated on their realclosure value kinds.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var ErrRCFNumberOwner = errors.New("api: invalid, foreign, or closed RCF number")
var ErrRCFDivisionByZero = errors.New("api: RCF division by zero")

// RCFNumber is the owning Go form of the source's opaque Z3_rcf_num pointer.
// Close it before closing its SMTContext.
type RCFNumber struct {
	context *smtContextState
	service *unumeral.RationalService
	value   unumeral.OwnedRational
	closed  bool
}

func (c *SMTContext) newRCFNumber(value unumeral.OwnedRational) (*RCFNumber, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	service := manager.NumeralService()
	if service == nil {
		return nil, ErrRCFNumberOwner
	}
	return &RCFNumber{context: c.smtContextState, service: service, value: value}, nil
}

func (c *SMTContext) RCFRational(text string) (result *RCFNumber, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	service := manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, service.CleanupValue(&value))
		}
	}()
	if err = service.SetString(&value, text); err != nil {
		return nil, err
	}
	return c.newRCFNumber(value)
}

func (c *SMTContext) RCFSmallInt(value int32) (*RCFNumber, error) {
	return c.RCFRational(stringInt32(value))
}

func stringInt32(value int32) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	n := int64(value)
	if negative {
		n = -n
	}
	var buffer [11]byte
	i := len(buffer)
	for n != 0 {
		i--
		buffer[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buffer[i] = '-'
	}
	return string(buffer[i:])
}

func (n *RCFNumber) open() error {
	if n == nil || n.context == nil || n.service == nil || n.closed || n.context.closed {
		return ErrRCFNumberOwner
	}
	return nil
}

func (n *RCFNumber) Close() error {
	if err := n.open(); err != nil {
		return err
	}
	err := n.service.CleanupValue(&n.value)
	if err == nil {
		n.closed = true
		n.service = nil
		n.context = nil
	}
	return err
}
