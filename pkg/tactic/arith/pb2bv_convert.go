package arith

// Source: src/tactic/arith/pb2bv_tactic.cpp adjust/convert/is_eq_vector.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type pbConstraintKind uint8

const (
	pbEqual pbConstraintKind = iota
	pbGreaterEqual
	pbLessEqual
)

type pbTacticConfiguration struct {
	rewriter.DefaultConfiguration
	owner *PB2BVTactic
}

func (c *pbTacticConfiguration) MaxStepsExceeded(uint32) (bool, error) {
	return false, c.owner.checkpoint()
}
func (c *pbTacticConfiguration) GetSubst(term ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	t := c.owner
	yes, err := t.constraintCore(term)
	if err != nil || !yes {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if err = t.convert(term, t.saved, true, false); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	value, err := t.manager.RefGet(t.saved)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if err = t.tracePair("pb2bv_convert", "get_subst", 223, term, "\n-->\n", value, "\n"); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return value, ast.ASTHandle{}, true, nil
}
func pbAdjust(q *unumeral.RationalManager, positive *bool, kind *pbConstraintKind, c *unumeral.Rational) error {
	if !*positive {
		if *kind == pbLessEqual {
			*positive = true
			*kind = pbGreaterEqual
			return q.Inc(c)
		} else if *kind == pbGreaterEqual {
			*positive = true
			*kind = pbLessEqual
			return q.Dec(c)
		}
	}
	return nil
}

// numeral is m_arith_util.is_numeral(h, r, is_int), writing r (a rational of
// the manager's NumeralService().Manager).
func (t *PB2BVTactic) numeral(h ast.ASTHandle, r *unumeral.Rational) (integer, found bool, err error) {
	integer, found, err = t.manager.ArithmeticNumeralRational(h, r)
	return integer, found && err == nil, err
}

// isNumeral is m_arith_util.is_numeral(h).
func (t *PB2BVTactic) isNumeral(h ast.ASTHandle) (bool, error) {
	return t.manager.IsArithmeticNumeral(h)
}
func (t *PB2BVTactic) addBoundsDependencies(h ast.ASTHandle) error {
	for _, lower := range []bool{true, false} {
		var dep ast.ASTDependency
		var err error
		if lower {
			dep, err = t.bounds.LowerDependency(h)
		} else {
			dep, err = t.bounds.UpperDependency(h)
		}
		if err != nil {
			return err
		}
		joined, err := t.manager.DependencyJoin(t.used, dep)
		if err != nil {
			return err
		}
		if err = t.manager.DependencyInc(joined); err != nil {
			return err
		}
		old := t.used
		t.used = joined
		if err = t.manager.DependencyDec(old); err != nil {
			return err
		}
	}
	return nil
}
func (t *PB2BVTactic) convert(term ast.ASTHandle, out ast.ASTRef, positive, root bool) (err error) {
	scope := &pbEncodingScope{owner: t}
	defer func() { err = errors.Join(err, scope.close()) }()
	value, err := t.convertCore(term, positive, root, scope)
	if err != nil {
		return err
	}
	return t.manager.SetRef(out, value)
}
func (t *PB2BVTactic) convertCore(term ast.ASTHandle, positive, root bool, s *pbEncodingScope) (_ ast.ASTHandle, resultErr error) {
	m := t.manager
	q := t.q()
	// numeral c; polynomial m_p, m_p2; numeral c2;
	c, c2 := unumeral.NewRational(), unumeral.NewRational()
	var p, p2 pbPolynomial
	defer func() {
		resultErr = errors.Join(resultErr, pbRelease(q, p2), pbRelease(q, p), q.Del(&c2), q.Del(&c))
	}()
	family, op, args, _, err := t.bounds.app(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(args) != 2 {
		return ast.ASTHandle{}, t.nonPB(term)
	}
	lhs, rhs := args[0], args[1]
	kind := pbEqual
	if family == m.BasicFamilyID() && op == ast.ASTOpEq {
		lc, e := t.bounds.constant(lhs)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rc, e := t.bounds.constant(rhs)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if lc && rc {
			if err = t.addBoundsDependencies(lhs); err != nil {
				return ast.ASTHandle{}, err
			}
			if err = t.addBoundsDependencies(rhs); err != nil {
				return ast.ASTHandle{}, err
			}
			// m.mk_iff(mon_lit2lit(lhs), mon_lit2lit(rhs)): GCC evaluates the
			// right operand first.
			second, e := t.integerLiteral(rhs, !positive)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			first, e := t.integerLiteral(lhs, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return s.hold(m.MkIff(first, second))
		}
	} else if family == t.bounds.family && op == 2 {
		kind = pbLessEqual
	} else if family == t.bounds.family && op == 3 {
		kind = pbGreaterEqual
	} else {
		return ast.ASTHandle{}, t.nonPB(term)
	}
	// z3 4.7.1 handled a numeral on the left in its own branch, which
	// encoded (= c x) as the unit x for every c outside {0,1} and
	// (<= c x)/(>= c x) without range checks (AUDIT.md tactic_arith_03-F1;
	// fork fix in pb2bv_tactic.cpp convert). Mirror the atom so the
	// numeral is on the right and use the checked rhs-numeral code.
	number, err := t.isNumeral(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if number {
		constant, e := t.bounds.constant(rhs)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !constant {
			return ast.ASTHandle{}, t.nonPB(term)
		}
		lhs, rhs = rhs, lhs
		if kind == pbLessEqual {
			kind = pbGreaterEqual
		} else if kind == pbGreaterEqual {
			kind = pbLessEqual
		}
	}
	integer, number, err := t.numeral(rhs, &c)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !number || !integer {
		return ast.ASTHandle{}, t.nonPB(term)
	}
	if err = pbAdjust(q, &positive, &kind, &c); err != nil {
		return ast.ASTHandle{}, err
	}
	constant, err := t.bounds.constant(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if constant {
		if err = t.addBoundsDependencies(lhs); err != nil {
			return ast.ASTHandle{}, err
		}
		cSign, err := q.Sign(&c)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		cVsOne, err := rationalCompareInt(q, &c, 1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if kind == pbEqual {
			if err = t.traceNumeral("pb2bv_bug", "convert", 673, "c: ", &c, "\n"); err != nil {
				return ast.ASTHandle{}, err
			}
			if cSign != 0 && cVsOne != 0 {
				if positive {
					return m.MkFalse()
				}
				return m.MkTrue()
			}
			sign := cSign == 0
			if !positive {
				sign = !sign
			}
			return t.integerLiteral(lhs, sign)
		}
		if kind == pbLessEqual {
			if cVsOne >= 0 {
				if err = t.traceDiscard(term, 691); err != nil {
					return ast.ASTHandle{}, err
				}
				return m.MkTrue()
			}
			if cSign < 0 {
				return m.MkFalse()
			}
		} else {
			if cSign <= 0 {
				if err = t.traceDiscard(term, 704); err != nil {
					return ast.ASTHandle{}, err
				}
				return m.MkTrue()
			}
			if cVsOne > 0 {
				return m.MkFalse()
			}
		}
		if !positive || !((cSign == 0 && kind == pbLessEqual) || (cVsOne == 0 && kind == pbGreaterEqual)) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return t.integerLiteral(lhs, kind == pbLessEqual)
	}
	family, op, terms, app, err := t.bounds.app(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !app || family != t.bounds.family || op != 6 {
		return ast.ASTHandle{}, t.nonPB(term)
	}
	// Validate every monomial before collecting any bound dependencies;
	// C++ throw_non_pb(t) names the atom, not the monomial.
	for _, mon := range terms {
		constant, e := t.bounds.constant(mon)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if constant {
			continue
		}
		f, k, a, ok, e := t.bounds.app(mon)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !ok || f != t.bounds.family || k != 9 || len(a) != 2 {
			return ast.ASTHandle{}, t.nonPB(term)
		}
		num, e := t.isNumeral(a[0])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		constant, e = t.bounds.constant(a[1])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !num || !constant {
			return ast.ASTHandle{}, t.nonPB(term)
		}
	}
	p = make(pbPolynomial, 0, len(terms))
	for _, mon := range terms {
		constant, e := t.bounds.constant(mon)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		// rational a(1);
		monomial, e := pbUnitMonomial(q, pbLiteral{mon, false})
		p = append(p, monomial)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		last := &p[len(p)-1]
		if !constant {
			_, _, a, _, e := t.bounds.app(mon)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if _, _, e = t.numeral(a[0], &last.weight); e != nil {
				return ast.ASTHandle{}, e
			}
			last.literal.variable = a[1]
			negative, e := q.IsNeg(&last.weight)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if negative {
				if e = q.Neg(&last.weight); e == nil {
					e = q.Add(&c, &last.weight, &c)
				}
				if e != nil {
					return ast.ASTHandle{}, e
				}
				last.literal.negative = true
			}
		}
		if err = t.addBoundsDependencies(last.literal.variable); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	var cmpErr error
	sort.SliceStable(p, func(i, j int) bool {
		c, e := q.Compare(&p[i].weight, &p[j].weight)
		cmpErr = errors.Join(cmpErr, e)
		return c > 0
	})
	if cmpErr != nil {
		return ast.ASTHandle{}, cmpErr
	}
	if kind == pbGreaterEqual {
		return t.mkPBC(&p, &c, root, s)
	}
	if kind == pbLessEqual {
		if err = q.Neg(&c); err != nil {
			return ast.ASTHandle{}, err
		}
		for i := range p {
			if err = q.Add(&c, &p[i].weight, &c); err != nil {
				return ast.ASTHandle{}, err
			}
			p[i].literal.negative = !p[i].literal.negative
		}
		return t.mkPBC(&p, &c, root, s)
	}
	eqVector, err := t.equalVector(p, &c)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if eqVector {
		if err = t.traceText("is_eq_vector", "convert", 796, "found eq vector\n"); err != nil {
			return ast.ASTHandle{}, err
		}
		eqs := make([]ast.ASTHandle, 0, len(p)/2)
		for i := 0; i < len(p); i += 2 {
			// m.mk_eq(int2lit(x_i), int2lit(y_i)): GCC evaluates y_i first.
			b, e := t.integerLiteral(p[i+1].literal.variable, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			a, e := t.integerLiteral(p[i].literal.variable, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			eq, e := s.hold(m.MkEq(a, b))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			eqs = append(eqs, eq)
		}
		value, e := s.gate(ast.ASTOpAnd, eqs...)
		if e != nil || positive {
			return value, e
		}
		return s.gate(ast.ASTOpNot, value)
	}
	p2 = make(pbPolynomial, 0, len(p))
	if err = errors.Join(q.Copy(&c2, &c), q.Neg(&c2)); err != nil {
		return ast.ASTHandle{}, err
	}
	for i := range p {
		literal := p[i].literal
		literal.negative = !literal.negative
		monomial, e := pbMonomialOf(q, &p[i].weight, literal)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		p2 = append(p2, monomial)
		if err = q.Add(&c2, &p[i].weight, &c2); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	first, err := t.mkPBC(&p, &c, false, s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	second, err := t.mkPBC(&p2, &c2, false, s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.traceEquality(term, p, &c, p2, &c2, first, second); err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := s.gate(ast.ASTOpAnd, first, second)
	if err != nil || positive {
		return value, err
	}
	return s.gate(ast.ASTOpNot, value)
}
func (t *PB2BVTactic) equalVector(p pbPolynomial, c *unumeral.Rational) (_ bool, resultErr error) {
	if err := t.tracePolynomial("is_eq_vector", "is_eq_vector", 586, "", p, c, ""); err != nil {
		return false, err
	}
	if len(p)%2 != 0 {
		return false, nil
	}
	n := len(p) / 2
	// rational two(2), power = 2^(n-1-i), bound = 2^n - 1
	q := t.q()
	two, power := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&two), q.Del(&power)) }()
	if err := errors.Join(q.SetInt32(&two, 2), q.Power(&two, uint32(n), &power), q.Dec(&power)); err != nil {
		return false, err
	}
	if eq, err := q.Equal(c, &power); err != nil || !eq {
		return false, err
	}
	for i := 0; i < n; i++ {
		a, b := &p[2*i], &p[2*i+1]
		if a.literal.negative == b.literal.negative {
			return false, nil
		}
		if eq, err := q.Equal(&a.weight, &b.weight); err != nil || !eq {
			return false, err
		}
		if err := q.Power(&two, uint32(n-i-1), &power); err != nil {
			return false, err
		}
		if eq, err := q.Equal(&a.weight, &power); err != nil || !eq {
			return false, err
		}
	}
	return true, nil
}
