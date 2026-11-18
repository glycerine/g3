package arith

// Source: src/tactic/arith/pb2bv_tactic.cpp bitblast_pbc/mk_pbc/split.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type pbLiteral struct {
	variable ast.ASTHandle
	negative bool
}

// pbMonomial is monomial: rational m_a (a rational of the manager's
// NumeralService().Manager, owned by the polynomial) and its literal.
type pbMonomial struct {
	weight  unumeral.Rational
	literal pbLiteral
}
type pbPolynomial []pbMonomial

func (t *PB2BVTactic) q() *unumeral.RationalManager { return t.manager.NumeralService().Manager }

// pbRelease is the polynomial's destructor: it destroys the weights.
func pbRelease(q *unumeral.RationalManager, p pbPolynomial) (err error) {
	for i := range p {
		err = errors.Join(err, q.Del(&p[i].weight))
	}
	return err
}

// pbMonomialOf is monomial(a, l): the polynomial owns a copy of a.
func pbMonomialOf(q *unumeral.RationalManager, a *unumeral.Rational, l pbLiteral) (pbMonomial, error) {
	m := pbMonomial{weight: unumeral.NewRational(), literal: l}
	if err := q.Copy(&m.weight, a); err != nil {
		return m, errors.Join(err, q.Del(&m.weight))
	}
	return m, nil
}

// pbUnitMonomial is monomial(l), whose m_a is one.
func pbUnitMonomial(q *unumeral.RationalManager, l pbLiteral) (pbMonomial, error) {
	m := pbMonomial{weight: unumeral.NewRational(), literal: l}
	return m, q.SetInt32(&m.weight, 1)
}

type pbBitBinding struct{ declaration, positive, negative ast.ASTHandle }

// This concrete lexical trail keeps the source expr_ref/vector results alive
// until the caller publishes its output. It owns no solver or callbacks.
type pbEncodingScope struct {
	owner *PB2BVTactic
	refs  []ast.ASTRef
}

func (s *pbEncodingScope) close() (err error) {
	for i := len(s.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, s.owner.manager.DelRef(&s.refs[i]))
	}
	s.refs = nil
	return err
}
func (s *pbEncodingScope) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return h, err
	}
	r, e := s.owner.manager.NewRef(h)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	s.refs = append(s.refs, r)
	return h, nil
}
func (s *pbEncodingScope) gate(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.owner.manager
	r, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.refs = append(s.refs, r)
	switch op {
	case ast.ASTOpAnd:
		err = s.owner.boolean.MkAnd(args, r)
	case ast.ASTOpNot:
		err = s.owner.boolean.MkNot(args[0], r)
	case ast.ASTOpIte:
		err = s.owner.boolean.MkIte(args[0], args[1], args[2], r)
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.RefGet(r)
}
func (t *PB2BVTactic) integerLiteral(v ast.ASTHandle, negative bool) (ast.ASTHandle, error) {
	m := t.manager
	decl, err := m.GetDecl(v)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if i, ok := t.bits[decl]; ok {
		if negative {
			return t.entries[i].negative, nil
		}
		return t.entries[i].positive, nil
	}
	b, err := m.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fresh, err := m.MkFreshConstNullable(nil, b)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	not, err := m.MkNot(fresh)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, h := range []ast.ASTHandle{decl, fresh} {
		if err = m.IncRef(h); err != nil {
			return ast.ASTHandle{}, err
		}
		t.heldKeys = append(t.heldKeys, h)
	}
	if err = m.IncRef(not); err != nil {
		return ast.ASTHandle{}, err
	}
	t.heldNots = append(t.heldNots, not)
	t.bits[decl] = len(t.entries)
	t.entries = append(t.entries, pbBitBinding{decl, fresh, not})
	if negative {
		return not, nil
	}
	return fresh, nil
}
func (t *PB2BVTactic) monomialLiteral(l pbLiteral) (ast.ASTHandle, error) {
	return t.integerLiteral(l.variable, l.negative)
}

func (t *PB2BVTactic) allClauses(p pbPolynomial, c *unumeral.Rational, s *pbEncodingScope) (_ ast.ASTHandle, resultErr error) {
	m := t.manager
	q := t.q()
	n := len(p)
	sums := make([]unumeral.Rational, n)
	for i := range sums {
		sums[i] = unumeral.NewRational()
	}
	defer func() {
		for i := range sums {
			resultErr = errors.Join(resultErr, q.Del(&sums[i]))
		}
	}()
	lits := make([]ast.ASTHandle, n)
	for i := n - 1; i >= 0; i-- {
		if err := q.Copy(&sums[i], &p[i].weight); err != nil {
			return ast.ASTHandle{}, err
		}
		if i+1 < n {
			if err := q.Add(&sums[i], &sums[i+1], &sums[i]); err != nil {
				return ast.ASTHandle{}, err
			}
		}
	}
	for i := range p {
		h, err := t.monomialLiteral(p[i].literal)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		lits[i] = h
	}
	var clauses, clause []ast.ASTHandle
	var process func(int, *unumeral.Rational) error
	process = func(index int, bound *unumeral.Rational) (resultErr error) {
		if positive, err := q.IsPos(bound); err != nil || !positive {
			return err
		}
		below := index == n
		if !below {
			c, err := q.Compare(&sums[index], bound)
			if err != nil {
				return err
			}
			below = c < 0
		}
		if below {
			var h ast.ASTHandle
			var err error
			switch len(clause) {
			case 0:
				h, err = m.MkFalse()
			case 1:
				h = clause[0]
			default:
				h, err = s.hold(m.MkOrArgs(clause))
			}
			if err != nil {
				return err
			}
			clauses = append(clauses, h)
			return nil
		}
		if err := t.checkpoint(); err != nil {
			return err
		}
		clause = append(clause, lits[index])
		if err := process(index+1, bound); err != nil {
			return err
		}
		clause = clause[:len(clause)-1]
		next := unumeral.NewRational()
		defer func() { resultErr = errors.Join(resultErr, q.Del(&next)) }()
		if err := q.Sub(bound, &p[index].weight, &next); err != nil {
			return err
		}
		return process(index+1, &next)
	}
	if err := process(0, c); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.gate(ast.ASTOpAnd, clauses...)
}
func (t *PB2BVTactic) bitblast(p pbPolynomial, c *unumeral.Rational, s *pbEncodingScope) (_ ast.ASTHandle, resultErr error) {
	m := t.manager
	q := t.q()
	cardinality := true
	for i := range p {
		one, err := q.IsOne(&p[i].weight)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !one {
			cardinality = false
			break
		}
	}
	n := len(p)
	cVsN, err := rationalCompareInt(q, c, int64(n))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	cIsOne, err := q.IsOne(c)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if cardinality && cVsN > 0 {
		return m.MkFalse()
	}
	if cardinality && (cIsOne || cVsN == 0) {
		args := make([]ast.ASTHandle, n)
		for i := range p {
			h, err := t.monomialLiteral(p[i].literal)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args[i] = h
		}
		if cIsOne {
			return s.hold(m.MkOrArgs(args))
		}
		return s.gate(ast.ASTOpAnd, args...)
	}
	if uint64(n) <= uint64(t.allClausesLimit) {
		return t.allClauses(p, c, s)
	}
	if cardinality {
		k, unsigned, err := rationalUnsigned(q, c)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !unsigned || k == 0 || cVsN >= 0 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		rows := uint32(n) - k + 1
		// Source multiplies unsigned operands before assigning to uint64.
		cost := uint64(k * rows)
		if cost <= uint64(t.cardinalityLimit) {
			truth, err := m.MkTrue()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			falsity, err := m.MkFalse()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			tmp := make([]ast.ASTHandle, rows)
			for i := range tmp {
				tmp[i] = truth
			}
			for i := uint32(0); i < k; i++ {
				for j := uint32(0); j < rows; j++ {
					lit, err := t.monomialLiteral(p[i+j].literal)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					otherwise := falsity
					if j != 0 {
						otherwise = tmp[j-1]
					}
					tmp[j], err = s.gate(ast.ASTOpIte, lit, tmp[j], otherwise)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
			}
			result := tmp[len(tmp)-1]
			if err := t.traceExpression("pb2bv_bv", "bitblast_pbc", 421, "BV Cardinality: ", result, "\n"); err != nil {
				return ast.ASTHandle{}, err
			}
			return result, nil
		}
	}
	if err := t.tracePolynomial("pb2bv_bv_detail", "bitblast_pbc", 427, "encoding:\n", p, c, ""); err != nil {
		return ast.ASTHandle{}, err
	}
	// rational sum(0);
	sum := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&sum)) }()
	for i := range p {
		positive, err := q.IsPos(&p[i].weight)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !positive {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if err = q.Add(&sum, &p[i].weight, &sum); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if cmp, err := q.Compare(&sum, c); err != nil {
		return ast.ASTHandle{}, err
	} else if cmp < 0 {
		return m.MkFalse()
	}
	bits, err := lia2pbBits(q, &sum)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.traceBitWidth(bits, &sum, n); err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, 0, n)
	for i := range p {
		// m.mk_ite(mon_lit2lit(l), mk_numeral(a), mk_numeral(0)): GCC
		// evaluates the arguments right to left.
		zero, err := s.hold(m.BitVectorNumeralUnsigned(0, bits))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		weight, err := s.hold(m.BitVectorNumeralFromRational(&p[i].weight, bits))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		lit, err := t.monomialLiteral(p[i].literal)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		term, err := s.hold(m.MkIte(lit, weight, zero))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args = append(args, term)
	}
	family, err := m.RegisterBitVectorPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lhs, err := s.hold(m.MkFamilyApp(family, 4, nil, args))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rhs, err := s.hold(m.BitVectorNumeralFromRational(c, bits))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return s.hold(m.MkBitVectorULE(rhs, lhs))
}

// split is split(m_p, m_c, m_clause): *p is rewritten in place and the
// clause is returned (owned by the caller, empty when nothing is split).
func (t *PB2BVTactic) split(p *pbPolynomial, c *unumeral.Rational) (clause pbPolynomial, resultErr error) {
	q := t.q()
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, pbRelease(q, clause))
			clause = nil
		}
	}()
	cIsOne, err := q.IsOne(c)
	if err != nil {
		return nil, err
	}
	if len(*p) <= 2 || cIsOne {
		return nil, nil
	}
	equalC := func(i int) (bool, error) { return q.Equal(&(*p)[i].weight, c) }
	for _, i := range []int{0, 1} {
		if eq, err := equalC(i); err != nil || !eq {
			return nil, err
		}
	}
	i := 2
	for i < len(*p) {
		eq, err := equalC(i)
		if err != nil {
			return nil, err
		}
		if !eq {
			break
		}
		i++
	}
	if i == len(*p) {
		return nil, nil
	}
	clause = make(pbPolynomial, 0, i+1)
	for j := 0; j < i; j++ {
		mon, err := pbUnitMonomial(q, (*p)[j].literal)
		if err != nil {
			return clause, errors.Join(err, q.Del(&mon.weight))
		}
		clause = append(clause, mon)
	}
	integer, err := t.manager.MkFamilySort(t.bounds.family, 1, nil)
	if err != nil {
		return clause, err
	}
	fresh, err := t.manager.MkFreshConstNullable(nil, integer)
	if err != nil {
		return clause, err
	}
	if err = t.manager.IncRef(fresh); err != nil {
		return clause, err
	}
	t.temporaryInts = append(t.temporaryInts, fresh)
	mon, err := pbUnitMonomial(q, pbLiteral{fresh, true})
	if err != nil {
		return clause, errors.Join(err, q.Del(&mon.weight))
	}
	clause = append(clause, mon)
	// m_p[0] = monomial(m_c, lit(new_var, false)) after moving [i, sz)
	// down; monomials [0, i) are destroyed.
	first, err := pbMonomialOf(q, c, pbLiteral{fresh, false})
	if err != nil {
		return clause, err
	}
	old := *p
	remaining := make(pbPolynomial, 1, len(old)-i+1)
	remaining[0] = first
	remaining = append(remaining, old[i:]...)
	*p = remaining
	return clause, pbRelease(q, old[:i])
}

// mkPBC is mk_pbc(m_p, m_c, r, enable_split): *p and c are normalized in
// place, as the source's reference parameters are.
func (t *PB2BVTactic) mkPBC(p *pbPolynomial, c *unumeral.Rational, split bool, s *pbEncodingScope) (_ ast.ASTHandle, resultErr error) {
	q := t.q()
	if err := t.tracePolynomial("mk_pbc", "mk_pbc", 507, "", *p, c, ""); err != nil {
		return ast.ASTHandle{}, err
	}
	if positive, err := q.IsPos(c); err != nil {
		return ast.ASTHandle{}, err
	} else if !positive {
		return t.manager.MkTrue()
	}
	if len(*p) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	// numeral a_gcd = (it->m_a > m_c) ? m_c : it->m_a;
	gcd := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&gcd)) }()
	if cmp, err := q.Compare(&(*p)[0].weight, c); err != nil {
		return ast.ASTHandle{}, err
	} else if cmp > 0 {
		err = q.Copy(&gcd, c)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	} else if err = q.Copy(&gcd, &(*p)[0].weight); err != nil {
		return ast.ASTHandle{}, err
	}
	for i := range *p {
		w := &(*p)[i].weight
		if isInt, err := q.IsInt(w); err != nil || !isInt {
			return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
		}
		if cmp, err := q.Compare(w, c); err != nil {
			return ast.ASTHandle{}, err
		} else if cmp > 0 {
			if err = q.Copy(w, c); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		if err := q.GCD(&gcd, w, &gcd); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if positive, err := q.IsPos(&gcd); err != nil || !positive {
		return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
	}
	gcdIsOne, err := q.IsOne(&gcd)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !gcdIsOne {
		for i := range *p {
			if err = q.Div(&(*p)[i].weight, &gcd, &(*p)[i].weight); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		// m_c = ceil(m_c/a_gcd);
		if err = q.Div(c, &gcd, c); err != nil {
			return ast.ASTHandle{}, err
		}
		if err = q.Ceil(c, c); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	gcdText, err := q.String(&gcd)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err := t.tracePolynomial("mk_pbc", "mk_pbc", 528, "GCD = "+gcdText+"; Normalized: ", *p, c, "\n"); err != nil {
		return ast.ASTHandle{}, err
	}
	// The source's a_sum loop adds m_c rather than the coefficients. For any
	// nonempty polynomial this branch succeeds immediately; preserve it.
	if err := t.tracePolynomial("split_bug", "mk_pbc", 542, "", *p, c, ""); err != nil {
		return ast.ASTHandle{}, err
	}
	// polynomial clause;
	var clause pbPolynomial
	defer func() { resultErr = errors.Join(resultErr, pbRelease(q, clause)) }()
	if split {
		clause, err = t.split(p, c)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if err = t.traceSplit(*p, c, clause); err != nil {
		return ast.ASTHandle{}, err
	}
	first, err := t.bitblast(*p, c, s)
	if err != nil || len(clause) == 0 {
		return first, err
	}
	one := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&one)) }()
	if err = q.SetInt32(&one, 1); err != nil {
		return ast.ASTHandle{}, err
	}
	second, err := t.bitblast(clause, &one, s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.tracePair("split_bug", "mk_pbc", 554, first, "\nAND\n", second, "\n"); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.gate(ast.ASTOpAnd, first, second)
}
