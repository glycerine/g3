package smtcore

// Source: src/smt/theory_arith.h:215-421 and
// src/smt/theory_arith_aux.h:330-420,732-800.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtArithmeticValue represents infinity + rational + epsilon in
// lexicographic order. Zero infinity covers mi_ext; zero epsilon covers i_ext.
// Its operations are smt_theory_arith_numerals.go.
type smtArithmeticValue struct {
	infinity, rational, infinitesimal numeral.Rational
}

type smtArithmeticBoundKind uint8

const (
	smtArithmeticLowerBound smtArithmeticBoundKind = iota
	smtArithmeticUpperBound
)

type smtArithmeticAtomKind uint8

const (
	smtArithmeticLowerAtom smtArithmeticAtomKind = iota
	smtArithmeticUpperAtom
)

type smtArithmeticAntecedents struct {
	literals                                  []SMTLiteral
	equalities                                []SMTEnodePair
	literalCoefficients, equalityCoefficients []numeral.Rational
}

// reset is antecedents_t::reset: the coefficient vectors free their
// numerals.
func (a *smtArithmeticAntecedents) reset(q *numeral.RationalManager) error {
	a.literals = a.literals[:0]
	a.equalities = a.equalities[:0]
	var err, e error
	a.literalCoefficients, err = smtQDelAll(q, a.literalCoefficients)
	a.equalityCoefficients, e = smtQDelAll(q, a.equalityCoefficients)
	if err == nil {
		err = e
	}
	return err
}

// smtQPush appends a copy of coefficient.
func smtQPush(q *numeral.RationalManager, rs []numeral.Rational, coefficient *numeral.Rational) ([]numeral.Rational, error) {
	rs = append(rs, smtQZero())
	return rs, q.Copy(&rs[len(rs)-1], coefficient)
}

func (a *smtArithmeticAntecedents) pushLiteral(q *numeral.RationalManager, literal SMTLiteral, coefficient *numeral.Rational, proofs bool) (err error) {
	if coefficient == nil {
		return ast.ErrASTDomain
	}
	a.literals = append(a.literals, literal)
	if proofs {
		a.literalCoefficients, err = smtQPush(q, a.literalCoefficients, coefficient)
	}
	return err
}
func (a *smtArithmeticAntecedents) pushEquality(q *numeral.RationalManager, pair SMTEnodePair, coefficient *numeral.Rational, proofs bool) (err error) {
	if coefficient == nil || pair.First == nil || pair.Second == nil {
		return ast.ErrASTDomain
	}
	a.equalities = append(a.equalities, pair)
	if proofs {
		a.equalityCoefficients, err = smtQPush(q, a.equalityCoefficients, coefficient)
	}
	return err
}
func (a *smtArithmeticAntecedents) numParameters() int {
	if len(a.literalCoefficients) == 0 && len(a.equalityCoefficients) == 0 {
		return 0
	}
	return 1 + len(a.literalCoefficients) + len(a.equalityCoefficients)
}

type smtArithmeticBoundJustification interface {
	pushJustification(*numeral.RationalManager, *smtArithmeticAntecedents, *numeral.Rational, bool) error
}

type smtArithmeticEqualityBound struct {
	smtArithmeticBound
	left, right *SMTEnode
}

func newSMTArithmeticEqualityBound(q *numeral.RationalManager, variable SMTTheoryVar, value *smtArithmeticValue, kind smtArithmeticBoundKind, left, right *SMTEnode) (*smtArithmeticEqualityBound, error) {
	if variable < 0 || value == nil || kind > smtArithmeticUpperBound || left == nil || right == nil || left.Root() != right.Root() {
		return nil, ast.ErrASTDomain
	}
	result := &smtArithmeticEqualityBound{left: left, right: right}
	result.variable, result.kind = variable, kind
	result.value.init()
	return result, result.value.set(q, value)
}
func (b *smtArithmeticEqualityBound) pushJustification(q *numeral.RationalManager, out *smtArithmeticAntecedents, coefficient *numeral.Rational, proofs bool) error {
	if b == nil || out == nil || b.left == nil || b.right == nil || b.left.Root() != b.right.Root() {
		return ast.ErrASTDomain
	}
	return out.pushEquality(q, SMTEnodePair{First: b.left, Second: b.right}, coefficient, proofs)
}

type smtArithmeticAtom struct {
	smtArithmeticBound
	boolean   SMTBoolVar
	threshold smtArithmeticValue
	atomKind  smtArithmeticAtomKind
	truth     bool
	// smallK caches threshold (which never changes after construction) as
	// machine words when its infinite part is 0 and its rational and
	// infinitesimal parts are inline integers. mk_bound_axioms compares a
	// new atom with every atom of its variable; mpq compares such values
	// inline, without a manager call.
	smallK         bool
	kRational, kEp int64
}

// thresholdCmp is a.threshold.cmp(&b.threshold).
func (a *smtArithmeticAtom) thresholdCmp(q *numeral.RationalManager, b *smtArithmeticAtom) (int, error) {
	if a.smallK && b.smallK {
		switch {
		case a.kRational < b.kRational:
			return -1, nil
		case a.kRational > b.kRational:
			return 1, nil
		case a.kEp < b.kEp:
			return -1, nil
		case a.kEp > b.kEp:
			return 1, nil
		}
		return 0, nil
	}
	return a.threshold.cmp(q, &b.threshold)
}

func (a *smtArithmeticAtom) cacheSmallThreshold() {
	k := &a.threshold
	r, ok1 := k.rational.SmallInt32()
	e, ok2 := k.infinitesimal.SmallInt32()
	if !k.infinity.IsZero() || !ok1 || !ok2 {
		return
	}
	a.smallK, a.kRational, a.kEp = true, int64(r), int64(e)
}

func newSMTArithmeticAtom(q *numeral.RationalManager, boolean SMTBoolVar, variable SMTTheoryVar, threshold *smtArithmeticValue, kind smtArithmeticAtomKind) (*smtArithmeticAtom, error) {
	if boolean < SMTFirstBoolVar || variable < 0 || threshold == nil || kind > smtArithmeticUpperAtom {
		return nil, ast.ErrASTDomain
	}
	result := &smtArithmeticAtom{boolean: boolean, atomKind: kind}
	result.variable, result.atom = variable, true
	result.value.init()
	result.threshold.init()
	if err := result.threshold.set(q, threshold); err != nil {
		return result, err
	}
	result.cacheSmallThreshold()
	return result, nil
}

// del frees the atom's numerals (its destructor).
func (a *smtArithmeticAtom) del(q *numeral.RationalManager) error {
	err := a.value.del(q)
	if e := a.threshold.del(q); e != nil && err == nil {
		err = e
	}
	return err
}
func (a *smtArithmeticAtom) assign(q *numeral.RationalManager, truth bool, epsilon *smtArithmeticValue) error {
	if a == nil || epsilon == nil {
		return ast.ErrASTDomain
	}
	a.truth = truth
	if err := a.value.set(q, &a.threshold); err != nil {
		return err
	}
	if truth {
		a.kind = smtArithmeticBoundKind(a.atomKind)
		return nil
	}
	if a.atomKind == smtArithmeticLowerAtom {
		a.kind = smtArithmeticUpperBound
		return a.value.sub(q, epsilon)
	}
	a.kind = smtArithmeticLowerBound
	return a.value.add(q, epsilon)
}
func (a *smtArithmeticAtom) pushJustification(q *numeral.RationalManager, out *smtArithmeticAntecedents, coefficient *numeral.Rational, proofs bool) error {
	if a == nil || out == nil {
		return ast.ErrASTDomain
	}
	return out.pushLiteral(q, NewSMTLiteral(a.boolean, !a.truth), coefficient, proofs)
}

type smtArithmeticDerivedBound struct {
	smtArithmeticBound
	literals   []SMTLiteral
	equalities []SMTEnodePair
}

func (b *smtArithmeticDerivedBound) pushLiteral(_ *numeral.RationalManager, literal SMTLiteral, _ *numeral.Rational) error {
	b.literals = append(b.literals, literal)
	return nil
}
func (b *smtArithmeticDerivedBound) pushEquality(_ *numeral.RationalManager, pair SMTEnodePair, _ *numeral.Rational) error {
	if pair.First == nil || pair.Second == nil {
		return ast.ErrASTDomain
	}
	b.equalities = append(b.equalities, pair)
	return nil
}

func (b *smtArithmeticDerivedBound) pushJustification(q *numeral.RationalManager, out *smtArithmeticAntecedents, coefficient *numeral.Rational, proofs bool) error {
	if b == nil || out == nil || coefficient == nil {
		return ast.ErrASTDomain
	}
	for _, literal := range b.literals {
		if err := out.pushLiteral(q, literal, coefficient, proofs); err != nil {
			return err
		}
	}
	for _, equality := range b.equalities {
		if err := out.pushEquality(q, equality, coefficient, proofs); err != nil {
			return err
		}
	}
	return nil
}

type smtArithmeticJustifiedDerivedBound struct {
	smtArithmeticDerivedBound
	literalCoefficients, equalityCoefficients []numeral.Rational
}

// del frees the bound's numerals (its destructor).
func (b *smtArithmeticJustifiedDerivedBound) del(q *numeral.RationalManager) error {
	err := b.value.del(q)
	var e error
	if b.literalCoefficients, e = smtQDelAll(q, b.literalCoefficients); e != nil && err == nil {
		err = e
	}
	if b.equalityCoefficients, e = smtQDelAll(q, b.equalityCoefficients); e != nil && err == nil {
		err = e
	}
	return err
}

func (b *smtArithmeticJustifiedDerivedBound) pushLiteral(q *numeral.RationalManager, literal SMTLiteral, coefficient *numeral.Rational) (err error) {
	if coefficient == nil {
		return ast.ErrASTDomain
	}
	for index, old := range b.literals {
		if old == literal {
			c := &b.literalCoefficients[index]
			return q.Add(c, coefficient, c)
		}
	}
	b.literals = append(b.literals, literal)
	b.literalCoefficients, err = smtQPush(q, b.literalCoefficients, coefficient)
	return err
}
func sameSMTEnodePair(left, right SMTEnodePair) bool {
	return left.First == right.First && left.Second == right.Second
}
func (b *smtArithmeticJustifiedDerivedBound) pushEquality(q *numeral.RationalManager, pair SMTEnodePair, coefficient *numeral.Rational) (err error) {
	if coefficient == nil || pair.First == nil || pair.Second == nil {
		return ast.ErrASTDomain
	}
	for index, old := range b.equalities {
		if sameSMTEnodePair(old, pair) {
			c := &b.equalityCoefficients[index]
			return q.Add(c, coefficient, c)
		}
	}
	b.equalities = append(b.equalities, pair)
	b.equalityCoefficients, err = smtQPush(q, b.equalityCoefficients, coefficient)
	return err
}
func (b *smtArithmeticJustifiedDerivedBound) pushJustification(q *numeral.RationalManager, out *smtArithmeticAntecedents, coefficient *numeral.Rational, proofs bool) error {
	if b == nil || out == nil || coefficient == nil {
		return ast.ErrASTDomain
	}
	scaled := smtQZero()
	defer smtQDel(q, &scaled)
	for index, literal := range b.literals {
		if err := q.Mul(coefficient, &b.literalCoefficients[index], &scaled); err != nil {
			return err
		}
		if err := out.pushLiteral(q, literal, &scaled, proofs); err != nil {
			return err
		}
	}
	for index, equality := range b.equalities {
		if err := q.Mul(coefficient, &b.equalityCoefficients[index], &scaled); err != nil {
			return err
		}
		if err := out.pushEquality(q, equality, &scaled, proofs); err != nil {
			return err
		}
	}
	return nil
}

type smtArithmeticDerivedTarget interface {
	pushLiteral(*numeral.RationalManager, SMTLiteral, *numeral.Rational) error
	pushEquality(*numeral.RationalManager, SMTEnodePair, *numeral.Rational) error
}

func smtAccumulateArithmeticJustification(q *numeral.RationalManager, source smtArithmeticBoundJustification, target smtArithmeticDerivedTarget, coefficient *numeral.Rational, proofs bool, literals map[int32]bool, equalities map[SMTEnodePair]bool) (err error) {
	if source == nil || target == nil || coefficient == nil || literals == nil || equalities == nil {
		return ast.ErrASTDomain
	}
	var antecedents smtArithmeticAntecedents
	defer func() {
		if e := antecedents.reset(q); e != nil && err == nil {
			err = e
		}
	}()
	if err := source.pushJustification(q, &antecedents, coefficient, proofs); err != nil {
		return err
	}
	zero := smtQZero()
	for index, literal := range antecedents.literals {
		if literals[literal.Index()] {
			continue
		}
		value := &zero
		if proofs {
			value = &antecedents.literalCoefficients[index]
		} else {
			literals[literal.Index()] = true
		}
		if err := target.pushLiteral(q, literal, value); err != nil {
			return err
		}
	}
	for index, equality := range antecedents.equalities {
		if equalities[equality] {
			continue
		}
		value := &zero
		if proofs {
			value = &antecedents.equalityCoefficients[index]
		} else {
			equalities[equality] = true
		}
		if err := target.pushEquality(q, equality, value); err != nil {
			return err
		}
	}
	return nil
}

// smtNormalizeArithmeticIntegerBound sets out to inf_numeral(ceil(k)) for a
// lower bound and inf_numeral(floor(k)) for an upper bound.
func smtNormalizeArithmeticIntegerBound(q *numeral.RationalManager, value *smtArithmeticValue, kind smtArithmeticBoundKind, out *smtArithmeticValue) error {
	if value == nil || !value.infinity.IsZero() || kind > smtArithmeticUpperBound {
		return ast.ErrASTDomain
	}
	r := smtQZero()
	defer smtQDel(q, &r)
	var err error
	if kind == smtArithmeticLowerBound {
		err = smtInfCeil(q, value, &r)
	} else {
		err = smtInfFloor(q, value, &r)
	}
	if err != nil {
		return err
	}
	return out.setRational(q, &r)
}

type smtArithmeticVariableKind uint8

const (
	smtArithmeticNonBase smtArithmeticVariableKind = iota
	smtArithmeticBase
	smtArithmeticQuasiBase
)

type smtArithmeticVariableData struct {
	rowID                        int
	kind                         smtArithmeticVariableKind
	integer, nonlinearPropagated bool
}
type smtArithmeticBoundTrail struct {
	encoded SMTTheoryVar
	old     *smtArithmeticBound
}

func newSMTArithmeticBoundTrail(variable SMTTheoryVar, old *smtArithmeticBound, upper bool) (smtArithmeticBoundTrail, error) {
	if variable < 0 {
		return smtArithmeticBoundTrail{}, ast.ErrASTDomain
	}
	encoded := variable << 1
	if upper {
		encoded++
	}
	return smtArithmeticBoundTrail{encoded: encoded, old: old}, nil
}
func (t smtArithmeticBoundTrail) upper() bool            { return t.encoded&1 != 0 }
func (t smtArithmeticBoundTrail) variable() SMTTheoryVar { return t.encoded >> 1 }
