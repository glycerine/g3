package cmd_context

// Source: src/opt/opt_context.h objective (rational m_adjust_value offsets,
// maxsmt weights and values); src/util/rational.h value semantics.
//
// The optimization state's rationals belong to the AST manager's rational
// manager (NumeralService().Manager), as opt::context's rationals belong to
// the global mpq manager. Every optimizationObjective owns its rationals and
// its soft terms' weights; copies are deep (cloneObjectives).

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *Context) optQ() *numeral.RationalManager { return c.manager.NumeralService().Manager }

// newOptimizationObjective is an objective with zero rationals.
func newOptimizationObjective() optimizationObjective {
	return optimizationObjective{
		softValue: numeral.NewRational(), softUpper: numeral.NewRational(),
		softOffset: numeral.NewRational(), boundLower: numeral.NewRational(),
		boundUpper: numeral.NewRational(), verifyOffset: numeral.NewRational(),
	}
}

func (o *optimizationObjective) rationals() []*numeral.Rational {
	return []*numeral.Rational{&o.softValue, &o.softUpper, &o.softOffset, &o.boundLower, &o.boundUpper, &o.verifyOffset}
}

// releaseSoftTerms destroys the weights of terms.
func (c *Context) releaseSoftTerms(terms []optimizationSoftTerm) (err error) {
	q := c.optQ()
	for i := range terms {
		err = errors.Join(err, q.Del(&terms[i].weight))
	}
	return err
}

// releaseObjectiveRationals destroys an objective's rationals and its soft
// terms' weights (not its AST references).
func (c *Context) releaseObjectiveRationals(o *optimizationObjective) (err error) {
	q := c.optQ()
	for _, r := range o.rationals() {
		err = errors.Join(err, q.Del(r))
	}
	return errors.Join(err, c.releaseSoftTerms(o.softTerms))
}

// releaseObjectives releases the rationals of every objective of objectives.
func (c *Context) releaseObjectives(objectives []optimizationObjective) (err error) {
	for i := range objectives {
		err = errors.Join(err, c.releaseObjectiveRationals(&objectives[i]))
	}
	return err
}

// cloneObjectives copies objectives with their own rationals and soft-term
// slices; AST references stay with the source (the clone holds none).
func (c *Context) cloneObjectives(source []optimizationObjective) ([]optimizationObjective, error) {
	q := c.optQ()
	out := make([]optimizationObjective, len(source))
	for i := range source {
		out[i] = source[i]
		out[i].softTerms = nil
		var err error
		src := source[i].rationals()
		for j, r := range out[i].rationals() {
			*r = numeral.NewRational()
			err = errors.Join(err, q.Copy(r, src[j]))
		}
		if len(source[i].softTerms) != 0 {
			out[i].softTerms = make([]optimizationSoftTerm, len(source[i].softTerms))
			for j := range source[i].softTerms {
				out[i].softTerms[j] = source[i].softTerms[j]
				out[i].softTerms[j].weight = numeral.NewRational()
				err = errors.Join(err, q.Copy(&out[i].softTerms[j].weight, &source[i].softTerms[j].weight))
			}
		}
		if err != nil {
			return nil, errors.Join(err, c.releaseObjectives(out[:i+1]))
		}
	}
	return out, nil
}

// optClone returns a caller-owned copy of r.
func (c *Context) optClone(r *numeral.Rational) (numeral.Rational, error) {
	out := numeral.NewRational()
	return out, c.optQ().Copy(&out, r)
}

// optDelRats releases rs, joining any failure onto err.
func (c *Context) optDelRats(rs []numeral.Rational, err error) error {
	q := c.optQ()
	for i := range rs {
		err = errors.Join(err, q.Del(&rs[i]))
	}
	return err
}

// optBitVectorMaximum sets out to 2^width - 1, a bit-vector objective's
// largest value.
func (c *Context) optBitVectorMaximum(width uint32, out *numeral.Rational) error {
	q := c.optQ()
	return errors.Join(q.SetInt32(out, 2), q.Power(out, width, out), q.Dec(out))
}

// optReplaceValue releases v's rationals and makes it a fresh value with
// valid zero rationals for the caller to fill.
func (c *Context) optReplaceValue(v *opt.ObjectiveValue) error {
	q := c.optQ()
	err := v.ReleaseRationals(q)
	*v = opt.NewObjectiveValues(q, 1)[0]
	return err
}

// optInfinitesimal is the rational r as simplex's current value type.
// TODO bigA: simplex.Infinitesimal still holds big.Rat; this boundary goes
// when it holds rationals.
func (c *Context) optInfinitesimal(r *numeral.Rational) (simplex.Infinitesimal, error) {
	var value simplex.Infinitesimal
	return value, c.optQ().BigRatOf(r, &value.Main)
}

// optRestoreInfinitesimal is d.RestoreValue(penalty) as an infinitesimal.
func (c *Context) optRestoreInfinitesimal(d *opt.DecomposedObjective, penalty *numeral.Rational) (simplex.Infinitesimal, error) {
	q := c.optQ()
	value := numeral.NewRational()
	err := d.RestoreValue(q, penalty, &value)
	var out simplex.Infinitesimal
	if err == nil {
		out, err = c.optInfinitesimal(&value)
	}
	return out, errors.Join(err, q.Del(&value))
}
