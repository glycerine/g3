package smtcore

// Source: theory_pb::arg_t (src/smt/theory_pb.cpp:84-114) and
// pb_rewriter_util<pb_lit_rewriter_util> (src/ast/rewriter/pb_rewriter_def.h).
//
// numeral is rational: coefficients and k are rationals of the AST manager's
// rational manager. An arg_t owns them; free it with del where the source's
// arg_t (or its ineq) dies.

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type smtPBTerm struct {
	literal SMTLiteral
	coeff   numeral.Rational
}

type smtPBArgs struct {
	terms []smtPBTerm
	bound numeral.Rational // m_k
}

// newSMTPBArgs is an empty arg_t (m_k = 0).
func newSMTPBArgs() smtPBArgs { return smtPBArgs{bound: smtQZero()} }

// del is ~arg_t: it frees k and every coefficient.
func (a *smtPBArgs) del(q *numeral.RationalManager) error {
	err := smtQDel(q, &a.bound)
	for i := range a.terms {
		err = smtErrJoin(err, smtQDel(q, &a.terms[i].coeff))
	}
	a.terms = nil
	return err
}

// reset is arg_t::reset() with m_k.reset(): the vector's rationals are
// destroyed and k becomes 0.
func (a *smtPBArgs) reset(q *numeral.RationalManager) error {
	var err error
	for i := range a.terms {
		err = smtErrJoin(err, smtQDel(q, &a.terms[i].coeff))
	}
	clear(a.terms)
	a.terms = a.terms[:0]
	return smtErrJoin(err, q.Reset(&a.bound))
}

// truncate is args.resize(n) for n <= size: the dropped rationals die.
func (a *smtPBArgs) truncate(q *numeral.RationalManager, n int) error {
	var err error
	for i := n; i < len(a.terms); i++ {
		err = smtErrJoin(err, smtQDel(q, &a.terms[i].coeff))
	}
	clear(a.terms[n:])
	a.terms = a.terms[:n]
	return err
}

// popBack is args.pop_back().
func (a *smtPBArgs) popBack(q *numeral.RationalManager) error {
	return a.truncate(q, len(a.terms)-1)
}

// pushBack is push_back(std::make_pair(l, c)): the vector copies c.
func (a *smtPBArgs) pushBack(q *numeral.RationalManager, l SMTLiteral, c *numeral.Rational) error {
	a.terms = append(a.terms, smtPBTerm{literal: l, coeff: smtQZero()})
	return q.Copy(&a.terms[len(a.terms)-1].coeff, c)
}

// unique is pb_rewriter_util::unique.
func (a *smtPBArgs) unique(q *numeral.RationalManager) error {
	for i := range a.terms {
		if a.terms[i].literal.Sign() {
			a.terms[i].literal = a.terms[i].literal.Negated()
			if err := q.Sub(&a.bound, &a.terms[i].coeff, &a.bound); err != nil {
				return err
			}
			if err := q.Neg(&a.terms[i].coeff); err != nil {
				return err
			}
		}
	}
	// remove constants
	for i := 0; i < len(a.terms); i++ {
		l := a.terms[i].literal
		if l == SMTTrueLiteral {
			if err := q.Sub(&a.bound, &a.terms[i].coeff, &a.bound); err != nil {
				return err
			}
		}
		if l == SMTTrueLiteral || l == SMTFalseLiteral {
			last := len(a.terms) - 1
			a.terms[i], a.terms[last] = a.terms[last], a.terms[i]
			if err := a.popBack(q); err != nil {
				return err
			}
			i--
		}
	}
	// sort and coalesce arguments
	sort.Slice(a.terms, func(i, j int) bool { return a.terms[i].literal.Index() < a.terms[j].literal.Index() })
	if len(a.terms) > 0 {
		i := 0
		for j := 1; j < len(a.terms); j++ {
			if a.terms[i].literal == a.terms[j].literal {
				if err := q.Add(&a.terms[i].coeff, &a.terms[j].coeff, &a.terms[i].coeff); err != nil {
					return err
				}
			} else {
				i++
				a.terms[i].literal = a.terms[j].literal
				if err := q.Copy(&a.terms[i].coeff, &a.terms[j].coeff); err != nil {
					return err
				}
			}
		}
		if err := a.truncate(q, i+1); err != nil {
			return err
		}
	}
	// remove 0s.
	i := 0
	for j := range a.terms {
		if !a.terms[j].coeff.IsZero() {
			if i != j {
				a.terms[i].literal = a.terms[j].literal
				if err := q.Copy(&a.terms[i].coeff, &a.terms[j].coeff); err != nil {
					return err
				}
			}
			i++
		}
	}
	return a.truncate(q, i)
}

// unit sets every coefficient to 1 and k to bound.
func (a *smtPBArgs) unit(q *numeral.RationalManager, bound int64) error {
	for i := range a.terms {
		if err := q.SetInt64(&a.terms[i].coeff, 1); err != nil {
			return err
		}
	}
	return q.SetInt64(&a.bound, bound)
}

// normalize is pb_rewriter_util::normalize.
func (a *smtPBArgs) normalize(q *numeral.RationalManager, eq bool) (_ lbool.LBool, err error) {
	// numeral sum(0), and the cutting-plane temporaries.
	sum, d, den, g, kNew, rem, c := smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&c, &rem, &kNew, &g, &den, &d, &sum} {
			err = smtErrJoin(err, smtQDel(q, r))
		}
	}()
	for i := range a.terms {
		term := &a.terms[i]
		if term.coeff.IsNeg() {
			if err = q.Sub(&a.bound, &term.coeff, &a.bound); err != nil {
				return lbool.Undef, err
			}
			if err = q.Neg(&term.coeff); err != nil {
				return lbool.Undef, err
			}
			term.literal = term.literal.Negated()
		}
		if err = q.Add(&sum, &term.coeff, &sum); err != nil {
			return lbool.Undef, err
		}
	}
	// detect tautologies
	if !eq && a.bound.Sign() <= 0 {
		if err = a.reset(q); err != nil {
			return lbool.Undef, err
		}
		return lbool.True, nil
	}
	if eq && a.bound.IsZero() && len(a.terms) == 0 {
		return lbool.True, nil
	}
	// detect infeasible constraints
	cmp, err := q.Compare(&sum, &a.bound)
	if err != nil {
		return lbool.Undef, err
	}
	if cmp < 0 {
		if err = a.reset(q); err != nil {
			return lbool.Undef, err
		}
		return lbool.False, q.SetInt64(&a.bound, 1)
	}
	if eq && cmp == 0 {
		return lbool.Undef, a.unit(q, int64(len(a.terms)))
	}
	// The source tests only the coefficients; g3 also scales a fractional k
	// (as it always has), so the strict integer complement in negate stays
	// exact. pb_util atoms carry integral k, so this is unreachable from
	// SMT-LIB input.
	allInt := a.bound.IsInt()
	for i := 0; allInt && i < len(a.terms); i++ {
		allInt = a.terms[i].coeff.IsInt()
	}
	if !allInt {
		// normalize to integers: d = lcm of the denominators.
		if err = q.DenominatorRational(&a.bound, &d); err != nil {
			return lbool.Undef, err
		}
		for i := range a.terms {
			if err = q.DenominatorRational(&a.terms[i].coeff, &den); err != nil {
				return lbool.Undef, err
			}
			if err = q.LCM(&d, &den, &d); err != nil {
				return lbool.Undef, err
			}
		}
		if err = q.Mul(&a.bound, &d, &a.bound); err != nil {
			return lbool.Undef, err
		}
		for i := range a.terms {
			if err = q.Mul(&a.terms[i].coeff, &d, &a.terms[i].coeff); err != nil {
				return lbool.Undef, err
			}
		}
	}
	if eq {
		return lbool.Undef, nil
	}
	// Ensure the largest coefficient is not larger than k.
	if err = q.Reset(&sum); err != nil {
		return lbool.Undef, err
	}
	for i := range a.terms {
		if cmp, err = q.Compare(&a.terms[i].coeff, &a.bound); err != nil {
			return lbool.Undef, err
		}
		if cmp > 0 {
			if err = q.Copy(&a.terms[i].coeff, &a.bound); err != nil {
				return lbool.Undef, err
			}
		}
		if err = q.Add(&sum, &a.terms[i].coeff, &sum); err != nil {
			return lbool.Undef, err
		}
	}
	// normalize tight inequalities to unit coefficients.
	if cmp, err = q.Compare(&sum, &a.bound); err != nil {
		return lbool.Undef, err
	}
	if cmp == 0 {
		if err = a.unit(q, int64(len(a.terms))); err != nil {
			return lbool.Undef, err
		}
	}
	// apply cutting plane reduction: g is the gcd of the coefficients < k.
	for i := 0; !g.IsOne() && i < len(a.terms); i++ {
		c := &a.terms[i].coeff
		if cmp, err = q.Compare(c, &a.bound); err != nil {
			return lbool.Undef, err
		}
		if cmp != 0 {
			if g.IsZero() {
				err = q.Copy(&g, c)
			} else {
				err = q.GCD(&g, c, &g)
			}
			if err != nil {
				return lbool.Undef, err
			}
		}
	}
	if g.IsZero() {
		// all coefficients are equal to k.
		if err = a.unit(q, 1); err != nil {
			return lbool.Undef, err
		}
	} else if g.Sign() > 0 && !g.IsOne() {
		// k_new = div(k, g), plus one when k % g != 0 (the ceiling).
		if err = q.IDiv(&a.bound, &g, &kNew); err != nil {
			return lbool.Undef, err
		}
		if err = q.Mod(&a.bound, &g, &rem); err != nil {
			return lbool.Undef, err
		}
		if !rem.IsZero() {
			if err = q.Inc(&kNew); err != nil {
				return lbool.Undef, err
			}
		}
		for i := range a.terms {
			if err = q.Copy(&c, &a.terms[i].coeff); err != nil {
				return lbool.Undef, err
			}
			if cmp, err = q.Compare(&c, &a.bound); err != nil {
				return lbool.Undef, err
			}
			if cmp == 0 {
				err = q.Copy(&c, &kNew)
			} else {
				err = q.IDiv(&c, &g, &c)
			}
			if err == nil {
				err = q.Copy(&a.terms[i].coeff, &c)
			}
			if err != nil {
				return lbool.Undef, err
			}
		}
		if err = q.Copy(&a.bound, &kNew); err != nil {
			return lbool.Undef, err
		}
	}
	// normalize coefficients that fall within a range
	// k/n <= ... < k/(n-1) for some n = 1,2,...
	if !a.bound.IsOne() && len(a.terms) > 0 {
		return lbool.Undef, a.setCardinality(q)
	}
	return lbool.Undef, nil
}

// setCardinality is normalize's last step: when floor(k/max) = ceil(k/min)-1
// and k/max is not integral, all coefficients become 1 and k = n1 + 1.
func (a *smtPBArgs) setCardinality(q *numeral.RationalManager) (err error) {
	min, max, n0, n1, n2 := smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&n2, &n1, &n0, &max, &min} {
			err = smtErrJoin(err, smtQDel(q, r))
		}
	}()
	if err = q.Copy(&min, &a.terms[0].coeff); err != nil {
		return err
	}
	if err = q.Copy(&max, &a.terms[0].coeff); err != nil {
		return err
	}
	for i := 1; i < len(a.terms); i++ {
		c := &a.terms[i].coeff
		cmp, err := q.Compare(c, &min)
		if err != nil {
			return err
		}
		if cmp < 0 {
			if err = q.Copy(&min, c); err != nil {
				return err
			}
		}
		if cmp, err = q.Compare(c, &max); err != nil {
			return err
		}
		if cmp > 0 {
			if err = q.Copy(&max, c); err != nil {
				return err
			}
		}
	}
	if min.Sign() <= 0 {
		return ast.ErrASTDomain
	}
	if err = q.Div(&a.bound, &max, &n0); err != nil {
		return err
	}
	if err = q.Floor(&n0, &n1); err != nil {
		return err
	}
	if err = q.Div(&a.bound, &min, &n2); err != nil {
		return err
	}
	if err = q.Ceil(&n2, &n2); err != nil {
		return err
	}
	if err = q.Dec(&n2); err != nil {
		return err
	}
	eq, err := q.Equal(&n1, &n2)
	if err != nil {
		return err
	}
	if eq && !n0.IsInt() {
		for i := range a.terms {
			if err = q.SetInt64(&a.terms[i].coeff, 1); err != nil {
				return err
			}
		}
		if err = q.Copy(&a.bound, &n1); err != nil {
			return err
		}
		return q.Inc(&a.bound)
	}
	return nil
}

// prune is pb_rewriter_util::prune.
func (a *smtPBArgs) prune(q *numeral.RationalManager, eq bool) (err error) {
	if eq {
		return nil
	}
	nlt := smtQZero()
	defer func() { err = smtErrJoin(err, smtQDel(q, &nlt)) }()
	occ := 0
	for i := 0; i < len(a.terms); i++ {
		cmp, err := q.Compare(&nlt, &a.bound)
		if err != nil {
			return err
		}
		if cmp >= 0 {
			break
		}
		if cmp, err = q.Compare(&a.terms[i].coeff, &a.bound); err != nil {
			return err
		}
		if cmp < 0 {
			if err = q.Add(&nlt, &a.terms[i].coeff, &nlt); err != nil {
				return err
			}
			occ++
		}
	}
	cmp, err := q.Compare(&nlt, &a.bound)
	if err != nil {
		return err
	}
	if occ > 0 && cmp < 0 {
		for i := 0; i < len(a.terms); i++ {
			if cmp, err = q.Compare(&a.terms[i].coeff, &a.bound); err != nil {
				return err
			}
			if cmp < 0 {
				// args[i] = args.back(); args.pop_back(); --i;
				last := len(a.terms) - 1
				a.terms[i].literal = a.terms[last].literal
				if err = q.Copy(&a.terms[i].coeff, &a.terms[last].coeff); err != nil {
					return err
				}
				if err = a.popBack(q); err != nil {
					return err
				}
				i--
			}
		}
		if err = a.unique(q); err != nil {
			return err
		}
		_, err = a.normalize(q, false)
		return err
	}
	return nil
}

// negate is arg_t::negate(): m_k = sum - m_k + 1, then normalize.
func (a *smtPBArgs) negate(q *numeral.RationalManager) (err error) {
	sum := smtQZero()
	defer func() { err = smtErrJoin(err, smtQDel(q, &sum)) }()
	for i := range a.terms {
		a.terms[i].literal = a.terms[i].literal.Negated()
		if err = q.Add(&sum, &a.terms[i].coeff, &sum); err != nil {
			return err
		}
	}
	if err = q.Sub(&sum, &a.bound, &a.bound); err != nil {
		return err
	}
	if err = q.Inc(&a.bound); err != nil {
		return err
	}
	// VERIFY evaluates normalize in every build, then checks its result
	// only in debug builds. Preserve normalization before reporting failure.
	truth, err := a.normalize(q, false)
	if err != nil {
		return err
	}
	if smtcoreSourceDebug && truth != lbool.Undef {
		return ast.ErrASTDomain
	}
	return nil
}

// copy is arg_t's copy constructor (m_k and append).
func (a *smtPBArgs) copy(q *numeral.RationalManager) (smtPBArgs, error) {
	out := newSMTPBArgs()
	if err := q.Copy(&out.bound, &a.bound); err != nil {
		return out, err
	}
	for i := range a.terms {
		if err := out.pushBack(q, a.terms[i].literal, &a.terms[i].coeff); err != nil {
			return out, smtErrJoin(err, out.del(q))
		}
	}
	return out, nil
}
