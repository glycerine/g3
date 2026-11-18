package smtcore

// Source: theory_pb variable/literal watches, assign_watch, assign_ineq,
// assign_watch_ge and their scope trails. Numerals are concrete exact rationals.

import (
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (t *SMTPseudoBooleanTheory) compileArgument(expression ast.ASTHandle) (SMTLiteral, error) {
	s := t.state
	m := s.manager
	negative := false
	if op, basic, err := s.basicOperation(expression); err != nil {
		return SMTNullLiteral, err
	} else if basic && op == ast.ASTOpNot {
		expression, err = m.GetArg(expression, 0)
		if err != nil {
			return SMTNullLiteral, err
		}
		negative = true
	}
	// Source strips exactly one leading negation before this guard, even
	// when the remaining expression already has a Boolean variable.
	if smtcoreSourceDebug {
		if op, basic, err := s.basicOperation(expression); err != nil {
			return SMTNullLiteral, err
		} else if basic && op == ast.ASTOpNot {
			return SMTNullLiteral, ast.ErrASTDomain
		}
	}
	if _, found := s.boolVar.lookup(expression); !found {
		if err := s.Internalize(expression, false); err != nil {
			return SMTNullLiteral, err
		}
	}
	literal, err := s.Literal(expression)
	if err != nil {
		return SMTNullLiteral, err
	}
	if literal != SMTTrueLiteral && literal != SMTFalseLiteral {
		data, err := s.BooleanData(literal.Var())
		if err != nil {
			return SMTNullLiteral, err
		}
		family, err := m.GetFamilyID(expression)
		if err != nil {
			return SMTNullLiteral, err
		}
		if family == ast.NullFamilyID && !data.IsTheoryAtom() {
			if err = s.SetVariableTheory(literal.Var(), t.FamilyID()); err != nil {
				return SMTNullLiteral, err
			}
		}
		if data.Theory() != t.FamilyID() {
			proxy, err := m.MkFreshPseudoBooleanAux()
			if err != nil {
				return SMTNullLiteral, err
			}
			iff, err := m.MkIff(proxy, expression)
			if err != nil {
				return SMTNullLiteral, err
			}
			if err = t.emitPB("compile_arg", 644, func(out io.Writer) error {
				_, _ = io.WriteString(out, "create proxy ")
				// The source prints the IFF before it is internalized.
				return t.displayPBApplication(out, iff, true)
			}); err != nil {
				return SMTNullLiteral, err
			}
			if err = s.Internalize(iff, false); err != nil {
				return SMTNullLiteral, err
			}
			definition, err := s.Literal(iff)
			if err != nil {
				return SMTNullLiteral, err
			}
			if err = s.addTheoryAxiom(t.FamilyID(), []SMTLiteral{definition}); err != nil {
				return SMTNullLiteral, err
			}
			if err = s.markRelevant(proxy); err != nil {
				return SMTNullLiteral, err
			}
			literal, err = s.Literal(proxy)
			if err != nil {
				return SMTNullLiteral, err
			}
		}
	}
	if negative {
		literal = literal.Negated()
	}
	return literal, nil
}

// smtPBConstraint is ineq. The watch sums are scoped_mpz values; the source
// keeps them on the theory's private mpz manager, here they live on the AST
// manager's integer manager with the coefficients' numerators (ncoeff) and
// k's numerator (mpz_k) they are compared with.
type smtPBConstraint struct {
	literal                            SMTLiteral
	equality                           bool
	args                               [2]smtPBArgs
	negative, active                   bool
	watchSize                          int
	watchSum, maxWatch, minSum, maxSum numeral.Int
	fixed                              int
	propagations                       uint32
	compilationThreshold               uint32
	compiled                           lbool.LBool
}

// newSMTPBConstraint is alloc(ineq, ...): empty arguments, zero sums.
func newSMTPBConstraint(literal SMTLiteral, equality bool) *smtPBConstraint {
	return &smtPBConstraint{literal: literal, equality: equality, args: [2]smtPBArgs{newSMTPBArgs(), newSMTPBArgs()}}
}

// del is dealloc(c): ~ineq frees both arg_t and the scoped_mpz sums.
func (c *smtPBConstraint) del(q *numeral.RationalManager) error {
	z := q.Integers()
	err := smtErrJoin(c.args[0].del(q), c.args[1].del(q))
	for _, n := range []*numeral.Int{&c.watchSum, &c.maxWatch, &c.minSum, &c.maxSum} {
		err = smtErrJoin(err, z.Del(n))
	}
	return err
}

// q is the AST manager's rational manager (rational's manager).
func (t *SMTPseudoBooleanTheory) q() *numeral.RationalManager {
	return t.state.manager.NumeralService().Manager
}

// keep records the first numeral failure of a step that cannot return it.
func (t *SMTPseudoBooleanTheory) keep(err error) {
	if err != nil && t.qErr == nil {
		t.qErr = err
	}
}

// takeQErr returns err, else a failure kept since the last call.
func (t *SMTPseudoBooleanTheory) takeQErr(err error) error {
	if err == nil {
		err = t.qErr
	}
	t.qErr = nil
	return err
}

// zcmp is an mpz comparison whose failure is kept.
func (t *SMTPseudoBooleanTheory) zcmp(a, b *numeral.Int) int {
	c, err := t.q().Integers().Compare(a, b)
	t.keep(err)
	return c
}

// zsum sets out = a + b, keeping a failure.
func (t *SMTPseudoBooleanTheory) zsum(a, b, out *numeral.Int) {
	t.keep(t.q().Integers().Add(a, b, out))
}

// zsave is a copy of n for a value_trail; the undo restores and frees it.
func (t *SMTPseudoBooleanTheory) zsave(n *numeral.Int) (numeral.Int, error) {
	var out numeral.Int
	return out, t.q().Integers().Copy(&out, n)
}

func (t *SMTPseudoBooleanTheory) zrestore(n, saved *numeral.Int) {
	z := t.q().Integers()
	t.keep(z.Copy(n, saved))
	t.keep(z.Del(saved))
}

func (c *smtPBConstraint) current() *smtPBArgs {
	if c.negative {
		return &c.args[1]
	}
	return &c.args[0]
}
func (c *smtPBConstraint) lit() SMTLiteral {
	if c.negative {
		return c.literal.Negated()
	}
	return c.literal
}

type smtPBVariableWatches struct {
	literals   [2][]*smtPBConstraint
	variables  []*smtPBConstraint
	constraint *smtPBConstraint
}

func (t *SMTPseudoBooleanTheory) watchVariables() []int32 {
	var variables []int32
	for v := range t.watches {
		variables = append(variables, v)
	}
	sort.Slice(variables, func(i, j int) bool { return variables[i] < variables[j] })
	return variables
}

// Source pb_justification retains the inequality used for propagation.
type smtPBJustification struct {
	*SMTTheoryPropagationJustification
	constraint *smtPBConstraint
}

func (t *SMTPseudoBooleanTheory) info(v int32) *smtPBVariableWatches {
	if t.watches == nil {
		t.watches = make(map[int32]*smtPBVariableWatches)
	}
	i := t.watches[v]
	if i == nil {
		i = &smtPBVariableWatches{}
		t.watches[v] = i
	}
	return i
}

func smtPBSign(l SMTLiteral) int {
	if l.Sign() {
		return 1
	}
	return 0
}

func smtPBRemove(list []*smtPBConstraint, c *smtPBConstraint) []*smtPBConstraint {
	for i, p := range list {
		if p == c {
			list[i] = list[len(list)-1]
			list[len(list)-1] = nil
			return list[:len(list)-1]
		}
	}
	return list
}

func (t *SMTPseudoBooleanTheory) clearWatch(c *smtPBConstraint) {
	for _, term := range c.current().terms {
		i := t.info(term.literal.Var())
		i.variables = smtPBRemove(i.variables, c)
		sign := smtPBSign(term.literal)
		i.literals[sign] = smtPBRemove(i.literals[sign], c)
	}
	z := t.q().Integers()
	c.watchSize = 0
	t.keep(z.Reset(&c.watchSum))
	t.keep(z.Reset(&c.maxWatch))
	t.keep(z.Reset(&c.minSum))
	t.keep(z.Reset(&c.maxSum))
	c.fixed = 0
}

func (t *SMTPseudoBooleanTheory) initVariableWatch(c *smtPBConstraint) {
	z := t.q().Integers()
	t.keep(z.Reset(&c.minSum))
	t.keep(z.Reset(&c.maxSum))
	c.fixed = 0
	a := c.current()
	for i := range a.terms {
		info := t.info(a.terms[i].literal.Var())
		info.variables = append(info.variables, c)
		t.zsum(&c.maxSum, &a.terms[i].coeff.Num, &c.maxSum)
	}
}

func (t *SMTPseudoBooleanTheory) addWatch(c *smtPBConstraint, index int) {
	a := c.current()
	t.zsum(&c.watchSum, &a.terms[index].coeff.Num, &c.watchSum)
	a.terms[index], a.terms[c.watchSize] = a.terms[c.watchSize], a.terms[index]
	term := &a.terms[c.watchSize]
	c.watchSize++
	if t.zcmp(&term.coeff.Num, &c.maxWatch) > 0 {
		t.keep(t.q().Integers().Copy(&c.maxWatch, &term.coeff.Num))
	}
	i := t.info(term.literal.Var())
	sign := smtPBSign(term.literal)
	i.literals[sign] = append(i.literals[sign], c)
}

func (t *SMTPseudoBooleanTheory) initLiteralWatch(c *smtPBConstraint) error {
	if smtcoreSourceDebug && c.equality {
		return ast.ErrASTDomain
	}
	z := t.q().Integers()
	c.watchSize = 0
	t.keep(z.Reset(&c.watchSum))
	t.keep(z.Reset(&c.maxWatch))
	var limit numeral.Int // mpz_k() + max_watch()
	defer func() { t.keep(z.Del(&limit)) }()
	for i := 0; i < len(c.current().terms); i++ {
		value, err := t.state.Assignment(c.current().terms[i].literal)
		if err != nil {
			return err
		}
		if value != lbool.False {
			t.addWatch(c, i)
			t.zsum(&c.current().bound.Num, &c.maxWatch, &limit)
			if t.zcmp(&c.watchSum, &limit) >= 0 {
				break
			}
		}
	}
	return t.takeQErr(nil)
}

func (t *SMTPseudoBooleanTheory) helpful(c *smtPBConstraint, negate bool) ([]SMTLiteral, error) {
	var out []SMTLiteral
	var sum numeral.Int
	defer func() { t.keep(t.q().Integers().Del(&sum)) }()
	for i := range c.current().terms {
		term := &c.current().terms[i]
		if t.zcmp(&sum, &c.current().bound.Num) >= 0 {
			break
		}
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return nil, err
		}
		if v == lbool.True {
			l := term.literal
			if negate {
				l = l.Negated()
			}
			out = append(out, l)
			t.zsum(&sum, &term.coeff.Num, &sum)
		}
	}
	if smtcoreSourceDebug && t.zcmp(&sum, &c.current().bound.Num) < 0 {
		return nil, ast.ErrASTDomain
	}
	return out, t.takeQErr(nil)
}

func (t *SMTPseudoBooleanTheory) unhelpful(c *smtPBConstraint, negate bool) ([]SMTLiteral, error) {
	var out []SMTLiteral
	for _, term := range c.current().terms {
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return nil, err
		}
		if v == lbool.False {
			l := term.literal
			if negate {
				l = l.Negated()
			}
			out = append(out, l)
		}
	}
	return out, nil
}

func (t *SMTPseudoBooleanTheory) allLiterals(c *smtPBConstraint) ([]SMTLiteral, error) {
	var out []SMTLiteral
	for _, term := range c.current().terms {
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return nil, err
		}
		if v != lbool.Undef {
			l := term.literal
			if v == lbool.False {
				l = l.Negated()
			}
			out = append(out, l)
		}
	}
	return out, nil
}

func (t *SMTPseudoBooleanTheory) addAssign(c *smtPBConstraint, premises []SMTLiteral, consequent SMTLiteral) error {
	t.incPropagations(c)
	t.statistics.propagations++
	if err := t.traceReason("add_assign", 1431, c, premises, consequent); err != nil {
		return err
	}
	base, err := NewSMTTheoryPropagationJustification(t.state.manager, t.FamilyID(), premises, consequent, nil)
	if err != nil {
		return err
	}
	owner := &smtPBJustification{SMTTheoryPropagationJustification: base, constraint: c}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return err
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	return t.state.AssignLiteral(consequent, wrapped)
}

func (t *SMTPseudoBooleanTheory) addConflict(c *smtPBConstraint, clause []SMTLiteral) error {
	t.incPropagations(c)
	t.statistics.conflicts++
	if err := t.traceReason("add_clause", 1454, c, clause, SMTNullLiteral); err != nil {
		return err
	}
	frequency := t.controls.ConflictFrequency
	if frequency == 0 || frequency-1 == c.propagations%frequency {
		if _, err := t.resolveConflict(c); err != nil {
			return err
		}
	}
	var owner SMTJustification
	proofs, err := t.state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		owner, err = NewSMTTheoryLemmaJustification(t.state.manager, t.FamilyID(), clause, t.state.boolExpr, nil)
		if err != nil {
			return err
		}
	}
	// Source conflict clauses are auxiliary lemmas, whose false literals
	// remain available to resolution even without proof production.
	if err = t.traceClause("add_clause", 1469, clause); err != nil {
		return err
	}
	return t.state.addClauseKindWithJustificationAndDeleteHandler(clause, false, SMTClauseAuxLemma, ast.ASTHandle{}, owner, nil)
}

func (t *SMTPseudoBooleanTheory) assignVariable(v int32, isTrue bool, c *smtPBConstraint) error {
	asgn, err := t.state.Assignment(c.lit())
	if err != nil {
		return err
	}
	a := c.current()
	if t.zcmp(&c.maxSum, &a.bound.Num) < 0 && asgn == lbool.False || !c.equality && t.zcmp(&c.minSum, &a.bound.Num) >= 0 && asgn == lbool.True {
		return t.takeQErr(nil)
	}
	index := -1
	for i, term := range a.terms {
		if term.literal.Var() == v {
			index = i
			break
		}
	}
	if err = t.emitPB("assign_watch", 1018, func(out io.Writer) error {
		_, _ = io.WriteString(out, "assign watch ")
		if err := smtPBWriteLiteral(out, NewSMTLiteral(v, !isTrue)); err != nil {
			return err
		}
		_, _ = io.WriteString(out, " ")
		return t.displayInequality(out, c, true)
	}); err != nil {
		return err
	}
	if index < 0 {
		return ast.ErrASTDomain
	}
	z := t.q().Integers()
	// value_trail<scoped_mpz> for the sum that changes, then m_nfixed.
	if a.terms[index].literal.Sign() == isTrue {
		oldMax, err := t.zsave(&c.maxSum)
		if err != nil {
			return err
		}
		_ = t.trail.PushUndo(func() { t.zrestore(&c.maxSum, &oldMax) })
		err = z.Sub(&c.maxSum, &a.terms[index].coeff.Num, &c.maxSum)
		if err != nil {
			return err
		}
	} else {
		oldMin, err := t.zsave(&c.minSum)
		if err != nil {
			return err
		}
		_ = t.trail.PushUndo(func() { t.zrestore(&c.minSum, &oldMin) })
		if err = z.Add(&c.minSum, &a.terms[index].coeff.Num, &c.minSum); err != nil {
			return err
		}
	}
	oldFixed := c.fixed
	_ = t.trail.PushUndo(func() { c.fixed = oldFixed })
	if err = t.validateVariableSums(c); err != nil {
		return err
	}
	c.fixed++
	if smtcoreSourceDebug && c.fixed > len(a.terms) {
		return ast.ErrASTDomain
	}
	if !c.equality && t.zcmp(&c.minSum, &a.bound.Num) >= 0 && asgn != lbool.True {
		if err = t.traceAssignment("assign_watch", 1047, "Set ", c.lit(), c, true); err != nil {
			return err
		}
		ls, err := t.helpful(c, false)
		if err != nil {
			return err
		}
		return t.addAssign(c, ls, c.lit())
	}
	if t.zcmp(&c.maxSum, &a.bound.Num) < 0 && asgn != lbool.False {
		if err = t.traceAssignment("assign_watch", 1051, "Set ", c.lit().Negated(), c, true); err != nil {
			return err
		}
		ls, err := t.unhelpful(c, true)
		if err != nil {
			return err
		}
		return t.addAssign(c, ls, c.lit().Negated())
	}
	if c.equality && c.fixed == len(a.terms) {
		holds := t.zcmp(&c.minSum, &a.bound.Num) == 0
		if holds && asgn != lbool.True || !holds && asgn != lbool.False {
			line := 1055
			literal := c.lit()
			if !holds {
				line = 1059
				literal = literal.Negated()
			}
			if err = t.traceAssignment("assign_watch", line, "Set ", literal, c, true); err != nil {
				return err
			}
			ls, err := t.allLiterals(c)
			if err != nil {
				return err
			}
			l := c.lit()
			if !holds {
				l = l.Negated()
			}
			return t.addAssign(c, ls, l)
		}
	}
	// else IF_VERBOSE(14, display(verbose_stream() << "no propagation ", c,
	// true)) (AUDIT.md FUZZ-F5; g3trace builds only).
	if diagnostic.VerboseProgress {
		return smtPBVerbose(14, func(out io.Writer) error {
			_, _ = io.WriteString(out, "no propagation ")
			return t.displayInequality(out, c, true)
		})
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) assignInequality(c *smtPBConstraint, isTrue bool) error {
	if c.equality || c.active {
		return nil
	}
	z := t.q().Integers()
	oldMax, err := t.zsave(&c.maxSum)
	if err != nil {
		return err
	}
	oldMin, err := t.zsave(&c.minSum)
	if err != nil {
		return smtErrJoin(err, z.Del(&oldMax))
	}
	oldFixed := c.fixed
	oldNegative := c.negative
	_ = t.trail.PushUndo(func() {
		t.clearWatch(c)
		c.negative = oldNegative
		c.active = false
		t.initVariableWatch(c)
		t.zrestore(&c.minSum, &oldMin)
		t.zrestore(&c.maxSum, &oldMax)
		c.fixed = oldFixed
	})
	t.clearWatch(c)
	c.negative = !isTrue
	c.active = true
	// scoped_mpz maxsum, mininc
	var maxsum, mininc, rest numeral.Int
	defer func() { t.keep(smtErrJoin(z.Del(&rest), z.Del(&mininc), z.Del(&maxsum))) }()
	for i := range c.current().terms {
		term := &c.current().terms[i]
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return err
		}
		if v != lbool.False {
			t.zsum(&maxsum, &term.coeff.Num, &maxsum)
		}
		if v == lbool.Undef && (mininc.IsSmall() && mininc.Val == 0 || t.zcmp(&mininc, &term.coeff.Num) > 0) {
			t.keep(z.Copy(&mininc, &term.coeff.Num))
		}
	}
	if err := t.takeQErr(nil); err != nil {
		return err
	}
	if err := t.traceAssignment("assign_ineq", 949, "assign: ", c.lit(), c, false); err != nil {
		return err
	}
	if t.zcmp(&maxsum, &c.current().bound.Num) < 0 {
		ls, err := t.unhelpful(c, false)
		if err != nil {
			return err
		}
		return t.addConflict(c, append(ls, c.lit().Negated()))
	}
	if err := t.initLiteralWatch(c); err != nil {
		return err
	}
	if err := t.validateWatch(c); err != nil {
		return err
	}
	if err := z.Sub(&maxsum, &mininc, &rest); err != nil {
		return err
	}
	if t.zcmp(&rest, &c.current().bound.Num) < 0 {
		ls, err := t.unhelpful(c, true)
		if err != nil {
			return err
		}
		ls = append(ls, c.lit())
		for _, term := range c.current().terms {
			v, err := t.state.Assignment(term.literal)
			if err != nil {
				return err
			}
			if v == lbool.Undef {
				if err = t.validateAssign(c, ls, term.literal); err != nil {
					return err
				}
				if err = t.addAssign(c, ls, term.literal); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) assignLiteralWatch(v int32, isTrue bool, c *smtPBConstraint, watchIndex int) (bool, error) {
	a := c.current()
	if smtcoreSourceDebug && (c.equality || c.watchSize < 0 || c.watchSize > len(a.terms)) {
		return false, ast.ErrASTDomain
	}
	w := -1
	for i := 0; i < c.watchSize; i++ {
		if a.terms[i].literal.Var() == v {
			w = i
			break
		}
	}
	if w < 0 {
		return false, ast.ErrASTDomain
	}
	if smtcoreSourceDebug {
		value, err := t.state.Assignment(c.lit())
		if err != nil {
			return false, err
		}
		if value != lbool.True || isTrue != a.terms[w].literal.Sign() {
			return false, ast.ErrASTDomain
		}
	}
	// scoped_mpz k_coeff = k + ncoeff(w); limit is k_coeff + max_watch().
	z := t.q().Integers()
	var kCoeff, limit, deficit numeral.Int
	defer func() { t.keep(smtErrJoin(z.Del(&deficit), z.Del(&limit), z.Del(&kCoeff))) }()
	t.zsum(&a.bound.Num, &a.terms[w].coeff.Num, &kCoeff)
	addMore := func() bool {
		t.zsum(&kCoeff, &c.maxWatch, &limit)
		return t.zcmp(&c.watchSum, &limit) < 0
	}
	for i := c.watchSize; addMore() && i < len(a.terms); i++ {
		value, err := t.state.Assignment(a.terms[i].literal)
		if err != nil {
			return false, err
		}
		if value != lbool.False {
			t.addWatch(c, i)
		}
	}
	if err := t.takeQErr(nil); err != nil {
		return false, err
	}
	if t.zcmp(&c.watchSum, &kCoeff) < 0 {
		ls, err := t.unhelpful(c, false)
		if err != nil {
			return false, err
		}
		if err = t.addConflict(c, append(ls, c.lit().Negated())); err != nil {
			return false, err
		}
		return false, t.traceAssignment("assign_watch_ge", 1157, "assign: ", NewSMTLiteral(v, !isTrue), c, false)
	}
	term := a.terms[w] // the moved slot keeps the coefficient
	info := t.info(v)
	sign := 0
	if isTrue {
		sign = 1
	}
	list := info.literals[sign]
	list[watchIndex] = list[len(list)-1]
	list[len(list)-1] = nil
	info.literals[sign] = list[:len(list)-1]
	a.terms[w], a.terms[c.watchSize-1] = a.terms[c.watchSize-1], a.terms[w]
	c.watchSize--
	t.keep(z.Sub(&c.watchSum, &term.coeff.Num, &c.watchSum))
	if t.zcmp(&term.coeff.Num, &c.maxWatch) == 0 {
		t.keep(z.Reset(&c.maxWatch))
		for i := 0; i < c.watchSize; i++ {
			if t.zcmp(&a.terms[i].coeff.Num, &c.maxWatch) > 0 {
				t.keep(z.Copy(&c.maxWatch, &a.terms[i].coeff.Num))
			}
		}
	}
	if err := t.takeQErr(nil); err != nil {
		return true, err
	}
	if smtcoreSourceDebug && t.zcmp(&c.watchSum, &a.bound.Num) < 0 {
		return true, ast.ErrASTDomain
	}
	t.zsum(&a.bound.Num, &c.maxWatch, &limit)
	if t.zcmp(&c.watchSum, &limit) < 0 {
		ls, err := t.unhelpful(c, true)
		if err != nil {
			return true, err
		}
		ls = append(ls, c.lit())
		if err := z.Sub(&c.watchSum, &a.bound.Num, &deficit); err != nil {
			return true, err
		}
		for i := range a.terms {
			term := &a.terms[i]
			value, err := t.state.Assignment(term.literal)
			if err != nil {
				return true, err
			}
			if value == lbool.Undef && t.zcmp(&deficit, &term.coeff.Num) < 0 {
				if err = t.validateAssign(c, ls, term.literal); err != nil {
					return true, err
				}
				if err = t.addAssign(c, ls, term.literal); err != nil {
					return true, err
				}
			}
		}
	}
	if err := t.traceAssignment("assign_watch_ge", 1157, "assign: ", NewSMTLiteral(v, !isTrue), c, false); err != nil {
		return true, err
	}
	return true, nil
}

func (t *SMTPseudoBooleanTheory) assignNative(v int32, isTrue bool) error {
	i := t.info(v)
	if err := t.traceAssignment("assign_eh", 785, "assign: ", NewSMTLiteral(v, !isTrue), nil, false); err != nil {
		return err
	}
	sign := 0
	if isTrue {
		sign = 1
	}
	// z3 4.7.1 bug: pop empties a freed Boolean's watch list but keeps it,
	// and the source bounds the Boolean's column whenever the list exists.
	// Reused as a PB predicate, that column is the predicate's slack, and
	// the 0-1 bound makes z3 answer unsat on satisfiable problems. The
	// fork fix bounds only 0-1 columns (m_vars.contains(v)).
	if t.controls.EnableSimplex && i.literals[sign] != nil && t.simplexIsVariable(v) {
		ok, err := t.simplexAssignArgument(v, isTrue)
		if err != nil || !ok {
			return err
		}
	}
	for n := 0; n < len(i.literals[sign]); {
		removed, err := t.assignLiteralWatch(v, isTrue, i.literals[sign][n], n)
		if err != nil {
			return err
		}
		if !removed {
			n++
		}
	}
	for _, c := range i.variables {
		if err := t.assignVariable(v, isTrue, c); err != nil {
			return err
		}
	}
	if i.constraint != nil {
		if t.controls.EnableSimplex && t.simplexHasRow(v) {
			ok, err := t.simplexAssignPredicate(i.constraint, isTrue)
			if err != nil || !ok {
				return err
			}
		}
		return t.assignInequality(i.constraint, isTrue)
	}
	return nil
}
