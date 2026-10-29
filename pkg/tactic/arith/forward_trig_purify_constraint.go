package arith

import "errors"

func (i *forwardTrigInvocation) unitCircle(pair *forwardTrigPair, entry *forwardTrigEntry) (err error) {
	m := i.owner.manager
	t := forwardTrigTemps{manager: m}
	defer func() { err = errors.Join(err, t.close()) }()
	x, e := m.RefGet(pair.sin)
	if e != nil {
		return e
	}
	y, e := m.RefGet(pair.cos)
	if e != nil {
		return e
	}
	xx, e := m.MkArithmeticMul(x, x)
	if e != nil {
		return e
	}
	t.hold(xx)
	yy, e := m.MkArithmeticMul(y, y)
	if e != nil {
		return e
	}
	t.hold(yy)
	sum, e := m.MkArithmeticAdd(xx, yy)
	if e != nil {
		return e
	}
	t.hold(sum)
	one, e := m.ArithmeticRealNumeralText("1")
	if e != nil {
		return e
	}
	eq, e := m.MkEq(one, sum)
	if e != nil {
		return e
	}
	return i.addConstraint(eq, entry)
}
