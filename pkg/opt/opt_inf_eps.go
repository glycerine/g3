package opt

// Source: src/util/inf_eps_rational.h. opt's inf_eps values (optsmt's
// m_lower/m_upper, the objective values and display bounds) are
// simplex.Infinitesimal values of the AST manager's rational manager; their
// holder frees them.

import (
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// infOf is inf_eps(r): a fresh value.
func (o *ratOps) infOf(r *numeral.Rational) simplex.Infinitesimal {
	if !o.ok() {
		return simplex.ZeroInfinitesimal()
	}
	v, err := simplex.RationalInfinitesimal(o.q, r)
	o.fail(err)
	return v
}

// infinite is inf_eps(rational(sign), inf_rational(0)): +oo or -oo.
func (o *ratOps) infinite(sign int64) simplex.Infinitesimal {
	if !o.ok() {
		return simplex.ZeroInfinitesimal()
	}
	v, err := simplex.InfiniteInfinitesimal(o.q, sign)
	o.fail(err)
	return v
}

// infClone is the inf_eps copy constructor.
func (o *ratOps) infClone(v *simplex.Infinitesimal) simplex.Infinitesimal {
	if !o.ok() {
		return simplex.ZeroInfinitesimal()
	}
	r, err := v.Clone(o.q)
	o.fail(err)
	return r
}

func (o *ratOps) infSet(dst, src *simplex.Infinitesimal) {
	if o.ok() {
		o.fail(dst.Set(o.q, src))
	}
}

func (o *ratOps) infNeg(v *simplex.Infinitesimal) {
	if o.ok() {
		o.fail(v.Neg(o.q))
	}
}

func (o *ratOps) infCmp(a, b *simplex.Infinitesimal) int {
	if !o.ok() {
		return 0
	}
	c, err := a.Cmp(o.q, b)
	o.fail(err)
	return c
}

// infDel is the inf_eps destructor; v is left a valid 0.
func (o *ratOps) infDel(v *simplex.Infinitesimal) {
	if o.q != nil {
		o.fail(v.Del(o.q))
	} else {
		*v = simplex.ZeroInfinitesimal()
	}
}

func (o *ratOps) infDelAll(vs []simplex.Infinitesimal) {
	for i := len(vs) - 1; i >= 0; i-- {
		o.infDel(&vs[i])
	}
}

// isFinite is m_infty.is_zero().
func (o *ratOps) isFinite(v *simplex.Infinitesimal) bool { return o.sign(&v.Infinity) == 0 }
