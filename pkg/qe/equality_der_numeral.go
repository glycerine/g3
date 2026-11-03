package qe

// Source: src/qe/qe_lite.cpp:227-237, arith_util::is_numeral.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// The coefficient belongs to the enclosing call's numbers. This concrete
// reader keeps the native copy invocation alive until its destination and
// detached allocations can be cleaned up together after an interrupted copy.
type equalityDERNumeralRead struct {
	manager    *ast.ASTManager
	receiver   *numeral.OwnedRational
	invocation numeral.RationalServiceInvocation
	bytes      uint64
}

func (r *equalityDERNumeralRead) retire() error {
	if err := r.invocation.SourceReady(); err != nil {
		return err
	}
	if err := r.invocation.Cleanup(false); err != nil {
		return err
	}
	if err := r.invocation.Cleanup(true); err != nil {
		return err
	}
	if err := r.invocation.EndContext(); err != nil {
		return err
	}
	r.invocation = numeral.RationalServiceInvocation{}
	r.receiver = nil
	return nil
}

func (r *equalityDERNumeralRead) close() error {
	if err := r.invocation.Cleanup(false); err != nil {
		return err
	}
	if r.receiver != nil && *r.receiver != (numeral.OwnedRational{}) {
		if err := r.manager.NumeralService().CleanupValue(r.receiver); err != nil {
			return err
		}
	}
	if err := r.invocation.Cleanup(true); err != nil {
		return err
	}
	if err := r.invocation.EndContext(); err != nil {
		return err
	}
	r.invocation = numeral.RationalServiceInvocation{}
	r.receiver = nil
	r.manager.Budget().Release(r.bytes)
	r.bytes = 0
	return nil
}

func (c *equalityDERCall) readNumeral(h ast.ASTHandle, coefficient *numeral.OwnedRational) (bool, error) {
	if c.numeralRead == nil {
		bytes := uint64(reflect.TypeOf(equalityDERNumeralRead{}).Size())
		if err := c.owner.manager.Budget().Reserve(bytes); err != nil {
			return false, err
		}
		c.numeralRead = &equalityDERNumeralRead{manager: c.owner.manager, bytes: bytes}
	}
	r := c.numeralRead
	r.receiver = coefficient
	var integer bool
	matched, err := c.owner.manager.ArithmeticNumeralValueWithInvocation(c.owner.arithmetic, h, coefficient, &integer, &r.invocation)
	if err == nil {
		err = r.retire()
	}
	return matched, err
}
