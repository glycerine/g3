package cmd_context

// TODO bigB: temporary bridge between cmd_context's big.Rat optimization
// state and opt's numeral rationals (the AST manager's rational manager).
// It goes away when cmd_context's optimization state holds rationals.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/opt"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *Context) optQ() *numeral.RationalManager { return c.manager.NumeralService().Manager }

// optRat returns a caller-owned rational equal to r.
func (c *Context) optRat(r *big.Rat) (numeral.Rational, error) {
	out := numeral.NewRational()
	return out, c.optQ().SetBigRat(&out, r)
}

// optRats returns caller-owned rationals equal to rs.
func (c *Context) optRats(rs []*big.Rat) ([]numeral.Rational, error) {
	out := make([]numeral.Rational, len(rs))
	for i := range out {
		out[i] = numeral.NewRational()
	}
	for i := range rs {
		if err := c.optQ().SetBigRat(&out[i], rs[i]); err != nil {
			return nil, c.optDelRats(out, err)
		}
	}
	return out, nil
}

func (c *Context) optDelRats(rs []numeral.Rational, err error) error {
	q := c.optQ()
	for i := range rs {
		if e := q.Del(&rs[i]); e != nil && err == nil {
			err = e
		}
	}
	return err
}

// optBig returns r as a big.Rat for cmd_context's state.
func (c *Context) optBig(r *numeral.Rational) (*big.Rat, error) {
	out := new(big.Rat)
	return out, c.optQ().BigRatOf(r, out)
}

// optSetBig sets dst from r, keeping the first error in *err.
func (c *Context) optSetBig(dst *big.Rat, r *numeral.Rational, err *error) {
	if *err != nil {
		return
	}
	*err = c.optQ().BigRatOf(r, dst)
}

// optRestoreBig is d.RestoreValue(penalty) as a big.Rat for simplex.
func (c *Context) optRestoreBig(d *opt.DecomposedObjective, penalty *numeral.Rational) (*big.Rat, error) {
	q := c.optQ()
	value := numeral.NewRational()
	err := d.RestoreValue(q, penalty, &value)
	var out *big.Rat
	if err == nil {
		out, err = c.optBig(&value)
	}
	return out, c.optDelRats([]numeral.Rational{value}, err)
}

// optReplaceValue releases v's rationals and makes it a fresh value with
// valid zero rationals for the caller to fill.
func (c *Context) optReplaceValue(v *opt.ObjectiveValue) error {
	q := c.optQ()
	err := v.ReleaseRationals(q)
	*v = opt.NewObjectiveValues(q, 1)[0]
	return err
}
