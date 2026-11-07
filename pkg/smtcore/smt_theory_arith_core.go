package smtcore

// Source: src/smt/theory_arith_core.h, ported in source order. Calls to
// get_context() go to t.state; calls to m_util build or inspect AST through
// the manager. The value_trail entries the source pushes for
// m_found_unsupported_op, m_found_underspecified_op and m_final_check_idx are
// kept in the theory scope records, which the context pops in step.

import (
	"errors"
	"math"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// ---- m_util (arith_util) helpers used by this file ----

// arithOp is is_app_of(n, m_afid, k): the declaration kind of an
// arithmetic application, and whether n is one.
func (t *SMTTheoryArith) arithOp(n ast.ASTHandle) (int32, bool) {
	m := t.state.manager
	kind, err := m.GetKind(n)
	if err != nil || kind != ast.ASTApp {
		return 0, false
	}
	family, err := m.GetFamilyID(n)
	if err != nil || family != t.FamilyID() {
		return 0, false
	}
	op, err := m.GetDeclKind(n)
	return op, err == nil
}

func (t *SMTTheoryArith) isOp(n ast.ASTHandle, op int32) bool {
	k, own := t.arithOp(n)
	return own && k == op
}

// isNumeral is m_util.is_numeral(n, r).
func (t *SMTTheoryArith) isNumeral(n ast.ASTHandle) (*big.Rat, bool) {
	text, _, matched, err := t.state.manager.ArithmeticNumeralText(n)
	if err != nil || !matched {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(text)
	return r, ok
}

// isIntSort is m_util.is_int(e): e has sort Int.
func (t *SMTTheoryArith) isIntSort(e ast.ASTHandle) (bool, error) {
	m := t.state.manager
	sort, err := m.GetSort(e)
	if err != nil {
		return false, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil || family != t.FamilyID() {
		return false, err
	}
	kind, err := m.GetDeclKind(sort)
	return kind == smtArithIntSort, err
}

// isRealSort is m_util.is_real(e).
func (t *SMTTheoryArith) isRealSort(e ast.ASTHandle) (bool, error) {
	m := t.state.manager
	sort, err := m.GetSort(e)
	if err != nil {
		return false, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil || family != t.FamilyID() {
		return false, err
	}
	kind, err := m.GetDeclKind(sort)
	return kind == smtArithRealSort, err
}

const (
	smtArithRealSort                 int32 = 0  // REAL_SORT
	smtArithIntSort                  int32 = 1  // INT_SORT
	smtArithOpIrrationalAlgebraicNum int32 = 1  // OP_IRRATIONAL_ALGEBRAIC_NUM
	smtArithOpPower                  int32 = 18 // OP_POWER
)

func (t *SMTTheoryArith) args(n ast.ASTHandle) ([]ast.ASTHandle, error) {
	m := t.state.manager
	count, err := m.GetNumArgs(n)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, count)
	for i := range result {
		if result[i], err = m.GetArg(n, uint32(i)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// smtArithExprs holds references to the AST nodes a function creates,
// the expr_ref locals of the source.
type smtArithExprs struct {
	m    *ast.ASTManager
	refs []ast.ASTRef
}

func (x *smtArithExprs) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return h, err
	}
	ref, err := x.m.NewRef(h)
	if err != nil {
		return h, err
	}
	x.refs = append(x.refs, ref)
	return h, nil
}

func (x *smtArithExprs) release() (result error) {
	for i := len(x.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, x.m.DelRef(&x.refs[i]))
	}
	x.refs = nil
	return result
}

func (t *SMTTheoryArith) mkApp(x *smtArithExprs, op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return x.hold(t.state.manager.MkFamilyApp(t.FamilyID(), op, nil, args))
}

// mkNumeral is m_util.mk_numeral(r, is_int).
func (t *SMTTheoryArith) mkNumeral(x *smtArithExprs, r *big.Rat, isInt bool) (ast.ASTHandle, error) {
	if isInt {
		return x.hold(t.state.manager.ArithmeticIntegerNumeralText(r.RatString()))
	}
	return x.hold(t.state.manager.ArithmeticRealNumeralText(r.RatString()))
}

// ---- core.h:30-100 ----

func (t *SMTTheoryArith) foundUnsupportedOpEh(n ast.ASTHandle) {
	if !t.foundUnsupportedOp {
		t.foundUnsupportedOp = true
	}
}

func (t *SMTTheoryArith) foundUnderspecifiedOpEh(n ast.ASTHandle) {
	if !t.foundUnderspecifiedOp {
		t.foundUnderspecifiedOp = true
	}
}

func (t *SMTTheoryArith) processAtoms() bool {
	if !t.adaptive() {
		return true
	}
	totalConflicts := t.state.statistics.NumConflicts
	if totalConflicts < 10 {
		return true
	}
	f := float64(t.getNumConflicts()) / float64(totalConflicts)
	return f >= t.adaptiveAssertionThreshold()
}

func (t *SMTTheoryArith) isIntExpr(e ast.ASTHandle) (bool, error) {
	if isInt, err := t.isIntSort(e); err != nil || isInt {
		return isInt, err
	}
	if t.isUninterp(e) {
		return false, nil
	}
	todo := []ast.ASTHandle{e}
	i := 0
	for len(todo) > 0 {
		i++
		if i > 100 {
			return false, nil
		}
		e = todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if t.isOp(e, smtArithOpToReal) {
			// pass
		} else if r, ok := t.isNumeral(e); ok && r.IsInt() {
			// pass
		} else if t.isOp(e, smtArithOpAdd) || t.isOp(e, smtArithOpMul) {
			args, err := t.args(e)
			if err != nil {
				return false, err
			}
			todo = append(todo, args...)
		} else {
			return false, nil
		}
	}
	return true, nil
}

// isUninterp is ::is_uninterp: an application of a declaration without a
// family.
func (t *SMTTheoryArith) isUninterp(e ast.ASTHandle) bool {
	if !t.isApp(e) {
		return false
	}
	family, err := t.state.manager.GetFamilyID(e)
	return err == nil && family == ast.NullFamilyID
}

// ---- core.h:103-142 ----

func (t *SMTTheoryArith) mkVar(n *SMTEnode) (SMTTheoryVar, error) {
	r, err := t.SMTTheoryBase.CreateVariable(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if int(r) != len(t.columns) {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	isInt, err := t.isIntExpr(n.owner)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	t.columns = append(t.columns, newSMTArithmeticColumn())
	t.data = append(t.data, smtArithmeticVariableData{integer: isInt})
	var value smtArithmeticValue
	if t.randomInitialValue() {
		// unsigned arithmetic, as in the source
		val := t.random.Next()%uint32(t.randomUpper()-t.randomLower()) + uint32(t.randomLower())
		value.rational.SetInt64(int64(val))
	}
	t.value = append(t.value, value)
	t.oldValue = append(t.oldValue, smtArithmeticValue{})
	t.varOccs = append(t.varOccs, nil)
	t.unassignedAtoms = append(t.unassignedAtoms, 0)
	t.varPos = append(t.varPos, -1)
	t.bounds[0] = append(t.bounds[0], nil)
	t.bounds[1] = append(t.bounds[1], nil)
	if int32(r) >= t.toPatch.Bounds() {
		if err = t.toPatch.SetBounds(int32(r) + 1); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	t.inUpdateTrailStack.AssureDomain(uint32(r))
	t.leftBasis.AssureDomain(uint32(r))
	t.inToCheck.AssureDomain(uint32(r))
	if t.isPureMonomial(n.owner) {
		t.nlMonomials = append(t.nlMonomials, r)
	}
	return r, nil
}

// is_pure_monomial (theory_arith.h:963).
func (t *SMTTheoryArith) isPureMonomial(m ast.ASTHandle) bool {
	if !t.isOp(m, smtArithOpMul) {
		return false
	}
	arg0, err := t.state.manager.GetArg(m, 0)
	if err != nil {
		return false
	}
	_, numeral := t.isNumeral(arg0)
	return !numeral
}

// ---- core.h:144-198 ----

func (t *SMTTheoryArith) reflectionEnabled() bool { return t.params.Reflect }

func (t *SMTTheoryArith) reflect(n ast.ASTHandle) bool {
	if t.reflectionEnabled() {
		return true // reflect everything
	}
	// Every underspecified operator must be reflected in the egraph.
	if op, own := t.arithOp(n); own {
		switch op {
		case smtArithOpDiv, smtArithOpIDiv, smtArithOpRem, smtArithOpMod:
			return true
		}
	}
	return false
}

func (t *SMTTheoryArith) enableCgcFor(n ast.ASTHandle) bool {
	// Congruence closure is not enabled for (+ ...) and (* ...) applications.
	op, own := t.arithOp(n)
	return !(own && (op == smtArithOpAdd || op == smtArithOpMul))
}

func (t *SMTTheoryArith) mkEnode(n ast.ASTHandle) (*SMTEnode, error) {
	if e := t.state.context.enodes.get(n); e != nil {
		return e, nil
	}
	return t.state.MkEnode(n, !t.reflect(n), false, t.enableCgcFor(n))
}

func (t *SMTTheoryArith) mkEnodeIfReflect(n ast.ASTHandle) error {
	if t.reflectionEnabled() {
		_, err := t.mkEnode(n)
		return err
	}
	return nil
}

// ---- core.h:200-251 ----

func (t *SMTTheoryArith) rowVarsCurrent() map[SMTTheoryVar]bool {
	return t.rowVars[t.rowVarsTop-1]
}

// addRowEntry adds coeff * v (or -coeff * v when invert) to row rID and
// updates the column.
func (t *SMTTheoryArith) addRowEntry(invert bool, rID int, coeff *big.Rat, v SMTTheoryVar) error {
	r := &t.rows[rID]
	c := &t.columns[v]
	if t.rowVarsCurrent()[v] {
		for i := range r.entries {
			it := &r.entries[i]
			if it.variable == v {
				if invert {
					it.coefficient.Sub(&it.coefficient, coeff)
				} else {
					it.coefficient.Add(&it.coefficient, coeff)
				}
				return nil
			}
		}
		return ast.ErrASTDomain
	}
	t.rowVarsCurrent()[v] = true
	rIdx, rEntry := r.addEntry()
	cIdx, cEntry := c.addEntry()
	rEntry.variable = v
	rEntry.coefficient.Set(coeff)
	if invert {
		rEntry.coefficient.Neg(&rEntry.coefficient)
	}
	rEntry.columnIndex = cIdx
	cEntry.rowID = rID
	cEntry.rowIndex = rIdx
	return nil
}

// scoped_row_vars (core.h:706-723).
func (t *SMTTheoryArith) pushRowVars() {
	if uint32(len(t.rowVars)) == t.rowVarsTop {
		t.rowVars = append(t.rowVars, make(map[SMTTheoryVar]bool))
	}
	clear(t.rowVars[t.rowVarsTop])
	t.rowVarsTop++
}
func (t *SMTTheoryArith) popRowVars() { t.rowVarsTop-- }

// ---- core.h:253-324 ----

func (t *SMTTheoryArith) internalizeInternalMonomial(m ast.ASTHandle, rID int) error {
	minusOne := big.NewRat(-1, 1)
	if e := t.state.context.enodes.get(m); e != nil && t.IsAttachedToVariable(e) {
		// there is already a theory variable (i.e., name) for m.
		return t.addRowEntry(false, rID, minusOne, e.TheoryVar(t.FamilyID()))
	}
	if t.isOp(m, smtArithOpMul) {
		args, err := t.args(m)
		if err != nil {
			return err
		}
		if len(args) == 2 {
			if val, ok := t.isNumeral(args[0]); ok && t.isApp(args[0]) && t.isApp(args[1]) {
				v, err := t.internalizeTermCore(args[1])
				if err != nil {
					return err
				}
				if t.reflectionEnabled() {
					if _, err = t.internalizeTermCore(args[0]); err != nil {
						return err
					}
					if _, err = t.mkEnode(m); err != nil {
						return err
					}
				}
				return t.addRowEntry(true, rID, val, v)
			}
		}
	}
	v, err := t.internalizeTermCore(m)
	if err != nil {
		return err
	}
	return t.addRowEntry(false, rID, minusOne, v)
}

func (t *SMTTheoryArith) isApp(n ast.ASTHandle) bool {
	kind, err := t.state.manager.GetKind(n)
	return err == nil && kind == ast.ASTApp
}

func (t *SMTTheoryArith) isVar(n ast.ASTHandle) bool {
	kind, err := t.state.manager.GetKind(n)
	return err == nil && kind == ast.ASTVar
}

var errSMTArithFreeVariable = errors.New("smt: arithmetic term contains a free variable")

func (t *SMTTheoryArith) internalizeAdd(n ast.ASTHandle) (SMTTheoryVar, error) {
	rID := t.mkRow()
	t.pushRowVars()
	defer t.popRowVars()
	args, err := t.args(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	for _, arg := range args {
		if t.isVar(arg) {
			return SMTNullTheoryVar, errSMTArithFreeVariable
		}
		if err = t.internalizeInternalMonomial(arg, rID); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	e, err := t.mkEnode(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	v := e.TheoryVar(t.FamilyID())
	if v == SMTNullTheoryVar {
		if v, err = t.mkVar(e); err != nil {
			return SMTNullTheoryVar, err
		}
		if err = t.addRowEntry(false, rID, big.NewRat(1, 1), v); err != nil {
			return SMTNullTheoryVar, err
		}
		return v, t.initRow(rID)
	}
	// n was already internalized by the calls above, which can happen when
	// one of them invokes mk_axiom (for example a nested to_int(t) term).
	t.delRow(rID)
	return v, nil
}

// ---- core.h:326-382 ----

func (t *SMTTheoryArith) internalizeMulCore(m ast.ASTHandle) (SMTTheoryVar, error) {
	if !t.isOp(m, smtArithOpMul) {
		return t.internalizeTermCore(m)
	}
	args, err := t.args(m)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	for _, arg := range args {
		v, err := t.internalizeTermCore(arg)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		if v == SMTNullTheoryVar {
			e, err := t.mkEnode(arg)
			if err != nil {
				return SMTNullTheoryVar, err
			}
			if _, err = t.mkVar(e); err != nil {
				return SMTNullTheoryVar, err
			}
		}
	}
	e, err := t.mkEnode(m)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	v := e.TheoryVar(t.FamilyID())
	if v == SMTNullTheoryVar {
		return t.mkVar(e)
	}
	return v, nil
}

func (t *SMTTheoryArith) internalizeMul(m ast.ASTHandle) (SMTTheoryVar, error) {
	args, err := t.args(m)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if val, ok := t.isNumeral(args[0]); ok {
		rID := t.mkRow()
		t.pushRowVars()
		defer t.popRowVars()
		if t.isVar(args[1]) {
			return SMTNullTheoryVar, errSMTArithFreeVariable
		}
		if t.reflectionEnabled() {
			if _, err = t.internalizeTermCore(args[0]); err != nil {
				return SMTNullTheoryVar, err
			}
		}
		v, err := t.internalizeMulCore(args[1])
		if err != nil {
			return SMTNullTheoryVar, err
		}
		if err = t.addRowEntry(true, rID, val, v); err != nil {
			return SMTNullTheoryVar, err
		}
		e, err := t.mkEnode(m)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		s, err := t.mkVar(e)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		if err = t.addRowEntry(false, rID, big.NewRat(1, 1), s); err != nil {
			return SMTNullTheoryVar, err
		}
		return s, t.initRow(rID)
	}
	return t.internalizeMulCore(m)
}

// ---- core.h:384-443 ----

func (t *SMTTheoryArith) mkBinaryOp(n ast.ASTHandle) (SMTTheoryVar, error) {
	if t.state.context.enodes.get(n) != nil {
		return t.expr2var(n), nil
	}
	args, err := t.args(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if err = t.state.Internalize(args[0], false); err != nil {
		return SMTNullTheoryVar, err
	}
	if err = t.state.Internalize(args[1], false); err != nil {
		return SMTNullTheoryVar, err
	}
	e, err := t.mkEnode(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	return t.mkVar(e)
}

// divisorUnderspecified is !m_util.is_numeral(arg1, r) || r.is_zero().
func (t *SMTTheoryArith) divisorUnderspecified(n ast.ASTHandle) (bool, error) {
	q, err := t.state.manager.GetArg(n, 1)
	if err != nil {
		return false, err
	}
	r, ok := t.isNumeral(q)
	return !ok || r.Sign() == 0, nil
}

// relevancy is ctx.relevancy(): m_relevancy_lvl > 0.
func (t *SMTTheoryArith) relevancy() bool { return t.state.relevancyLevel > 0 }

func (t *SMTTheoryArith) internalizeDiv(n ast.ASTHandle) (SMTTheoryVar, error) {
	under, err := t.divisorUnderspecified(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if under {
		t.foundUnderspecifiedOpEh(n)
	}
	t.foundUnderspecifiedOpEh(n)
	s, err := t.mkBinaryOp(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if !t.relevancy() {
		args, err := t.args(n)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		return s, t.mkDivAxiom(args[0], args[1])
	}
	return s, nil
}

func (t *SMTTheoryArith) internalizeIdiv(n ast.ASTHandle) (result SMTTheoryVar, err error) {
	under, err := t.divisorUnderspecified(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if under {
		t.foundUnderspecifiedOpEh(n)
	}
	s, err := t.mkBinaryOp(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	x := smtArithExprs{m: t.state.manager}
	defer func() { err = errors.Join(err, x.release()) }()
	args, err := t.args(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	mod, err := t.mkApp(&x, smtArithOpMod, args[0], args[1])
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if err = t.state.Internalize(mod, false); err != nil {
		return SMTNullTheoryVar, err
	}
	if t.relevancy() {
		if err = t.state.relevancy.AddDependency(n, mod); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	return s, nil
}

func (t *SMTTheoryArith) internalizeMod(n ast.ASTHandle) (SMTTheoryVar, error) {
	under, err := t.divisorUnderspecified(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if under {
		t.foundUnderspecifiedOpEh(n)
	}
	s, err := t.mkBinaryOp(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if !t.relevancy() {
		args, err := t.args(n)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		return s, t.mkIdivModAxioms(args[0], args[1])
	}
	return s, nil
}

func (t *SMTTheoryArith) internalizeRem(n ast.ASTHandle) (SMTTheoryVar, error) {
	under, err := t.divisorUnderspecified(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if under {
		t.foundUnderspecifiedOpEh(n)
	}
	s, err := t.mkBinaryOp(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if !t.relevancy() {
		args, err := t.args(n)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		return s, t.mkRemAxiom(args[0], args[1])
	}
	return s, nil
}

// ---- core.h:445-492 ----

// rewrite is th_rewriter s(e, result).
func (t *SMTTheoryArith) rewrite(x *smtArithExprs, e ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.state.manager
	if t.state.theoryRewriter == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	ref, err := m.NewRef(e)
	if err != nil {
		return e, err
	}
	x.refs = append(x.refs, ref)
	if err = t.state.theoryRewriter.Rewrite(e, ref); err != nil {
		return e, err
	}
	return m.RefGet(ref)
}

// cancelFlag is context::get_cancel_flag(): !m_manager.limit().inc(), which
// counts a step and is set by cancellation or an exhausted resource limit.
func (t *SMTTheoryArith) cancelFlag() bool {
	canceled, err := t.state.manager.Canceled()
	return err != nil || canceled
}

func (t *SMTTheoryArith) mkAxiom(ante, conseq ast.ASTHandle, simplifyConseq bool) (err error) {
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	sAnte, err := t.rewrite(&x, ante)
	if err != nil {
		return err
	}
	if t.cancelFlag() {
		return nil
	}
	negated, sAnteN, err := m.IsNot(sAnte)
	if err != nil {
		return err
	}
	if negated {
		sAnte = sAnteN
	}
	if err = t.state.Internalize(sAnte, false); err != nil {
		return err
	}
	lAnte, err := t.state.Literal(sAnte)
	if err != nil {
		return err
	}
	if negated {
		lAnte = lAnte.Negated()
	}
	sConseq := conseq
	if simplifyConseq {
		if sConseq, err = t.rewrite(&x, conseq); err != nil {
			return err
		}
	}
	if t.cancelFlag() {
		return nil
	}
	negated, sConseqN, err := m.IsNot(sConseq)
	if err != nil {
		return err
	}
	if negated {
		sConseq = sConseqN
	}
	if err = t.state.Internalize(sConseq, false); err != nil {
		return err
	}
	lConseq, err := t.state.Literal(sConseq)
	if err != nil {
		return err
	}
	if negated {
		lConseq = lConseq.Negated()
	}
	if err = t.mkClause2(lAnte, lConseq, nil); err != nil {
		return err
	}
	if t.relevancy() {
		if lAnte == SMTFalseLiteral {
			return t.markLiteralAsRelevant(lConseq)
		}
		// The antecedent must be relevant, otherwise the core will not
		// propagate it to the theory of arithmetic.
		if err = t.markLiteralAsRelevant(lAnte); err != nil {
			return err
		}
		// mark consequent as relevant if antecedent is false.
		return t.addRelWatch(lAnte.Negated(), sConseq)
	}
	return nil
}

// mark_as_relevant(literal).
func (t *SMTTheoryArith) markLiteralAsRelevant(l SMTLiteral) error {
	if l.Var() == SMTTrueBoolVar {
		return nil
	}
	e, err := t.state.BoolExpression(l.Var())
	if err != nil {
		return err
	}
	return t.state.markRelevant(e)
}

// add_rel_watch(l, n): mark n relevant when l becomes true.
func (t *SMTTheoryArith) addRelWatch(l SMTLiteral, n ast.ASTHandle) error {
	e, err := t.state.BoolExpression(l.Var())
	if err != nil {
		return err
	}
	return t.state.relevancy.AddWatchTarget(e, !l.Sign(), n)
}

// ---- core.h:494-616 ----

func (t *SMTTheoryArith) isZero(n ast.ASTHandle) bool {
	r, ok := t.isNumeral(n)
	return ok && r.Sign() == 0
}

func (t *SMTTheoryArith) mkDivAxiom(p, q ast.ASTHandle) (err error) {
	if t.isZero(q) {
		return nil
	}
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	div, err := t.mkApp(&x, smtArithOpDiv, p, q)
	if err != nil {
		return err
	}
	zero, err := t.mkNumeral(&x, new(big.Rat), false)
	if err != nil {
		return err
	}
	eqz, err := x.hold(m.MkEq(q, zero))
	if err != nil {
		return err
	}
	qdiv, err := t.mkApp(&x, smtArithOpMul, q, div)
	if err != nil {
		return err
	}
	eq, err := x.hold(m.MkEq(qdiv, p))
	if err != nil {
		return err
	}
	return t.mkAxiom(eqz, eq, true)
}

func (t *SMTTheoryArith) mkIdivModAxioms(dividend, divisor ast.ASTHandle) (err error) {
	if t.isZero(divisor) {
		return nil
	}
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	_, isNumeral := t.isNumeral(divisor)
	div, err := t.mkApp(&x, smtArithOpIDiv, dividend, divisor)
	if err != nil {
		return err
	}
	mod, err := t.mkApp(&x, smtArithOpMod, dividend, divisor)
	if err != nil {
		return err
	}
	zero, err := t.mkNumeral(&x, new(big.Rat), true)
	if err != nil {
		return err
	}
	one, err := t.mkNumeral(&x, big.NewRat(1, 1), true)
	if err != nil {
		return err
	}
	lt, err := t.mkApp(&x, smtArithOpLT, divisor, zero)
	if err != nil {
		return err
	}
	negDivisor, err := t.mkApp(&x, smtArithOpSub, zero, divisor)
	if err != nil {
		return err
	}
	ite, err := x.hold(m.MkIte(lt, negDivisor, divisor))
	if err != nil {
		return err
	}
	absDivisor, err := t.mkApp(&x, smtArithOpSub, ite, one)
	if err != nil {
		return err
	}
	if absDivisor, err = t.rewrite(&x, absDivisor); err != nil {
		return err
	}
	eqz, err := x.hold(m.MkEq(divisor, zero))
	if err != nil {
		return err
	}
	prod, err := t.mkApp(&x, smtArithOpMul, divisor, div)
	if err != nil {
		return err
	}
	sum, err := t.mkApp(&x, smtArithOpAdd, prod, mod)
	if err != nil {
		return err
	}
	eq, err := x.hold(m.MkEq(sum, dividend))
	if err != nil {
		return err
	}
	lowerE, err := t.mkApp(&x, smtArithOpGE, mod, zero)
	if err != nil {
		return err
	}
	upperE, err := t.mkApp(&x, smtArithOpLE, mod, absDivisor)
	if err != nil {
		return err
	}
	for _, c := range []ast.ASTHandle{eq, lowerE, upperE} {
		if err = t.mkAxiom(eqz, c, !isNumeral); err != nil {
			return err
		}
	}
	if k, ok := t.isNumeral(divisor); t.params.EnumConstMod && ok && k.Sign() > 0 && k.Cmp(big.NewRat(8, 1)) < 0 {
		var lits []SMTLiteral
		for j := new(big.Rat); j.Cmp(k) < 0; j.Add(j, big.NewRat(1, 1)) {
			num, err := t.mkNumeral(&x, j, true)
			if err != nil {
				return err
			}
			modJ, err := x.hold(m.MkEq(mod, num))
			if err != nil {
				return err
			}
			if err = t.state.Internalize(modJ, false); err != nil {
				return err
			}
			lit, err := t.state.Literal(modJ)
			if err != nil {
				return err
			}
			lits = append(lits, lit)
			if err = t.markLiteralAsRelevant(lit); err != nil {
				return err
			}
		}
		return t.state.addTheoryAxiom(t.FamilyID(), lits)
	}
	return nil
}

func (t *SMTTheoryArith) mkRemAxiom(dividend, divisor ast.ASTHandle) (err error) {
	// if divisor is zero, then rem is an uninterpreted function.
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	zero, err := t.mkNumeral(&x, new(big.Rat), true)
	if err != nil {
		return err
	}
	rem, err := t.mkApp(&x, smtArithOpRem, dividend, divisor)
	if err != nil {
		return err
	}
	mod, err := t.mkApp(&x, smtArithOpMod, dividend, divisor)
	if err != nil {
		return err
	}
	dltz, err := t.mkApp(&x, smtArithOpLT, divisor, zero)
	if err != nil {
		return err
	}
	eq1, err := x.hold(m.MkEq(rem, mod))
	if err != nil {
		return err
	}
	negMod, err := t.mkApp(&x, smtArithOpSub, zero, mod)
	if err != nil {
		return err
	}
	eq2, err := x.hold(m.MkEq(rem, negMod))
	if err != nil {
		return err
	}
	// n < 0 || rem(a,n) = mod(a, n)
	if err = t.mkAxiom(dltz, eq1, true); err != nil {
		return err
	}
	notDltz, err := x.hold(m.MkNot(dltz))
	if err != nil {
		return err
	}
	// !n < 0 || rem(a,n) = -mod(a, n)
	return t.mkAxiom(notDltz, eq2, true)
}

// mkToIntAxiom creates s := x - to_real(to_int(x)) with 0 <= s < 1.
func (t *SMTTheoryArith) mkToIntAxiom(n ast.ASTHandle) (err error) {
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	arg, err := m.GetArg(n, 0)
	if err != nil {
		return err
	}
	falseE, err := m.MkFalse()
	if err != nil {
		return err
	}
	// to_int (to_real x) = x
	if t.isOp(arg, smtArithOpToReal) {
		inner, err := m.GetArg(arg, 0)
		if err != nil {
			return err
		}
		eq, err := x.hold(m.MkEq(inner, n))
		if err != nil {
			return err
		}
		return t.mkAxiom(falseE, eq, true)
	}
	toR, err := t.mkApp(&x, smtArithOpToReal, n)
	if err != nil {
		return err
	}
	minusOne, err := t.mkNumeral(&x, big.NewRat(-1, 1), false)
	if err != nil {
		return err
	}
	neg, err := t.mkApp(&x, smtArithOpMul, minusOne, toR)
	if err != nil {
		return err
	}
	diff, err := t.mkApp(&x, smtArithOpAdd, arg, neg)
	if err != nil {
		return err
	}
	zero, err := t.mkNumeral(&x, new(big.Rat), false)
	if err != nil {
		return err
	}
	one, err := t.mkNumeral(&x, big.NewRat(1, 1), false)
	if err != nil {
		return err
	}
	lo, err := t.mkApp(&x, smtArithOpGE, diff, zero)
	if err != nil {
		return err
	}
	hi, err := t.mkApp(&x, smtArithOpGE, diff, one)
	if err != nil {
		return err
	}
	if hi, err = x.hold(m.MkNot(hi)); err != nil {
		return err
	}
	if err = t.mkAxiom(falseE, lo, false); err != nil {
		return err
	}
	return t.mkAxiom(falseE, hi, false)
}

// ---- core.h:618-704 ----

func (t *SMTTheoryArith) internalizeToInt(n ast.ASTHandle) (SMTTheoryVar, error) {
	if t.state.context.enodes.get(n) != nil {
		return t.expr2var(n), nil
	}
	arg, err := t.state.manager.GetArg(n, 0)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if _, err = t.internalizeTermCore(arg); err != nil {
		return SMTNullTheoryVar, err
	}
	e, err := t.mkEnode(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	r, err := t.mkVar(e)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if !t.relevancy() {
		return r, t.mkToIntAxiom(n)
	}
	return r, nil
}

// mkIsIntAxiom creates (iff (is_int x) (= x (to_real (to_int x)))).
func (t *SMTTheoryArith) mkIsIntAxiom(n ast.ASTHandle) (err error) {
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	arg, err := m.GetArg(n, 0)
	if err != nil {
		return err
	}
	toInt, err := t.mkApp(&x, smtArithOpToInt, arg)
	if err != nil {
		return err
	}
	toReal, err := t.mkApp(&x, smtArithOpToReal, toInt)
	if err != nil {
		return err
	}
	eq, err := x.hold(m.MkEq(toReal, arg))
	if err != nil {
		return err
	}
	notN, err := x.hold(m.MkNot(n))
	if err != nil {
		return err
	}
	if err = t.mkAxiom(notN, eq, true); err != nil {
		return err
	}
	notEq, err := x.hold(m.MkNot(eq))
	if err != nil {
		return err
	}
	return t.mkAxiom(notEq, n, true)
}

func (t *SMTTheoryArith) internalizeIsInt(n ast.ASTHandle) error {
	if _, found := t.state.BoolVar(n); found {
		return nil
	}
	arg, err := t.state.manager.GetArg(n, 0)
	if err != nil {
		return err
	}
	if _, err = t.internalizeTermCore(arg); err != nil {
		return err
	}
	e, err := t.mkEnode(n)
	if err != nil {
		return err
	}
	if _, err = t.mkVar(e); err != nil {
		return err
	}
	if !t.relevancy() {
		return t.mkIsIntAxiom(n)
	}
	return nil
}

// internalizeToReal creates the row r - arg = 0.
func (t *SMTTheoryArith) internalizeToReal(n ast.ASTHandle) (SMTTheoryVar, error) {
	if t.state.context.enodes.get(n) != nil {
		return t.expr2var(n), nil
	}
	argE, err := t.state.manager.GetArg(n, 0)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	arg, err := t.internalizeTermCore(argE)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	// n may be internalized by the call above if n is (to_real (to_int t)).
	if t.state.context.enodes.get(n) != nil {
		return t.expr2var(n), nil
	}
	e, err := t.mkEnode(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	r, err := t.mkVar(e)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	rID := t.mkRow()
	t.pushRowVars()
	defer t.popRowVars()
	one := big.NewRat(1, 1)
	if err = t.addRowEntry(true, rID, one, arg); err != nil {
		return SMTNullTheoryVar, err
	}
	if err = t.addRowEntry(false, rID, one, r); err != nil {
		return SMTNullTheoryVar, err
	}
	return r, t.initRow(rID)
}

func (t *SMTTheoryArith) internalizeNumeral(n ast.ASTHandle) (SMTTheoryVar, error) {
	val, ok := t.isNumeral(n)
	if !ok {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	e, err := t.mkEnode(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	v, err := t.mkVar(e)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	var ival smtArithmeticValue
	ival.setRational(val)
	l := &smtArithmeticBound{variable: v, kind: smtArithmeticLowerBound}
	l.value.set(&ival)
	u := &smtArithmeticBound{variable: v, kind: smtArithmeticUpperBound}
	u.value.set(&ival)
	if err = t.setBound(l, false); err != nil {
		return SMTNullTheoryVar, err
	}
	if err = t.setBound(u, true); err != nil {
		return SMTNullTheoryVar, err
	}
	t.boundsToDelete = append(t.boundsToDelete, l, u)
	t.value[v].set(&ival)
	return v, nil
}

// ---- core.h:725-790 ----

// internalizeTermCore internalizes n and returns its alias, or
// null_theory_var if the theory does not implement the term.
func (t *SMTTheoryArith) internalizeTermCore(n ast.ASTHandle) (SMTTheoryVar, error) {
	if e := t.state.context.enodes.get(n); e != nil && t.IsAttachedToVariable(e) {
		return e.TheoryVar(t.FamilyID()), nil
	}
	op, own := t.arithOp(n)
	if own {
		switch op {
		case smtArithOpAdd:
			return t.internalizeAdd(n)
		case smtArithOpMul:
			return t.internalizeMul(n)
		case smtArithOpDiv:
			return t.internalizeDiv(n)
		case smtArithOpIDiv:
			return t.internalizeIdiv(n)
		case smtArithOpMod:
			return t.internalizeMod(n)
		case smtArithOpRem:
			return t.internalizeRem(n)
		case smtArithOpToReal:
			return t.internalizeToReal(n)
		case smtArithOpToInt:
			return t.internalizeToInt(n)
		case smtArithOpNum:
			return t.internalizeNumeral(n)
		case smtArithOpPower:
			t.foundUnsupportedOpEh(n) // unsupported
			return t.mkBinaryOp(n)
		case smtArithOpIrrationalAlgebraicNum:
			t.foundUnsupportedOpEh(n) // unsupported
			e, err := t.mkEnode(n)
			if err != nil {
				return SMTNullTheoryVar, err
			}
			return t.mkVar(e)
		}
		t.foundUnsupportedOpEh(n)
		if t.state.context.enodes.get(n) != nil {
			return t.expr2var(n), nil
		}
		args, err := t.args(n)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		for _, arg := range args {
			if err = t.state.Internalize(arg, false); err != nil {
				return SMTNullTheoryVar, err
			}
		}
		e, err := t.mkEnode(n)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		return t.mkVar(e)
	}
	if t.state.context.enodes.get(n) == nil {
		if err := t.state.Internalize(n, false); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	e := t.state.context.enodes.get(n)
	if e == nil {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	if !t.IsAttachedToVariable(e) {
		return t.mkVar(e)
	}
	return e.TheoryVar(t.FamilyID()), nil
}

// ---- core.h:792-945 ----

// mkRow creates a new empty row and returns its id.
func (t *SMTTheoryArith) mkRow() int {
	var r int
	if n := len(t.deadRows); n == 0 {
		r = len(t.rows)
		t.rows = append(t.rows, newSMTArithmeticRow())
	} else {
		r = t.deadRows[n-1]
		t.deadRows = t.deadRows[:n-1]
	}
	t.inToCheck.AssureDomain(uint32(r))
	return r
}

// initRow initializes a new row; its last monomial, with coefficient 1,
// owns the row.
func (t *SMTTheoryArith) initRow(rID int) error {
	r := &t.rows[rID]
	if r.firstFree != -1 || r.size == 0 || r.size != len(r.entries) || !smtRatIsOne(&r.entries[r.size-1].coefficient) {
		return ast.ErrASTDomain
	}
	s := r.entries[r.size-1].variable
	r.base = s
	t.setVarRow(s, rID)
	if t.lazyPivotingLvl() > 2 {
		t.setVarKind(s, smtArithmeticQuasiBase)
		if err := t.normalizeQuasiBaseRow(rID); err != nil {
			return err
		}
	} else if err := t.normalizeBaseRow(rID); err != nil {
		return err
	}
	if t.propagationMode() != smtArithBPNone {
		t.markRowForBoundProp(rID)
	}
	return nil
}

// collectVars collects the variables of kind k in the row, other than the
// row owner, with negated coefficients.
func (t *SMTTheoryArith) collectVars(rID int, k smtArithmeticVariableKind) []smtArithLinearMonomial {
	r := &t.rows[rID]
	base := r.base
	var result []smtArithLinearMonomial
	for i := range r.entries {
		it := &r.entries[i]
		if !it.dead() && t.getVarKind(it.variable) == k && it.variable != base {
			var m smtArithLinearMonomial
			m.coeff.Neg(&it.coefficient)
			m.v = it.variable
			result = append(result, m)
		}
	}
	return result
}

func (t *SMTTheoryArith) normalizeQuasiBaseRow(rID int) error {
	return t.addRows(rID, t.collectVars(rID, smtArithmeticQuasiBase))
}

func (t *SMTTheoryArith) quasiBaseRow2BaseRow(rID int) error {
	if err := t.addRows(rID, t.collectVars(rID, smtArithmeticBase)); err != nil {
		return err
	}
	s := t.rows[rID].base
	t.setVarKind(s, smtArithmeticBase)
	var tmp smtArithmeticValue
	if t.getImpliedOldValue(s, &tmp) {
		// restore_assignment restores the old values of the variables the
		// row depends on; s must stay compatible with them.
		t.value[s].set(&tmp)
		t.saveValue(s)
	}
	t.value[s].set(t.getImpliedValue(s))
	return nil
}

func (t *SMTTheoryArith) normalizeBaseRow(rID int) error {
	if t.lazyPivotingLvl() > 0 {
		if err := t.normalizeQuasiBaseRow(rID); err != nil {
			return err
		}
	}
	return t.quasiBaseRow2BaseRow(rID)
}

// ---- core.h:946-1205 ----

// farkasOneParameters are the parameters { farkas, 1, 1 } of mk_bound_axiom,
// created only when proofs are enabled.
func (t *SMTTheoryArith) farkasOneParameters(n int) ([]ast.ASTParameter, error) {
	if !t.proofsEnabled() {
		return nil, nil
	}
	ante := smtArithmeticAntecedents{}
	for i := 0; i < n; i++ {
		ante.literalCoefficients = append(ante.literalCoefficients, big.NewRat(1, 1))
	}
	return ante.parameters(t.state.manager, "farkas")
}

func (t *SMTTheoryArith) mkClause2(l1, l2 SMTLiteral, params []ast.ASTParameter) error {
	return t.state.addTheoryAxiomWithParameters(t.FamilyID(), []SMTLiteral{l1, l2}, params)
}

func (t *SMTTheoryArith) mkClause3(l1, l2, l3 SMTLiteral, params []ast.ASTParameter) error {
	return t.state.addTheoryAxiomWithParameters(t.FamilyID(), []SMTLiteral{l1, l2, l3}, params)
}

func (t *SMTTheoryArith) mkBoundAxioms(a1 *smtArithmeticAtom) error {
	v := a1.variable
	occs := t.varOccs[v]
	if !t.state.searching {
		// The user is assumed to call propagation before internal scopes are
		// pushed, which flushes newly asserted atoms into the right context.
		t.newAtoms = append(t.newAtoms, a1)
		return nil
	}
	k1, kind1 := &a1.threshold, a1.atomKind
	end := len(occs)
	loInf, loSup, hiInf, hiSup := end, end, end, end
	for i, a2 := range occs {
		k2, kind2 := &a2.threshold, a2.atomKind
		if k1.cmp(k2) == 0 && kind1 == kind2 {
			continue
		}
		if kind2 == smtArithmeticLowerAtom {
			if k2.cmp(k1) < 0 {
				if loInf == end || k2.cmp(&occs[loInf].threshold) > 0 {
					loInf = i
				}
			} else if loSup == end || k2.cmp(&occs[loSup].threshold) < 0 {
				loSup = i
			}
		} else if k2.cmp(k1) < 0 {
			if hiInf == end || k2.cmp(&occs[hiInf].threshold) > 0 {
				hiInf = i
			}
		} else if hiSup == end || k2.cmp(&occs[hiSup].threshold) < 0 {
			hiSup = i
		}
	}
	for _, i := range []int{loInf, loSup, hiInf, hiSup} {
		if i != end {
			if err := t.mkBoundAxiom(a1, occs[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTTheoryArith) mkBoundAxiom(a1, a2 *smtArithmeticAtom) (err error) {
	v := a1.variable
	l1, l2 := NewSMTLiteral(a1.boolean, false), NewSMTLiteral(a2.boolean, false)
	k1, k2 := &a1.threshold, &a2.threshold
	kind1, kind2 := a1.atomKind, a2.atomKind
	vIsInt := t.isInt(v)
	if k1.cmp(k2) == 0 && kind1 == kind2 {
		return nil
	}
	coeffs, err := t.farkasOneParameters(2)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, smtArithmeticReleaseParameters(t.state.manager, coeffs)) }()
	var one smtArithmeticValue
	one.rational.SetInt64(1)
	if kind1 == smtArithmeticLowerAtom {
		if kind2 == smtArithmeticLowerAtom {
			if k2.cmp(k1) <= 0 {
				return t.mkClause2(l1.Negated(), l2, coeffs)
			}
			return t.mkClause2(l1, l2.Negated(), coeffs)
		}
		if k1.cmp(k2) <= 0 {
			// k1 <= k2, k1 <= x or x <= k2
			return t.mkClause2(l1, l2, coeffs)
		}
		// k1 > hi_inf, k1 <= x => ~(x <= hi_inf)
		if err = t.mkClause2(l1.Negated(), l2.Negated(), coeffs); err != nil {
			return err
		}
		var k2p1 smtArithmeticValue
		k2p1.set(k2)
		k2p1.add(&one)
		if vIsInt && k1.cmp(&k2p1) == 0 {
			// k1 <= x or x <= k1-1
			return t.mkClause2(l1, l2, coeffs)
		}
		return nil
	}
	if kind2 == smtArithmeticLowerAtom {
		if k1.cmp(k2) >= 0 {
			// k1 >= lo_inf, k1 >= x or lo_inf <= x
			return t.mkClause2(l1, l2, coeffs)
		}
		// k1 < k2, k2 <= x => ~(x <= k1)
		if err = t.mkClause2(l1.Negated(), l2.Negated(), coeffs); err != nil {
			return err
		}
		var k2m1 smtArithmeticValue
		k2m1.set(k2)
		k2m1.sub(&one)
		if vIsInt && k1.cmp(&k2m1) == 0 {
			// x <= k1 or k1+l <= x
			return t.mkClause2(l1, l2, coeffs)
		}
		return nil
	}
	// kind1 == A_UPPER, kind2 == A_UPPER
	if k1.cmp(k2) >= 0 {
		// k1 >= k2, x <= k2 => x <= k1
		return t.mkClause2(l1, l2.Negated(), coeffs)
	}
	// k1 <= hi_sup , x <= k1 =>  x <= hi_sup
	return t.mkClause2(l1.Negated(), l2, coeffs)
}

// flushBoundAxioms. The source sorts with std::sort, which is not stable;
// sort.SliceStable keeps atoms with equal thresholds in creation order.
func (t *SMTTheoryArith) flushBoundAxioms() error {
	for len(t.newAtoms) > 0 {
		last := len(t.newAtoms) - 1
		atoms := []*smtArithmeticAtom{t.newAtoms[last]}
		t.newAtoms = t.newAtoms[:last]
		v := atoms[0].variable
		for i := 0; i < len(t.newAtoms); i++ {
			if t.newAtoms[i].variable == v {
				atoms = append(atoms, t.newAtoms[i])
				t.newAtoms[i] = t.newAtoms[len(t.newAtoms)-1]
				t.newAtoms = t.newAtoms[:len(t.newAtoms)-1]
				i--
			}
		}
		occs := append([]*smtArithmeticAtom(nil), t.varOccs[v]...)
		sort.SliceStable(atoms, func(i, j int) bool { return atoms[i].threshold.cmp(&atoms[j].threshold) < 0 })
		sort.SliceStable(occs, func(i, j int) bool { return occs[i].threshold.cmp(&occs[j].threshold) < 0 })
		end := len(occs)
		begin1 := t.first(smtArithmeticLowerAtom, occs, 0)
		begin2 := t.first(smtArithmeticUpperAtom, occs, 0)
		loInf, loSup := begin1, begin1
		hiInf, hiSup := begin2, begin2
		visited := make(map[*smtArithmeticAtom]bool)
		for _, a1 := range atoms {
			loInf1, floInf := t.nextInf(a1, smtArithmeticLowerAtom, occs, loInf)
			hiInf1, fhiInf := t.nextInf(a1, smtArithmeticUpperAtom, occs, hiInf)
			loSup1, floSup := t.nextSup(a1, smtArithmeticLowerAtom, occs, loSup)
			hiSup1, fhiSup := t.nextSup(a1, smtArithmeticUpperAtom, occs, hiSup)
			if loInf1 != end {
				loInf = loInf1
			}
			if loSup1 != end {
				loSup = loSup1
			}
			if hiInf1 != end {
				hiInf = hiInf1
			}
			if hiSup1 != end {
				hiSup = hiSup1
			}
			if !floInf {
				loInf = end
			}
			if !fhiInf {
				hiInf = end
			}
			if !floSup {
				loSup = end
			}
			if !fhiSup {
				hiSup = end
			}
			visited[a1] = true
			if loInf1 != end && loInf != end && !visited[occs[loInf]] {
				if err := t.mkBoundAxiom(a1, occs[loInf]); err != nil {
					return err
				}
			}
			if loSup1 != end && loSup != end && !visited[occs[loSup]] {
				if err := t.mkBoundAxiom(a1, occs[loSup]); err != nil {
					return err
				}
			}
			if hiInf1 != end && hiInf != end && !visited[occs[hiInf]] {
				if err := t.mkBoundAxiom(a1, occs[hiInf]); err != nil {
					return err
				}
			}
			if hiSup1 != end && hiSup != end && !visited[occs[hiSup]] {
				if err := t.mkBoundAxiom(a1, occs[hiSup]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *SMTTheoryArith) first(kind smtArithmeticAtomKind, occs []*smtArithmeticAtom, it int) int {
	for ; it < len(occs); it++ {
		if occs[it].atomKind == kind {
			return it
		}
	}
	return len(occs)
}

func (t *SMTTheoryArith) nextInf(a1 *smtArithmeticAtom, kind smtArithmeticAtomKind, occs []*smtArithmeticAtom, it int) (int, bool) {
	k1 := &a1.threshold
	result := len(occs)
	foundCompatible := false
	for ; it < len(occs); it++ {
		a2 := occs[it]
		if a1 == a2 || a2.atomKind != kind {
			continue
		}
		foundCompatible = true
		if a2.threshold.cmp(k1) <= 0 {
			result = it
		} else {
			break
		}
	}
	return result, foundCompatible
}

func (t *SMTTheoryArith) nextSup(a1 *smtArithmeticAtom, kind smtArithmeticAtomKind, occs []*smtArithmeticAtom, it int) (int, bool) {
	k1 := &a1.threshold
	foundCompatible := false
	for ; it < len(occs); it++ {
		a2 := occs[it]
		if a1 == a2 || a2.atomKind != kind {
			continue
		}
		foundCompatible = true
		if k1.cmp(&a2.threshold) < 0 {
			return it, true
		}
	}
	return len(occs), foundCompatible
}

// isEqApp is m.is_eq(atom, lhs, rhs).
func (t *SMTTheoryArith) isEqApp(e ast.ASTHandle) (bool, ast.ASTHandle, ast.ASTHandle, error) {
	m := t.state.manager
	if !t.isApp(e) {
		return false, ast.ASTHandle{}, ast.ASTHandle{}, nil
	}
	family, err := m.GetFamilyID(e)
	if err != nil || family != m.BasicFamilyID() {
		return false, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	op, err := m.GetDeclKind(e)
	if err != nil || op != ast.ASTOpEq {
		return false, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	args, err := t.args(e)
	if err != nil || len(args) != 2 {
		return false, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	return true, args[0], args[1], nil
}

// ---- core.h:1208-1318 ----

var errSMTArithMalformedAtom = errors.New("smt: malformed atomic constraint")

func (t *SMTTheoryArith) InternalizeAtom(n ast.ASTHandle, gateCtx bool) (bool, error) {
	if t.isOp(n, smtArithOpIsInt) {
		if err := t.internalizeIsInt(n); err != nil {
			return false, err
		}
		if _, found := t.state.BoolVar(n); found {
			return true, nil
		}
		bv, err := t.state.MkBoolVar(n)
		if err != nil {
			return false, err
		}
		return true, t.state.SetVariableTheory(bv, t.FamilyID())
	}
	var kind smtArithmeticAtomKind
	if t.isOp(n, smtArithOpLE) {
		kind = smtArithmeticUpperAtom
	} else if t.isOp(n, smtArithOpGE) {
		kind = smtArithmeticLowerAtom
	} else {
		return false, ast.ErrASTDomain
	}
	args, err := t.args(n)
	if err != nil {
		return false, err
	}
	if !t.isApp(args[0]) || !t.isApp(args[1]) {
		return false, nil
	}
	lhs, rhs := args[0], args[1]
	if t.isOp(rhs, smtArithOpToReal) {
		if rhs2, err := t.state.manager.GetArg(rhs, 0); err == nil && t.isApp(rhs2) {
			rhs = rhs2
		}
	}
	k0, ok := t.isNumeral(rhs)
	if !ok {
		return false, errSMTArithMalformedAtom
	}
	v, err := t.internalizeTermCore(lhs)
	if err != nil {
		return false, err
	}
	if v == SMTNullTheoryVar {
		return false, nil
	}
	if _, found := t.state.BoolVar(n); found {
		return true, nil // term was re-internalized
	}
	bv, err := t.state.MkBoolVar(n)
	if err != nil {
		return false, err
	}
	if err = t.state.SetVariableTheory(bv, t.FamilyID()); err != nil {
		return false, err
	}
	if t.isInt(v) && !k0.IsInt() {
		if kind == smtArithmeticUpperAtom {
			k0.SetInt(smtRatFloor(k0))
		} else {
			k0.SetInt(smtRatFloor(k0))
			k0.Add(k0, big.NewRat(1, 1)) // ceil of a non-integer
		}
	}
	var k smtArithmeticValue
	k.setRational(k0)
	a, err := newSMTArithmeticAtom(bv, v, &k, kind)
	if err != nil {
		return false, err
	}
	if err = t.mkBoundAxioms(a); err != nil {
		return false, err
	}
	t.unassignedAtoms[v]++
	t.varOccs[v] = append(t.varOccs[v], a)
	t.atoms = append(t.atoms, a)
	t.insertBv2a(bv, a)
	return true, nil
}

func (t *SMTTheoryArith) InternalizeTerm(term ast.ASTHandle) (bool, error) {
	v, err := t.internalizeTermCore(term)
	return v != SMTNullTheoryVar, err
}

func (t *SMTTheoryArith) InternalizeEquality(atom ast.ASTHandle, v int32) error {
	isEq, lhs, rhs, err := t.isEqApp(atom)
	if err != nil || !t.params.EagerEqAxioms || !isEq || !t.isApp(lhs) || !t.isApp(rhs) {
		return err
	}
	n1 := t.state.context.enodes.get(lhs)
	n2 := t.state.context.enodes.get(rhs)
	// The atom may be a theory axiom that is not in simplified form, such
	// as (= a a); mk_axioms expects n1 != n2.
	if n1 != nil && n2 != nil && n1.TheoryVar(t.FamilyID()) != SMTNullTheoryVar &&
		n2.TheoryVar(t.FamilyID()) != SMTNullTheoryVar && n1 != n2 {
		return t.arithEqAdapter.MkAxioms(n1.TheoryVar(t.FamilyID()), n2.TheoryVar(t.FamilyID()))
	}
	return nil
}

func (t *SMTTheoryArith) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error { return nil } // do nothing

func (t *SMTTheoryArith) Assign(v int32, isTrue bool) error {
	a := t.getBv2a(v)
	if a == nil {
		return nil
	}
	if err := a.assign(isTrue, t.getEpsilon(a.variable)); err != nil {
		return err
	}
	t.assertedBounds = append(t.assertedBounds, a)
	return nil
}

// ---- core.h:1320-1418 ----

func (t *SMTTheoryArith) Relevant(n ast.ASTHandle) error {
	op, own := t.arithOp(n)
	if !own {
		return nil
	}
	args, err := t.args(n)
	if err != nil {
		return err
	}
	switch op {
	case smtArithOpMod:
		return t.mkIdivModAxioms(args[0], args[1])
	case smtArithOpRem:
		return t.mkRemAxiom(args[0], args[1])
	case smtArithOpDiv:
		return t.mkDivAxiom(args[0], args[1])
	case smtArithOpToInt:
		return t.mkToIntAxiom(n)
	case smtArithOpIsInt:
		return t.mkIsIntAxiom(n)
	}
	return nil
}

func (t *SMTTheoryArith) NewEquality(v1, v2 SMTTheoryVar) (err error) {
	n1, err := t.Enode(v1)
	if err != nil {
		return err
	}
	isInt, err := t.isIntSort(n1.owner)
	if err != nil {
		return err
	}
	isReal, err := t.isRealSort(n1.owner)
	if err != nil || (!isInt && !isReal) {
		return err
	}
	if !t.params.EqBounds {
		return t.arithEqAdapter.NewEquality(v1, v2)
	}
	n2, err := t.Enode(v2)
	if err != nil {
		return err
	}
	if _, ok := t.isNumeral(n1.owner); ok {
		v1, v2 = v2, v1
		n1, n2 = n2, n1
	}
	var b1, b2 *smtArithmeticEqualityBound
	if k, ok := t.isNumeral(n2.owner); ok {
		var val smtArithmeticValue
		val.setRational(k)
		if b1, err = newSMTArithmeticEqualityBound(v1, &val, smtArithmeticLowerBound, n1, n2); err != nil {
			return err
		}
		if b2, err = newSMTArithmeticEqualityBound(v1, &val, smtArithmeticUpperBound, n1, n2); err != nil {
			return err
		}
	} else {
		id1, err := n1.OwnerID()
		if err != nil {
			return err
		}
		id2, err := n2.OwnerID()
		if err != nil {
			return err
		}
		if id1 > id2 {
			n1, n2 = n2, n1
		}
		m := t.state.manager
		x := smtArithExprs{m: m}
		defer func() { err = errors.Join(err, x.release()) }()
		isIntN1, err := t.isIntSort(n1.owner)
		if err != nil {
			return err
		}
		minusOne, err := t.mkNumeral(&x, big.NewRat(-1, 1), isIntN1)
		if err != nil {
			return err
		}
		prod, err := t.mkApp(&x, smtArithOpMul, minusOne, n2.owner)
		if err != nil {
			return err
		}
		s, err := t.mkApp(&x, smtArithOpAdd, n1.owner, prod)
		if err != nil {
			return err
		}
		if err = t.state.Internalize(s, false); err != nil {
			return err
		}
		eS := t.state.context.enodes.get(s)
		if err = t.state.markRelevant(s); err != nil {
			return err
		}
		vS := eS.TheoryVar(t.FamilyID())
		var zero smtArithmeticValue
		if b1, err = newSMTArithmeticEqualityBound(vS, &zero, smtArithmeticLowerBound, n1, n2); err != nil {
			return err
		}
		if b2, err = newSMTArithmeticEqualityBound(vS, &zero, smtArithmeticUpperBound, n1, n2); err != nil {
			return err
		}
	}
	t.boundsToDelete = append(t.boundsToDelete, b1, b2)
	t.assertedBounds = append(t.assertedBounds, b1, b2)
	return nil
}

func (t *SMTTheoryArith) UseDisequalities() bool { return true }

func (t *SMTTheoryArith) NewDisequality(v1, v2 SMTTheoryVar) error {
	t.stats.assertDiseq++
	return t.arithEqAdapter.NewDisequality(v1, v2)
}

func (t *SMTTheoryArith) Restart() error { return t.arithEqAdapter.Restart() }

func (t *SMTTheoryArith) InitializeSearch() error {
	t.numConflicts = 0
	t.branchCutCounter = 0
	t.eagerGCD = t.params.EagerGCD
	if t.lazyPivotingLvl() == 1 {
		if err := t.elimQuasiBaseRows(); err != nil {
			return err
		}
	}
	if err := t.moveUnconstrainedToBase(); err != nil {
		return err
	}
	if err := t.arithEqAdapter.InitializeSearch(); err != nil {
		return err
	}
	t.finalCheckIdx = 0
	t.nlGbExhausted = false
	t.nlStrategyIdx = 0
	return nil
}

// ---- core.h:1420-1495 ----

func (t *SMTTheoryArith) finalCheckCore() (SMTFinalCheckStatus, error) {
	t.modelDependsOnComputedEpsilon = false
	oldIdx := t.finalCheckIdx
	result := SMTFinalCheckDone
	for {
		if t.cancelFlag() {
			return SMTFinalCheckGiveUp, nil
		}
		var ok SMTFinalCheckStatus
		var err error
		switch t.finalCheckIdx {
		case 0:
			ok, err = t.checkIntFeasibility()
		case 1:
			var progress bool
			progress, err = t.assumeEqsCore()
			ok = SMTFinalCheckDone
			if progress {
				ok = SMTFinalCheckContinue
			}
		default:
			ok, err = t.processNonLinear()
		}
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		t.finalCheckIdx = (t.finalCheckIdx + 1) % 3
		switch ok {
		case SMTFinalCheckGiveUp:
			result = SMTFinalCheckGiveUp
		case SMTFinalCheckContinue:
			return SMTFinalCheckContinue, nil
		}
		if t.finalCheckIdx == oldIdx {
			break
		}
	}
	if result == SMTFinalCheckDone && t.foundUnsupportedOp {
		result = SMTFinalCheckGiveUp
	}
	return result, nil
}

func (t *SMTTheoryArith) FinalCheck() (SMTFinalCheckStatus, error) {
	ok, err := t.propagateCore()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if !ok {
		return SMTFinalCheckContinue, nil
	}
	if progress, err := t.delayedAssumeEqs(); err != nil || progress {
		return SMTFinalCheckContinue, err
	}
	// m_final_check_idx is restored by the scope record (value_trail).
	t.liberalFinalCheck = true
	t.changedAssignment = false
	result, err := t.finalCheckCore()
	if err != nil || result != SMTFinalCheckDone {
		return result, err
	}
	if !t.changedAssignment {
		return SMTFinalCheckDone, nil
	}
	t.liberalFinalCheck = false
	t.changedAssignment = false
	return t.finalCheckCore()
}

// ---- core.h:1497-1609 ----

func (t *SMTTheoryArith) CanPropagate() bool {
	return t.processAtoms() && int(t.assertedQhead) < len(t.assertedBounds)
}

func (t *SMTTheoryArith) Propagate() error {
	if !t.processAtoms() {
		return nil
	}
	_, err := t.propagateCore()
	return err
}

func (t *SMTTheoryArith) propagateCore() (bool, error) {
	if err := t.flushBoundAxioms(); err != nil {
		return false, err
	}
	if _, err := t.propagateLinearMonomials(); err != nil {
		return false, err
	}
	for int(t.assertedQhead) < len(t.assertedBounds) {
		b := t.assertedBounds[t.assertedQhead]
		t.assertedQhead++
		ok, err := t.assertBound(b)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, t.failed()
		}
	}
	ok, err := t.makeFeasible()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, t.failed()
	}
	if t.cancelFlag() {
		return true, nil
	}
	t.discardUpdateTrail()
	return true, t.propagateBounds()
}

func (t *SMTTheoryArith) failed() error {
	t.restoreAssignment()
	if err := t.toPatch.Reset(); err != nil {
		return err
	}
	t.toCheck = t.toCheck[:0]
	t.inToCheck.Reset()
	return nil
}

func (t *SMTTheoryArith) validateEqInModel(v1, v2 SMTTheoryVar, isTrue bool) bool { return true }

// impliedValue is get_implied_value: the value of a base or quasi-base
// variable computed from its row. The result aliases t.tmp, as in the source.
func (t *SMTTheoryArith) getImpliedValue(v SMTTheoryVar) *smtArithmeticValue {
	sum := &t.tmp
	sum.reset()
	row := &t.rows[t.getVarRow(v)]
	for i := range row.entries {
		entry := &row.entries[i]
		if !entry.dead() && entry.variable != v {
			sum.addMul(&entry.coefficient, &t.value[entry.variable])
		}
	}
	sum.neg()
	return sum
}

// impliedOldValue is get_implied_old_value.
func (t *SMTTheoryArith) getImpliedOldValue(v SMTTheoryVar, result *smtArithmeticValue) bool {
	diff := false
	result.reset()
	row := &t.rows[t.getVarRow(v)]
	for i := range row.entries {
		entry := &row.entries[i]
		if entry.dead() || entry.variable == v {
			continue
		}
		v2 := entry.variable
		if t.inUpdateTrailStack.Contains(uint32(v2)) {
			result.addMul(&entry.coefficient, &t.oldValue[v2])
			diff = true
		} else {
			result.addMul(&entry.coefficient, &t.value[v2])
		}
	}
	result.neg()
	return diff
}

// addRow sets rows[rid1] <- rows[rid1] + coeff * rows[rid2].
func (t *SMTTheoryArith) addRow(rid1 int, coeff *big.Rat, rid2 int, applyGCDTest bool) error {
	t.stats.addRows++
	if t.propagationMode() != smtArithBPNone {
		t.markRowForBoundProp(rid1)
	}
	r1, r2 := &t.rows[rid1], &t.rows[rid2]
	if err := r1.compressIfNeeded(t.columns); err != nil {
		return err
	}
	if err := r2.compressIfNeeded(t.columns); err != nil {
		return err
	}
	if err := r1.saveVariablePositions(t.varPos); err != nil {
		return err
	}
	isOne := coeff.IsInt() && coeff.Num().IsInt64() && coeff.Num().Int64() == 1
	isMinusOne := coeff.IsInt() && coeff.Num().IsInt64() && coeff.Num().Int64() == -1
	var scaled big.Rat
	for i := range r2.entries {
		it := &r2.entries[i]
		if it.dead() {
			continue
		}
		v := it.variable
		pos := t.varPos[v]
		if pos == -1 {
			// variable v is not in row1
			rowIdx, rEntry := r1.addEntry()
			rEntry.variable = v
			switch {
			case isOne:
				rEntry.coefficient.Set(&it.coefficient)
			case isMinusOne:
				rEntry.coefficient.Neg(&it.coefficient)
			default:
				rEntry.coefficient.Mul(&it.coefficient, coeff)
			}
			colIdx, cEntry := t.columns[v].addEntry()
			rEntry.columnIndex = colIdx
			cEntry.rowID = rid1
			cEntry.rowIndex = rowIdx
		} else {
			// variable v is in row1
			rEntry := &r1.entries[pos]
			switch {
			case isOne:
				rEntry.coefficient.Add(&rEntry.coefficient, &it.coefficient)
			case isMinusOne:
				rEntry.coefficient.Sub(&rEntry.coefficient, &it.coefficient)
			default:
				rEntry.coefficient.Add(&rEntry.coefficient, scaled.Mul(&it.coefficient, coeff))
			}
			if rEntry.coefficient.Sign() == 0 {
				colIdx := rEntry.columnIndex
				if err := r1.deleteEntry(pos); err != nil {
					return err
				}
				if err := t.columns[v].deleteEntry(colIdx); err != nil {
					return err
				}
			}
			t.varPos[v] = -1
		}
	}
	if err := r1.resetVariablePositions(t.varPos); err != nil {
		return err
	}
	if applyGCDTest {
		v := r1.base
		if t.isInt(v) && !t.getValue(v).isInt() {
			if _, err := t.gcdTest(rid1); err != nil {
				return err
			}
		}
	}
	return nil
}

// addRows sets r1 <- r1 + sum_i a_xs[i].coeff * row(a_xs[i].var).
func (t *SMTTheoryArith) addRows(r1 int, monomials []smtArithLinearMonomial) error {
	for i := range monomials {
		m := &monomials[i]
		if err := t.addRow(r1, &m.coeff, t.getVarRow(m.v), false); err != nil {
			return err
		}
	}
	t.limitInc(uint(len(monomials)))
	return nil
}

func (t *SMTTheoryArith) saveValue(v SMTTheoryVar) {
	if !t.inUpdateTrailStack.Contains(uint32(v)) {
		t.inUpdateTrailStack.Insert(uint32(v))
		t.oldValue[v].set(&t.value[v])
		t.updateTrailStack = append(t.updateTrailStack, uint32(v))
	}
	t.changedAssignment = true
}

func (t *SMTTheoryArith) discardUpdateTrail() {
	t.inUpdateTrailStack.Reset()
	t.updateTrailStack = t.updateTrailStack[:0]
}

func (t *SMTTheoryArith) restoreAssignment() {
	for _, v := range t.updateTrailStack {
		t.value[v].set(&t.oldValue[v])
	}
	t.updateTrailStack = t.updateTrailStack[:0]
	t.inUpdateTrailStack.Reset()
}

// updateValueCore is m_value[v] += delta.
func (t *SMTTheoryArith) updateValueCore(v SMTTheoryVar, delta *smtArithmeticValue) error {
	t.saveValue(v)
	t.value[v].add(delta)
	if t.isBase(v) && !t.toPatch.Contains(int32(v)) && (t.belowLower(v) || t.aboveUpper(v)) {
		if err := t.toPatch.Insert(int32(v)); err != nil {
			return err
		}
	}
	t.limitInc(1)
	return nil
}

// updateValue is m_value[v] += delta and the induced change of the base
// variables whose rows contain v.
func (t *SMTTheoryArith) updateValue(v SMTTheoryVar, delta *smtArithmeticValue) error {
	if err := t.updateValueCore(v, delta); err != nil {
		return err
	}
	column := &t.columns[v]
	if err := column.compressIfNeeded(t.rows); err != nil {
		return err
	}
	var delta2 smtArithmeticValue
	for i := range column.entries {
		entry := column.entries[i]
		if entry.dead() {
			continue
		}
		row := &t.rows[entry.rowID]
		s := row.base
		if s != SMTNullTheoryVar && !t.isQuasiBase(s) {
			delta2.set(delta)
			delta2.mulRat(&row.entries[entry.rowIndex].coefficient)
			delta2.neg()
			if err := t.updateValueCore(s, &delta2); err != nil {
				return err
			}
		}
	}
	return nil
}

// pivot makes x_j the base variable of the row owned by x_i; a_ij is the
// coefficient of x_j in that row. lazy eliminates x_j only from base rows.
func (t *SMTTheoryArith) pivot(lazy bool, xi, xj SMTTheoryVar, aij *big.Rat, applyGCDTest bool) error {
	t.stats.pivots++
	rowID := t.getVarRow(xi)
	row := &t.rows[rowID]
	isMinusOne := aij.IsInt() && aij.Num().IsInt64() && aij.Num().Int64() == -1
	isOne := aij.IsInt() && aij.Num().IsInt64() && aij.Num().Int64() == 1
	if isMinusOne {
		for i := range row.entries {
			if !row.entries[i].dead() {
				row.entries[i].coefficient.Neg(&row.entries[i].coefficient)
			}
		}
	} else if !isOne {
		divisor := new(big.Rat).Set(aij) // aij may alias an entry of this row
		for i := range row.entries {
			if !row.entries[i].dead() {
				row.entries[i].coefficient.Quo(&row.entries[i].coefficient, divisor)
			}
		}
	}
	t.limitInc(uint(row.size))
	t.setVarRow(xi, -1)
	t.setVarRow(xj, rowID)
	if row.base != xi {
		return ast.ErrASTDomain
	}
	row.base = xj
	t.setVarKind(xi, smtArithmeticNonBase)
	t.setVarKind(xj, smtArithmeticBase)
	return t.eliminate(lazy, xj, applyGCDTest)
}

// eliminate removes x_i from every row other than the one it owns. When
// lazy, only base rows are touched.
func (t *SMTTheoryArith) eliminate(lazy bool, xi SMTTheoryVar, applyGCDTest bool) error {
	rowID := t.getVarRow(xi)
	column := &t.columns[xi]
	sPos := -1
	var akj big.Rat
	for i := 0; i < len(column.entries); i++ {
		entry := column.entries[i]
		if entry.dead() {
			continue
		}
		if entry.rowID != rowID {
			r2 := &t.rows[entry.rowID]
			s2 := r2.base
			if s2 != SMTNullTheoryVar && (!lazy || t.isBase(s2)) {
				akj.Neg(&r2.entries[entry.rowIndex].coefficient)
				if err := t.addRow(entry.rowID, &akj, rowID, applyGCDTest); err != nil {
					return err
				}
				t.limitInc(uint(t.rows[rowID].size+r2.size) * uint(akj.Num().BitLen()/32+1))
			}
		} else {
			sPos = i
		}
	}
	column = &t.columns[xi]
	if column.size == 1 {
		// When lazy pivoting is used, c may not be a singleton after pivoting.
		return column.compressSingleton(t.rows, sPos)
	}
	if !lazy {
		return ast.ErrASTDomain
	}
	return nil
}

// updateAndPivot sets x_j to be the new base variable of the row owned by x_i,
// moving x_i to x_iNewVal.
func (t *SMTTheoryArith) updateAndPivot(xi, xj SMTTheoryVar, aij *big.Rat, xiNewVal *smtArithmeticValue) error {
	var theta smtArithmeticValue
	theta.set(&t.value[xi])
	theta.sub(xiNewVal)
	theta.quoRat(aij)
	if err := t.updateValue(xj, &theta); err != nil {
		return err
	}
	if !t.toPatch.Contains(int32(xj)) && (t.belowLower(xj) || t.aboveUpper(xj)) {
		if err := t.toPatch.Insert(int32(xj)); err != nil {
			return err
		}
	}
	return t.pivot(true, xi, xj, aij, t.eagerGCD)
}

// numNonFreeDepVars is get_num_non_free_dep_vars.
func (t *SMTTheoryArith) getNumNonFreeDepVars(v SMTTheoryVar, bestSoFar int) int {
	result := 0
	if t.isNonFree(v) {
		result = 1
	}
	column := &t.columns[v]
	for i := range column.entries {
		entry := &column.entries[i]
		if entry.dead() {
			continue
		}
		s := t.rows[entry.rowID].base
		if s != SMTNullTheoryVar && t.isBase(s) {
			if t.isNonFree(s) {
				result++
			}
			if result > bestSoFar {
				return result
			}
		}
	}
	return result
}

// selectBlandsPivotCore selects the smallest eligible variable.
func (t *SMTTheoryArith) selectBlandsPivotCore(xi SMTTheoryVar, isBelow bool) (SMTTheoryVar, *big.Rat) {
	max := SMTTheoryVar(t.getNumVars())
	result := max
	var out *big.Rat
	row := &t.rows[t.getVarRow(xi)]
	for i := range row.entries {
		it := &row.entries[i]
		if it.dead() {
			continue
		}
		xj := it.variable
		isNeg := it.coefficient.Sign() > 0
		if isBelow {
			isNeg = it.coefficient.Sign() < 0
		}
		isPos := !isNeg
		if xi != xj && ((isPos && t.aboveLower(xj)) || (isNeg && t.belowUpper(xj))) {
			if xj < result {
				result = xj
				out = &it.coefficient
			}
		}
	}
	if result < max {
		return result, out
	}
	return SMTNullTheoryVar, nil
}

// selectPivotCore selects the variable with the fewest non-free dependents,
// breaking ties by column size and then at random.
func (t *SMTTheoryArith) selectPivotCore(isBelow bool, xi SMTTheoryVar) (SMTTheoryVar, *big.Rat) {
	max := SMTTheoryVar(t.getNumVars())
	result := max
	var out *big.Rat
	row := &t.rows[t.getVarRow(xi)]
	bestColSize := math.MaxInt32
	bestSoFar := math.MaxInt32
	n := uint32(0)
	for i := range row.entries {
		it := &row.entries[i]
		if it.dead() {
			continue
		}
		xj := it.variable
		isNeg := it.coefficient.Sign() > 0
		if isBelow {
			isNeg = it.coefficient.Sign() < 0
		}
		isPos := !isNeg
		if xi != xj && ((isPos && t.aboveLower(xj)) || (isNeg && t.belowUpper(xj))) {
			num := t.getNumNonFreeDepVars(xj, bestSoFar)
			colSize := t.columns[xj].size
			if num < bestSoFar || (num == bestSoFar && colSize < bestColSize) {
				result, out = xj, &it.coefficient
				bestSoFar, bestColSize = num, colSize
				n = 1
			} else if num == bestSoFar && colSize == bestColSize {
				n++
				if t.random.Next()%n == 0 {
					result, out = xj, &it.coefficient
				}
			}
		}
	}
	if result < max {
		return result, out
	}
	return SMTNullTheoryVar, nil
}

func (t *SMTTheoryArith) selectPivot(xi SMTTheoryVar, isBelow bool) (SMTTheoryVar, *big.Rat) {
	if t.blandsRule {
		return t.selectBlandsPivotCore(xi, isBelow)
	}
	return t.selectPivotCore(isBelow, xi)
}

// makeVarFeasible assumes x_i is a base variable and reports false, with a
// conflict set, when it cannot be made feasible.
func (t *SMTTheoryArith) makeVarFeasible(xi SMTTheoryVar) (bool, error) {
	var isBelow bool
	switch {
	case t.belowLower(xi):
		isBelow = true
	case t.aboveUpper(xi):
		isBelow = false
	default:
		return true, nil
	}
	xj, aij := t.selectPivot(xi, isBelow)
	if xj != SMTNullTheoryVar {
		// aij points into row storage that the pivot rewrites.
		a := new(big.Rat).Set(aij)
		target := &t.getBound(xi, !isBelow).base().value
		var newValue smtArithmeticValue
		newValue.set(target)
		return true, t.updateAndPivot(xi, xj, a, &newValue)
	}
	return false, t.signRowConflict(xi, isBelow)
}

// selectLGErrorVar is select_lg_error_var: the variable in m_to_patch with
// the least (greatest) error, removed from the heap.
func (t *SMTTheoryArith) selectLgErrorVar(least bool) (SMTTheoryVar, error) {
	best := SMTNullTheoryVar
	var bestError, currError smtArithmeticValue
	values, err := t.toPatch.AppendValues(nil)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	for _, raw := range values {
		v := SMTTheoryVar(raw)
		switch {
		case t.belowLower(v):
			currError.set(&t.lower(v).base().value)
			currError.sub(t.getValue(v))
		case t.aboveUpper(v):
			currError.set(t.getValue(v))
			currError.sub(&t.upper(v).base().value)
		default:
			continue
		}
		var zero smtArithmeticValue
		if currError.cmp(&zero) <= 0 {
			return SMTNullTheoryVar, ast.ErrASTDomain
		}
		c := currError.cmp(&bestError)
		if best == SMTNullTheoryVar || (!least && c > 0) || (least && c < 0) {
			best = v
			bestError.set(&currError)
		}
	}
	if best == SMTNullTheoryVar {
		return best, t.toPatch.Clear()
	}
	return best, t.toPatch.Erase(int32(best))
}

func (t *SMTTheoryArith) selectSmallestVar() (SMTTheoryVar, error) {
	v, err := t.toPatch.EraseMin()
	return SMTTheoryVar(v), err
}

func (t *SMTTheoryArith) selectVarToFix() (SMTTheoryVar, error) {
	if t.blandsRule {
		return t.selectSmallestVar()
	}
	switch t.params.PivotStrategy {
	case smtArithPivotGreatestError:
		return t.selectLgErrorVar(false)
	case smtArithPivotLeastError:
		return t.selectLgErrorVar(true)
	default:
		return t.selectSmallestVar()
	}
}

// makeFeasible reports whether every variable in m_to_patch could be patched.
func (t *SMTTheoryArith) makeFeasible() (bool, error) {
	t.leftBasis.Reset()
	t.blandsRule = false
	numRepeated := uint32(0)
	for !t.toPatch.Empty() {
		v, err := t.selectVarToFix()
		if err != nil {
			return false, err
		}
		if v == SMTNullTheoryVar {
			break // all variables were satisfied
		}
		if !t.blandsRule {
			if t.leftBasis.Contains(uint32(v)) {
				numRepeated++
				if numRepeated > t.blandsRuleThreshold() {
					t.blandsRule = true
				}
			} else {
				t.leftBasis.Insert(uint32(v))
			}
		}
		ok, err := t.makeVarFeasible(v)
		if err != nil || !ok {
			return false, err
		}
		if t.cancelFlag() {
			return true, nil
		}
	}
	return true, nil
}

// signRowConflict: the row of x_i implies a bound on x_i that contradicts
// the bound it already has.
func (t *SMTTheoryArith) signRowConflict(xi SMTTheoryVar, isBelow bool) error {
	var delta smtArithmeticValue
	row := &t.rows[t.getVarRow(xi)]
	idx := row.indexOf(xi)
	if idx < 0 {
		return ast.ErrASTDomain
	}
	var b smtArithBound
	if isBelow {
		b = t.lower(xi)
		if t.relaxBounds() {
			delta.set(&b.base().value)
			delta.sub(t.getValue(xi))
			delta.sub(t.getEpsilon(xi))
			if delta.sign() < 0 {
				delta.reset()
			}
		}
	} else {
		b = t.upper(xi)
		if t.relaxBounds() {
			delta.set(t.getValue(xi))
			delta.sub(&b.base().value)
			delta.sub(t.getEpsilon(xi))
			if delta.sign() < 0 {
				delta.reset()
			}
		}
	}
	ante := t.acquireAntecedents()
	defer t.releaseAntecedentsScope()
	if err := t.explainBound(row, idx, !isBelow, &delta, ante); err != nil {
		return err
	}
	one := big.NewRat(1, 1)
	if err := b.pushJustification(ante, one, t.coeffsEnabled()); err != nil {
		return err
	}
	return t.setConflict(ante, "farkas")
}

func (t *SMTTheoryArith) assertLower(b smtArithBound) (bool, error) {
	c := b.base()
	v := c.variable
	k := &c.value
	u, l := t.upper(v), t.lower(v)
	if u != nil && k.cmp(&u.base().value) > 0 {
		return false, t.signBoundConflict(u, b)
	}
	if l != nil && k.cmp(&l.base().value) <= 0 {
		return true, nil // redundant
	}
	switch t.getVarKind(v) {
	case smtArithmeticQuasiBase:
		if err := t.quasiBaseRow2BaseRow(t.getVarRow(v)); err != nil {
			return false, err
		}
		fallthrough
	case smtArithmeticBase:
		if !t.toPatch.Contains(int32(v)) && t.getValue(v).cmp(k) < 0 {
			if err := t.toPatch.Insert(int32(v)); err != nil {
				return false, err
			}
		}
	case smtArithmeticNonBase:
		if t.getValue(v).cmp(k) < 0 {
			if err := t.setValue(v, k); err != nil {
				return false, err
			}
		}
	}
	t.pushBoundTrail(v, l, false)
	if err := t.setBound(b, false); err != nil {
		return false, err
	}
	if t.propagationMode() != smtArithBPNone {
		t.markRowsForBoundProp(v)
	}
	return true, nil
}

func (t *SMTTheoryArith) assertUpper(b smtArithBound) (bool, error) {
	c := b.base()
	v := c.variable
	k := &c.value
	u, l := t.upper(v), t.lower(v)
	if l != nil && k.cmp(&l.base().value) < 0 {
		return false, t.signBoundConflict(l, b)
	}
	if u != nil && k.cmp(&u.base().value) >= 0 {
		return true, nil // redundant
	}
	switch t.getVarKind(v) {
	case smtArithmeticQuasiBase:
		if err := t.quasiBaseRow2BaseRow(t.getVarRow(v)); err != nil {
			return false, err
		}
		fallthrough
	case smtArithmeticBase:
		if !t.toPatch.Contains(int32(v)) && t.getValue(v).cmp(k) > 0 {
			if err := t.toPatch.Insert(int32(v)); err != nil {
				return false, err
			}
		}
	case smtArithmeticNonBase:
		if t.getValue(v).cmp(k) > 0 {
			if err := t.setValue(v, k); err != nil {
				return false, err
			}
		}
	}
	t.pushBoundTrail(v, u, true)
	if err := t.setBound(b, true); err != nil {
		return false, err
	}
	if t.propagationMode() != smtArithBPNone {
		t.markRowsForBoundProp(v)
	}
	return true, nil
}

// assertBound asserts b and reports false, with a conflict set, when it
// contradicts a known bound.
func (t *SMTTheoryArith) assertBound(b smtArithBound) (bool, error) {
	c := b.base()
	v := c.variable
	if c.atom {
		if t.unassignedAtoms[v] == 0 {
			return false, ast.ErrASTDomain
		}
		t.pushDecUnassignedAtomsTrail(v)
		t.unassignedAtoms[v]--
	}
	if c.kind == smtArithmeticLowerBound {
		t.stats.assertLower++
		return t.assertLower(b)
	}
	t.stats.assertUpper++
	return t.assertUpper(b)
}

// signBoundConflict: bounds b1 and b2 on one variable are contradictory.
func (t *SMTTheoryArith) signBoundConflict(b1, b2 smtArithBound) error {
	if b1.base().variable != b2.base().variable {
		return ast.ErrASTDomain
	}
	ante := t.acquireAntecedents()
	defer t.releaseAntecedentsScope()
	one := big.NewRat(1, 1)
	if err := b1.pushJustification(ante, one, t.coeffsEnabled()); err != nil {
		return err
	}
	if err := b2.pushJustification(ante, one, t.coeffsEnabled()); err != nil {
		return err
	}
	return t.setConflict(ante, "farkas")
}

// markRowForBoundProp marks row r1 for bound propagation.
func (t *SMTTheoryArith) markRowForBoundProp(r1 int) {
	if !t.inToCheck.Contains(uint32(r1)) && t.rows[r1].base != SMTNullTheoryVar {
		t.inToCheck.Insert(uint32(r1))
		t.toCheck = append(t.toCheck, uint32(r1))
	}
}

// markRowsForBoundProp marks every row that contains v.
func (t *SMTTheoryArith) markRowsForBoundProp(v SMTTheoryVar) {
	column := &t.columns[v]
	for i := range column.entries {
		if !column.entries[i].dead() {
			t.markRowForBoundProp(column.entries[i].rowID)
		}
	}
}

// isRowUsefulForBoundProp computes lower_idx and upper_idx: >= 0 the single
// monomial for which the row can imply a bound, -1 every monomial, -2 none.
func (t *SMTTheoryArith) isRowUsefulForBoundProp(r *smtArithmeticRow) (lowerIdx, upperIdx int) {
	lowerIdx, upperIdx = -1, -1
	update := func(idx *int, i int) {
		if *idx == -1 {
			*idx = i
		} else {
			*idx = -2
		}
	}
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		if t.skipBigCoeffs() && smtRatIsBig(&it.coefficient) {
			return -2, -2
		}
		isPos := it.coefficient.Sign() > 0
		if t.lower(it.variable) == nil {
			if isPos {
				update(&upperIdx, i)
			} else {
				update(&lowerIdx, i)
			}
		}
		if t.upper(it.variable) == nil {
			if isPos {
				update(&lowerIdx, i)
			} else {
				update(&upperIdx, i)
			}
		}
		if lowerIdx == -2 && upperIdx == -2 {
			return
		}
	}
	return
}

// implyBoundForMonomial implies a bound for the monomial at idx from the
// others, and from it a bound for the monomial variable.
func (t *SMTTheoryArith) implyBoundForMonomial(r *smtArithmeticRow, idx int, isLower bool) error {
	entry := &r.entries[idx]
	if t.unassignedAtoms[entry.variable] == 0 {
		return nil
	}
	var impliedK smtArithmeticValue
	for idx2 := range r.entries {
		it := &r.entries[idx2]
		if it.dead() || idx == idx2 {
			continue
		}
		b := t.getBound(it.variable, smtArithUseUpper(&it.coefficient, isLower))
		if b == nil {
			return ast.ErrASTDomain
		}
		impliedK.subMul(&it.coefficient, &b.base().value)
	}
	impliedK.quoRat(&entry.coefficient)
	if (entry.coefficient.Sign() > 0) == isLower {
		// impliedK is a lower bound for entry.variable
		curr := t.lower(entry.variable)
		if curr == nil || impliedK.cmp(&curr.base().value) > 0 {
			return t.mkImpliedBound(r, idx, isLower, entry.variable, smtArithmeticLowerBound, &impliedK)
		}
		return nil
	}
	curr := t.upper(entry.variable)
	if curr == nil || impliedK.cmp(&curr.base().value) < 0 {
		return t.mkImpliedBound(r, idx, isLower, entry.variable, smtArithmeticUpperBound, &impliedK)
	}
	return nil
}

// implyBoundForAllMonomials implies a lower (upper) bound for every
// monomial in the row, using one pass to compute the common sum.
func (t *SMTTheoryArith) implyBoundForAllMonomials(r *smtArithmeticRow, isLower bool) error {
	var bb smtArithmeticValue
	for i := range r.entries {
		it := &r.entries[i]
		if !it.dead() {
			b := t.getBound(it.variable, smtArithUseUpper(&it.coefficient, isLower))
			bb.subMul(&it.coefficient, &b.base().value)
		}
	}
	var impliedK smtArithmeticValue
	for idx := range r.entries {
		it := &r.entries[idx]
		if it.dead() || t.unassignedAtoms[it.variable] == 0 {
			continue
		}
		b := t.getBound(it.variable, smtArithUseUpper(&it.coefficient, isLower))
		impliedK.set(&bb)
		impliedK.addMul(&it.coefficient, &b.base().value)
		impliedK.quoRat(&it.coefficient)
		if (it.coefficient.Sign() > 0) == isLower {
			curr := t.lower(it.variable)
			if curr == nil || impliedK.cmp(&curr.base().value) > 0 {
				if err := t.mkImpliedBound(r, idx, isLower, it.variable, smtArithmeticLowerBound, &impliedK); err != nil {
					return err
				}
			}
		} else {
			curr := t.upper(it.variable)
			if curr == nil || impliedK.cmp(&curr.base().value) < 0 {
				if err := t.mkImpliedBound(r, idx, isLower, it.variable, smtArithmeticUpperBound, &impliedK); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// explainBound creates the explanation for the bound of the monomial at idx.
// delta relaxes the explanation: the implied bound may be delta weaker.
// isLower says whether the other monomials imply a lower (upper) bound.
func (t *SMTTheoryArith) explainBound(r *smtArithmeticRow, idx int, isLower bool, delta *smtArithmeticValue, ante *smtArithmeticAntecedents) error {
	if delta.sign() < 0 {
		return ast.ErrASTDomain
	}
	if !t.relaxBounds() && (len(ante.literals) > 0 || len(ante.equalities) > 0) {
		return nil
	}
	proofs := t.coeffsEnabled()
	if t.relaxBounds() {
		// a delta change of the variable at idx is a |coeff|*delta change of
		// its monomial.
		coeff := new(big.Rat).Abs(&r.entries[idx].coefficient)
		delta.mulRat(coeff)
	}
	for idx2 := range r.entries {
		it := &r.entries[idx2]
		if it.dead() || idx == idx2 {
			continue
		}
		b := t.getBound(it.variable, smtArithUseUpper(&it.coefficient, isLower))
		if b == nil {
			return ast.ErrASTDomain
		}
		if !t.relaxBounds() || delta.isZero() {
			if err := b.pushJustification(ante, &it.coefficient, proofs); err != nil {
				return err
			}
			continue
		}
		coeff := new(big.Rat).Abs(&it.coefficient)
		invCoeff := new(big.Rat).Inv(coeff)
		bc := b.base()
		isBLower := bc.kind == smtArithmeticLowerBound
		var k1, limitK1, k2 smtArithmeticValue
		k1.set(&bc.value)
		limitK1.set(&k1)
		// if the max decrease (increase) of the current monomial is delta,
		// the maximal decrease (increase) of the variable is delta/|coeff|.
		if isBLower {
			limitK1.subMul(invCoeff, delta)
		} else {
			limitK1.addMul(invCoeff, delta)
		}
		k2.set(&k1)
		var newAtom *smtArithmeticAtom
		for _, a := range t.varOccs[it.variable] {
			if smtArithBound(a) == b {
				continue
			}
			val := t.getAssignment(a.boolean)
			if val == lbool.Undef {
				continue
			}
			if err := a.assign(val == lbool.True, t.getEpsilon(a.variable)); err != nil {
				return err
			}
			if a.kind == bc.kind {
				if isBLower {
					if a.value.cmp(&limitK1) >= 0 && a.value.cmp(&k2) < 0 {
						k2.set(&a.value)
						newAtom = a
					}
				} else if a.value.cmp(&limitK1) <= 0 && a.value.cmp(&k2) > 0 {
					k2.set(&a.value)
					newAtom = a
				}
			}
		}
		if newAtom == nil {
			if err := b.pushJustification(ante, coeff, proofs); err != nil {
				return err
			}
			continue
		}
		var diff smtArithmeticValue
		if isBLower {
			diff.set(&k1)
			diff.sub(&k2)
		} else {
			diff.set(&k2)
			diff.sub(&k1)
		}
		delta.subMul(coeff, &diff)
		if err := newAtom.pushJustification(ante, coeff, proofs); err != nil {
			return err
		}
	}
	return nil
}

// mkImpliedBound assigns the atoms of v that the implied bound k decides.
func (t *SMTTheoryArith) mkImpliedBound(r *smtArithmeticRow, idx int, isLower bool, v SMTTheoryVar, kind smtArithmeticBoundKind, k *smtArithmeticValue) error {
	epsilon := t.getEpsilon(v)
	var delta smtArithmeticValue
	for i := 0; i < len(t.varOccs[v]); i++ {
		a := t.varOccs[v][i]
		bv := a.boolean
		l := NewSMTLiteral(bv, false)
		if t.getAssignment(bv) != lbool.Undef {
			continue
		}
		k2 := &a.threshold
		delta.reset()
		if a.atomKind == smtArithmeticLowerAtom {
			// v >= k, k >= k2 |- v >= k2
			if kind == smtArithmeticLowerBound && k.cmp(k2) >= 0 {
				if t.relaxBounds() {
					delta.set(k)
					delta.sub(k2)
				}
				if err := t.assignBoundLiteral(l, r, idx, isLower, &delta); err != nil {
					return err
				}
			}
			// v <= k, k < k2 |- v < k2 |- not v >= k2
			if kind == smtArithmeticUpperBound && k.cmp(k2) < 0 {
				// k < k2 is not enough: with k = -1/5*eps and k2 = 0,
				// v <= -1/5*eps means not v >= 0, i.t. v <= -eps.
				delta.set(k2)
				delta.sub(k)
				delta.sub(epsilon)
				if delta.sign() >= 0 {
					if err := t.assignBoundLiteral(l.Negated(), r, idx, isLower, &delta); err != nil {
						return err
					}
				}
			}
		} else {
			// v >= k, k > k2 |- v > k2 |- not v <= k2
			if kind == smtArithmeticLowerBound && k.cmp(k2) > 0 {
				delta.set(k)
				delta.sub(k2)
				delta.sub(epsilon)
				if delta.sign() >= 0 {
					if err := t.assignBoundLiteral(l.Negated(), r, idx, isLower, &delta); err != nil {
						return err
					}
				}
			}
			// v <= k, k <= k2 |- v <= k2
			if kind == smtArithmeticUpperBound && k.cmp(k2) <= 0 {
				if t.relaxBounds() {
					delta.set(k2)
					delta.sub(k)
				}
				if err := t.assignBoundLiteral(l, r, idx, isLower, &delta); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// assignBoundLiteral assigns l, justified by the explanation of the bound
// at idx: as a small auxiliary lemma, or as a theory propagation.
func (t *SMTTheoryArith) assignBoundLiteral(l SMTLiteral, r *smtArithmeticRow, idx int, isLower bool, delta *smtArithmeticValue) error {
	t.stats.boundProps++
	ante := t.acquireAntecedents()
	defer t.releaseAntecedentsScope()
	if err := t.explainBound(r, idx, isLower, delta, ante); err != nil {
		return err
	}
	if uint32(len(ante.literals)) < t.smallLemmaSize() && len(ante.equalities) == 0 {
		lits := make([]SMTLiteral, 0, 1+len(ante.literals))
		lits = append(lits, l)
		for _, a := range ante.literals {
			lits = append(lits, a.Negated())
		}
		return t.mkAuxLemmaClause(lits, ante)
	}
	return t.assignPropagatedLiteral(l, ante)
}

// propagateBounds traverses the rows in to_check and derives improved bounds.
func (t *SMTTheoryArith) propagateBounds() error {
	for i := 0; i < len(t.toCheck); i++ {
		r := &t.rows[t.toCheck[i]]
		if r.base == SMTNullTheoryVar {
			continue
		}
		if uint32(r.size) < t.maxLemmaSize() { // ignore big rows
			lowerIdx, upperIdx := t.isRowUsefulForBoundProp(r)
			var err error
			if lowerIdx >= 0 {
				err = t.implyBoundForMonomial(r, lowerIdx, true)
			} else if lowerIdx == -1 {
				err = t.implyBoundForAllMonomials(r, true)
			}
			if err != nil {
				return err
			}
			if upperIdx >= 0 {
				err = t.implyBoundForMonomial(r, upperIdx, false)
			} else if upperIdx == -1 {
				err = t.implyBoundForAllMonomials(r, false)
			}
			if err != nil {
				return err
			}
			// cheap equality detection piggybacks on this loop
			if err = t.propagateCheapEq(int(t.toCheck[i])); err != nil {
				return err
			}
		}
	}
	t.toCheck = t.toCheck[:0]
	t.inToCheck.Reset()
	return nil
}

// get_context().get_assignment(bool_var).
func (t *SMTTheoryArith) getAssignment(bv SMTBoolVar) lbool.LBool {
	value, err := t.state.Assignment(NewSMTLiteral(bv, false))
	if err != nil {
		return lbool.Undef
	}
	return value
}

// get_manager().limit().inc(n).
func (t *SMTTheoryArith) limitInc(n uint) bool {
	return t.state.manager.Limit.IncOffset(uint32(n))
}

// assign_bound_literal's ctx.mk_clause(lits, theory_lemma_justification,
// CLS_AUX_LEMMA) branch.
func (t *SMTTheoryArith) mkAuxLemmaClause(lits []SMTLiteral, ante *smtArithmeticAntecedents) (result error) {
	m := t.state.manager
	var justification *SMTTheoryLemmaJustification
	if t.proofsEnabled() {
		params, err := ante.parameters(m, "assign-bounds")
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(m, params)) }()
		if justification, err = NewSMTTheoryLemmaJustification(m, t.FamilyID(), lits, t.state.boolExpr, params); err != nil {
			return err
		}
	}
	var owner SMTJustification
	if justification != nil {
		owner = justification
	}
	beforeClauses, beforeOwners := len(t.state.clauses), len(t.state.ownedJustifications)
	err := t.state.addClauseKindWithJustificationAndDeleteHandler(lits, false, SMTClauseAuxLemma, ast.ASTHandle{}, owner, nil)
	if justification != nil && len(t.state.clauses) == beforeClauses && len(t.state.ownedJustifications) == beforeOwners {
		err = errors.Join(err, justification.Release(m))
	}
	return err
}

// assign_bound_literal's ctx.assign(l, ext_theory_propagation_justification)
// branch.
func (t *SMTTheoryArith) assignPropagatedLiteral(l SMTLiteral, ante *smtArithmeticAntecedents) (result error) {
	m := t.state.manager
	params, err := ante.parameters(m, "assign-bounds")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(m, params)) }()
	owner, err := NewSMTExtTheoryPropagationJustification(m, t.FamilyID(), ante.literals, ante.equalities, l, params)
	if err != nil {
		return err
	}
	justification, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(m))
	}
	if err = t.state.AssignLiteral(l, justification); err != nil {
		return errors.Join(err, owner.Release(m))
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	return nil
}

// ---- core.h:3056-3100 (set_conflict) ----

func (t *SMTTheoryArith) setConflict(ante *smtArithmeticAntecedents, proofRule string) error {
	return t.setConflictCore(ante.literals, ante.equalities, ante, proofRule)
}

func (t *SMTTheoryArith) setConflictDerived(ante smtArithDerivedBound, bounds *smtArithmeticAntecedents, proofRule string) error {
	d := ante.derived()
	return t.setConflictCore(d.literals, d.equalities, bounds, proofRule)
}

func (t *SMTTheoryArith) setConflictCore(lits []SMTLiteral, eqs []SMTEnodePair, bounds *smtArithmeticAntecedents, proofRule string) (result error) {
	if len(lits) == 0 && len(eqs) == 0 {
		return ast.ErrASTDomain
	}
	t.stats.conflicts++
	t.numConflicts++
	m := t.state.manager
	params, err := bounds.parameters(m, proofRule)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(m, params)) }()
	if err = t.recordConflict(lits, eqs, bounds); err != nil {
		return err
	}
	if len(eqs) == 0 {
		return t.state.SetTheoryConflict(t.FamilyID(), lits, params...)
	}
	if t.state.inconsistent {
		return nil
	}
	owner, err := NewSMTExtTheoryConflictJustification(m, t.FamilyID(), lits, eqs, params)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(m))
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	t.state.inconsistent = true
	t.state.conflict = wrapped
	t.state.conflictLiteral = SMTNullLiteral
	t.state.statistics.NumConflicts++
	return nil
}

// collectFixedVarJustifications pushes the bound justifications of the fixed
// variables in r.
func (t *SMTTheoryArith) collectFixedVarJustifications(r *smtArithmeticRow, ante *smtArithmeticAntecedents) error {
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() || !t.isFixed(it.variable) {
			continue
		}
		if err := t.lower(it.variable).pushJustification(ante, &it.coefficient, t.coeffsEnabled()); err != nil {
			return err
		}
		if err := t.upper(it.variable).pushJustification(ante, &it.coefficient, t.coeffsEnabled()); err != nil {
			return err
		}
	}
	return nil
}

// ---- core.h:3155-3313 (model generation and model checker support) ----

// updateEpsilon updates m_epsilon using the inequality l <= u.
func (t *SMTTheoryArith) updateEpsilon(l, u *smtArithmeticValue) {
	if l.rational.Cmp(&u.rational) < 0 && l.infinitesimal.Cmp(&u.infinitesimal) > 0 {
		var num, den, newEpsilon big.Rat
		num.Sub(&u.rational, &l.rational)
		den.Sub(&l.infinitesimal, &u.infinitesimal)
		newEpsilon.Quo(&num, &den)
		if newEpsilon.Cmp(&t.epsilon) < 0 {
			t.epsilon.Set(&newEpsilon)
		}
	}
}

func (t *SMTTheoryArith) computeEpsilon() {
	t.epsilon.SetInt64(1)
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		l, u := t.lower(v), t.upper(v)
		if l != nil {
			t.updateEpsilon(&l.base().value, t.getValue(v))
		}
		if u != nil {
			t.updateEpsilon(t.getValue(v), &u.base().value)
		}
	}
}

// refineEpsilon halves epsilon until no two shared real variables with
// distinct values receive the same model value.
func (t *SMTTheoryArith) refineEpsilon() error {
	for {
		mapping := make(map[string]SMTTheoryVar)
		num := SMTTheoryVar(t.getNumVars())
		refine := false
		for v := SMTTheoryVar(0); v < num; v++ {
			if t.isIntSrc(v) {
				continue
			}
			n, err := t.Enode(v)
			if err != nil {
				return err
			}
			shared, err := t.ctxIsShared(n)
			if err != nil {
				return err
			}
			if !shared {
				continue
			}
			val := t.getValue(v)
			if t.isInfinite(val) {
				continue
			}
			var value big.Rat
			value.Mul(&t.epsilon, &val.infinitesimal)
			value.Add(&value, &val.rational)
			key := value.RatString()
			if v2, found := mapping[key]; found {
				if t.getValue(v).cmp(t.getValue(v2)) != 0 {
					// v and v2 are not known to be equal; epsilon makes them so.
					refine = true
					break
				}
			} else {
				mapping[key] = v
			}
		}
		if !refine {
			return nil
		}
		t.epsilon.Quo(&t.epsilon, big.NewRat(2, 1))
	}
}

// ctxIsShared is context::is_shared(n).
func (t *SMTTheoryArith) ctxIsShared(n *SMTEnode) (bool, error) {
	shared, consult, err := t.state.sharedTheoryNode(n)
	if err != nil || !consult {
		return shared, err
	}
	return t.isShared(n.root.TheoryVar(t.FamilyID()))
}

func (t *SMTTheoryArith) InitializeModel(mg *SMTModelGenerator) error {
	factory, err := protomodel.NewArithmeticFactory(t.state.manager)
	if err != nil {
		return err
	}
	t.factory = factory
	if err = mg.RegisterFactory(factory); err != nil {
		return err
	}
	t.computeEpsilon()
	if !t.modelDependsOnComputedEpsilon {
		return t.refineEpsilon()
	}
	return nil
}

func (t *SMTTheoryArith) ModelValue(n *SMTEnode, mg *SMTModelGenerator) (SMTModelValueProcedure, error) {
	v := n.TheoryVar(t.FamilyID())
	if v == SMTNullTheoryVar {
		return nil, ast.ErrASTDomain
	}
	val := t.getValue(v)
	var num big.Rat
	num.Mul(&t.epsilon, &val.infinitesimal)
	num.Add(&num, &val.rational)
	if t.isInt(v) && !num.IsInt() {
		// Truncating non-integer value; possible for non-linear constraints.
		num.SetInt(smtRatFloor(&num))
	}
	isIntSrc, err := t.isIntSort(t.var2expr(v))
	if err != nil {
		return nil, err
	}
	value, err := t.factory.MkNumValue(&num, isIntSrc)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(value)
}

// toExpr is to_expr: only values without an infinitesimal have an expression.
func (t *SMTTheoryArith) toExpr(val *smtArithmeticValue, isInt bool) (ast.ASTHandle, bool, error) {
	if val.infinitesimal.Sign() != 0 {
		return ast.ASTHandle{}, false, nil
	}
	h, err := t.state.manager.ArithmeticNumeralRat(&val.rational, isInt)
	return h, err == nil, err
}

func (t *SMTTheoryArith) GetTheoryValue(n *SMTEnode) (ast.ASTHandle, bool, error) {
	v := n.TheoryVar(t.FamilyID())
	if v == SMTNullTheoryVar {
		return ast.ASTHandle{}, false, nil
	}
	val := t.getValue(v)
	if t.isInt(v) && !val.isInt() {
		return ast.ASTHandle{}, false, nil
	}
	return t.toExpr(val, t.isInt(v))
}

func (t *SMTTheoryArith) getLower(n *SMTEnode) (ast.ASTHandle, bool, error) {
	v := n.TheoryVar(t.FamilyID())
	if v == SMTNullTheoryVar || t.lower(v) == nil {
		return ast.ASTHandle{}, false, nil
	}
	return t.toExpr(&t.lower(v).base().value, t.isInt(v))
}

func (t *SMTTheoryArith) getUpper(n *SMTEnode) (ast.ASTHandle, bool, error) {
	v := n.TheoryVar(t.FamilyID())
	if v == SMTNullTheoryVar || t.upper(v) == nil {
		return ast.ASTHandle{}, false, nil
	}
	return t.toExpr(&t.upper(v).base().value, t.isInt(v))
}

// ---- core.h:3315-3372 (backtracking) ----

func (t *SMTTheoryArith) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.arithEqAdapter.PushScope(); err != nil {
		return err
	}
	t.scopes = append(t.scopes, smtArithScope{
		atomsLim:                uint32(len(t.atoms)),
		boundTrailLim:           uint32(len(t.boundTrail)),
		unassignedAtomsTrailLim: uint32(len(t.unassignedAtomsTrail)),
		assertedBoundsLim:       uint32(len(t.assertedBounds)),
		assertedQheadOld:        t.assertedQhead,
		boundsToDeleteLim:       uint32(len(t.boundsToDelete)),
		nlMonomialsLim:          uint32(len(t.nlMonomials)),
		nlPropagatedLim:         uint32(len(t.nlPropagated)),
		foundUnsupportedOp:      t.foundUnsupportedOp,
		foundUnderspecifiedOp:   t.foundUnderspecifiedOp,
		finalCheckIdx:           t.finalCheckIdx,
		assumeEqCandidatesLim:   uint32(len(t.assumeEqCandidates)),
		assumeEqHead:            t.assumeEqHead,
		nlRounds:                t.nlRounds,
		nlStrategyIdx:           t.nlStrategyIdx,
		nlGbExhausted:           t.nlGbExhausted,
	})
	return nil
}

func (t *SMTTheoryArith) PopScopes(numScopes uint32) error {
	// The update trail may not be empty: quasi_base_row2base_row also
	// pushes to it, so restore_assignment must run.
	t.restoreAssignment()
	if err := t.toPatch.Reset(); err != nil {
		return err
	}
	lvl := uint32(len(t.scopes))
	if numScopes > lvl {
		return ast.ErrASTDomain
	}
	newLvl := lvl - numScopes
	s := t.scopes[newLvl]
	if err := t.restoreBounds(int(s.boundTrailLim)); err != nil {
		return err
	}
	t.restoreUnassignedAtoms(int(s.unassignedAtomsTrailLim))
	clear(t.assertedBounds[s.assertedBoundsLim:])
	t.assertedBounds = t.assertedBounds[:s.assertedBoundsLim]
	t.assertedQhead = s.assertedQheadOld
	t.restoreNLPropagatedFlag(int(s.nlPropagatedLim))
	t.nlMonomials = t.nlMonomials[:s.nlMonomialsLim]
	t.delAtoms(int(s.atomsLim))
	t.delBounds(int(s.boundsToDeleteLim))
	oldNumVars, err := t.OldNumVariables(numScopes)
	if err != nil {
		return err
	}
	if err = t.delVars(oldNumVars); err != nil {
		return err
	}
	// value_trail entries pushed in the popped scopes
	t.foundUnsupportedOp = s.foundUnsupportedOp
	t.foundUnderspecifiedOp = s.foundUnderspecifiedOp
	t.finalCheckIdx = s.finalCheckIdx
	t.assumeEqCandidates = t.assumeEqCandidates[:s.assumeEqCandidatesLim]
	t.assumeEqHead = s.assumeEqHead
	t.nlRounds = s.nlRounds
	t.nlStrategyIdx = s.nlStrategyIdx
	t.nlGbExhausted = s.nlGbExhausted
	t.scopes = t.scopes[:newLvl]
	if err = t.arithEqAdapter.PopScopes(numScopes); err != nil {
		return err
	}
	if err = t.SMTTheoryBase.PopScopes(numScopes); err != nil {
		return err
	}
	ok, err := t.makeFeasible()
	if err != nil {
		return err
	}
	if !ok || !t.toPatch.Empty() {
		return ast.ErrASTDomain // VERIFY(make_feasible())
	}
	t.toCheck = t.toCheck[:0]
	t.inToCheck.Reset()
	t.newAtoms = t.newAtoms[:0]
	return nil
}
func (t *SMTTheoryArith) restoreNLPropagatedFlag(oldTrailSize int) {
	for i := len(t.nlPropagated) - 1; i >= oldTrailSize; i-- {
		t.data[t.nlPropagated[i]].nonlinearPropagated = false
	}
	t.nlPropagated = t.nlPropagated[:oldTrailSize]
}

func (t *SMTTheoryArith) restoreBounds(oldTrailSize int) error {
	for i := len(t.boundTrail) - 1; i >= oldTrailSize; i-- {
		bt := t.boundTrail[i]
		v := bt.v
		t.restoreBound(v, bt.oldBound, bt.upper)
		if t.lazyPivotingLvl() > 2 && bt.oldBound == nil && t.isBase(v) && t.isFree(v) {
			if err := t.eliminate(false, v, false); err != nil {
				return err
			}
			if t.columns[v].size != 1 {
				return ast.ErrASTDomain
			}
			t.setVarKind(v, smtArithmeticQuasiBase)
		}
		t.boundTrail[i] = smtArithBoundTrail{}
	}
	t.boundTrail = t.boundTrail[:oldTrailSize]
	return nil
}

func (t *SMTTheoryArith) restoreUnassignedAtoms(oldTrailSize int) {
	for i := len(t.unassignedAtomsTrail) - 1; i >= oldTrailSize; i-- {
		t.unassignedAtoms[t.unassignedAtomsTrail[i]]++
	}
	t.unassignedAtomsTrail = t.unassignedAtomsTrail[:oldTrailSize]
}

func (t *SMTTheoryArith) delAtoms(oldSize int) {
	for i := len(t.atoms) - 1; i >= oldSize; i-- {
		a := t.atoms[i]
		t.eraseBv2a(a.boolean)
		occs := t.varOccs[a.variable]
		t.varOccs[a.variable] = occs[:len(occs)-1]
		t.atoms[i] = nil
	}
	t.atoms = t.atoms[:oldSize]
}

func (t *SMTTheoryArith) delBounds(oldSize int) {
	clear(t.boundsToDelete[oldSize:])
	t.boundsToDelete = t.boundsToDelete[:oldSize]
}

// delVars removes the variables created since the scope with oldNumVars
// variables, deleting or pivoting their rows.
func (t *SMTTheoryArith) delVars(oldNumVars int) error {
	numVars := len(t.columns)
	if numVars == oldNumVars {
		return nil
	}
	for v := SMTTheoryVar(numVars) - 1; int(v) >= oldNumVars; v-- {
		switch t.getVarKind(v) {
		case smtArithmeticQuasiBase:
			t.delRow(t.getVarRow(v))
		case smtArithmeticBase:
			if t.lazyPivotingLvl() > 0 {
				if err := t.eliminate(false, v, false); err != nil {
					return err
				}
			}
			t.delRow(t.getVarRow(v))
		case smtArithmeticNonBase:
			entry, err := t.getABaseRowThatContains(v)
			if err != nil {
				return err
			}
			if entry != nil {
				r := &t.rows[entry.rowID]
				coefficient := new(big.Rat).Set(&r.entries[entry.rowIndex].coefficient)
				if err := t.pivot(false, r.base, v, coefficient, false); err != nil {
					return err
				}
				t.delRow(t.getVarRow(v))
			}
		}
		t.inUpdateTrailStack.Remove(uint32(v))
		t.leftBasis.Remove(uint32(v))
		// The source removes a variable id from the row set m_in_to_check.
		if uint32(v) < t.inToCheck.Domain() {
			t.inToCheck.Remove(uint32(v))
		}
	}
	t.columns = t.columns[:oldNumVars]
	t.data = t.data[:oldNumVars]
	t.value = t.value[:oldNumVars]
	t.oldValue = t.oldValue[:oldNumVars]
	t.varOccs = t.varOccs[:oldNumVars]
	t.unassignedAtoms = t.unassignedAtoms[:oldNumVars]
	t.varPos = t.varPos[:oldNumVars]
	t.bounds[0] = t.bounds[0][:oldNumVars]
	t.bounds[1] = t.bounds[1][:oldNumVars]
	return nil
}

func (t *SMTTheoryArith) delRow(rowID int) {
	r := &t.rows[rowID]
	for i := range r.entries {
		it := &r.entries[i]
		if !it.dead() {
			_ = t.columns[it.variable].deleteEntry(it.columnIndex)
		}
	}
	r.base = SMTNullTheoryVar
	r.reset()
	t.deadRows = append(t.deadRows, rowID)
}

// ---- core.h:3523-3541 (antecedents) ----

// acquireAntecedents is the antecedents constructor: it borrows and resets
// the next of three reusable records; releaseAntecedentsScope is the
// destructor.
func (t *SMTTheoryArith) acquireAntecedents() *smtArithmeticAntecedents {
	a := &t.antecedents[t.antecedentsIndex]
	a.reset()
	t.antecedentsIndex++
	return a
}
func (t *SMTTheoryArith) releaseAntecedentsScope() { t.antecedentsIndex-- }

// smtArithUseUpper is the bound selector of the bound propagation code:
// is_lower ? coeff.is_pos() : coeff.is_neg().
func smtArithUseUpper(c *big.Rat, isLower bool) bool {
	if isLower {
		return c.Sign() > 0
	}
	return c.Sign() < 0
}
