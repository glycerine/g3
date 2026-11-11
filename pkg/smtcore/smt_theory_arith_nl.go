package smtcore

// Source: src/smt/theory_arith_nl.h, ported in source order. interval is
// smtOldInterval (old_interval.go) and m_dep_manager is
// smtArithmeticBoundDependencyManager, whose leaves are base bounds; the
// polymorphic bound of each leaf is kept in t.depBounds.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/grobner"
)

func (t *SMTTheoryArith) mkNaryMul(x *smtArithExprs, args []ast.ASTHandle, isInt bool) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return t.mkNumeral(x, big.NewRat(1, 1), isInt)
	case 1:
		return args[0], nil
	case 2:
		return t.mkApp(x, smtArithOpMul, args[0], args[1])
	}
	if _, ok := t.isNumeral(args[0]); ok {
		rest, err := t.mkApp(x, smtArithOpMul, args[1:]...)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return t.mkApp(x, smtArithOpMul, args[0], rest)
	}
	return t.mkApp(x, smtArithOpMul, args...)
}

func (t *SMTTheoryArith) mkNaryAdd(x *smtArithExprs, args []ast.ASTHandle, isInt bool) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return t.mkNumeral(x, new(big.Rat), isInt)
	case 1:
		return args[0], nil
	}
	return t.mkApp(x, smtArithOpAdd, args...)
}

// markVar inserts v into vars and alreadyFound if it is not already there.
func (t *SMTTheoryArith) markVar(v SMTTheoryVar, vars *[]SMTTheoryVar, alreadyFound map[SMTTheoryVar]bool) {
	if alreadyFound[v] {
		return
	}
	alreadyFound[v] = true
	*vars = append(*vars, v)
}

// markDependents invokes mark_var for all variables in rows that contain v.
func (t *SMTTheoryArith) markDependents(v SMTTheoryVar, vars *[]SMTTheoryVar, alreadyFound map[SMTTheoryVar]bool, alreadyVisitedRows map[int]bool) error {
	if t.isPureMonomial(t.var2expr(v)) {
		args, err := t.args(t.var2expr(v))
		if err != nil {
			return err
		}
		for _, curr := range args {
			t.markVar(t.expr2var(curr), vars, alreadyFound)
		}
	}
	if t.isFixed(v) {
		return nil
	}
	c := &t.columns[v]
	for i := range c.entries {
		it := &c.entries[i]
		if it.dead() || alreadyVisitedRows[it.rowID] {
			continue
		}
		alreadyVisitedRows[it.rowID] = true
		r := &t.rows[it.rowID]
		s := r.base
		// ignore quasi base vars... they should not be used if the problem
		// is non linear.
		if t.isQuasiBase(s) {
			continue
		}
		// A free base variable other than v was eliminated by substitution;
		// its row need not be in the non linear cluster.
		if t.isFree(s) && s != v {
			continue
		}
		for j := range r.entries {
			it2 := &r.entries[j]
			if !it2.dead() && !t.isFixed(it2.variable) {
				t.markVar(it2.variable, vars, alreadyFound)
			}
		}
	}
	return nil
}

// getNonLinearCluster stores in vars the variables of the non linear cluster
// of constraints that are not satisfied by the current assignment.
func (t *SMTTheoryArith) getNonLinearCluster(vars *[]SMTTheoryVar) error {
	if len(t.nlMonomials) == 0 {
		return nil
	}
	alreadyFound := make(map[SMTTheoryVar]bool)
	alreadyVisitedRows := make(map[int]bool)
	for _, v := range t.nlMonomials {
		if t.state.isRelevant(t.var2expr(v)) {
			t.markVar(v, vars, alreadyFound)
		}
	}
	for idx := 0; idx < len(*vars); idx++ {
		if err := t.markDependents((*vars)[idx], vars, alreadyFound, alreadyVisitedRows); err != nil {
			return err
		}
	}
	return nil
}

// analyzeMonomial returns the number (0, 1, or 2 meaning 2 or more) of
// variables of odd power without bounds, and the index of such a variable
// (-1 if none).
func (t *SMTTheoryArith) analyzeMonomial(m ast.ASTHandle) (uint32, int, error) {
	args, err := t.args(m)
	if err != nil {
		return 0, -1, err
	}
	var v ast.ASTHandle
	hasVar := false
	power := uint32(0)
	c := uint32(0)
	freeVarIdx := -1
	idx := 0
	for _, arg := range args {
		switch {
		case !hasVar:
			v, hasVar, power = arg, true, 1
		case arg == v:
			power++
		default:
			if power%2 == 1 && t.isFree(t.expr2var(v)) {
				c++
				freeVarIdx = idx
				if c > 1 {
					return 2, freeVarIdx, nil
				}
			}
			v, power = arg, 1
			idx++
		}
	}
	if power%2 == 1 && t.isFree(t.expr2var(v)) {
		c++
		freeVarIdx = idx
	}
	return c, freeVarIdx, nil
}

// getMonomialBody: given a monomial c*M, return M.
func (t *SMTTheoryArith) getMonomialBody(m ast.ASTHandle) (ast.ASTHandle, error) {
	args, err := t.args(m)
	if err != nil {
		return m, err
	}
	if _, ok := t.isNumeral(args[0]); ok {
		return args[1], nil
	}
	return m, nil
}

// getMonomialCoeff: given a monomial c*M, return c.
func (t *SMTTheoryArith) getMonomialCoeff(m ast.ASTHandle) (*big.Rat, error) {
	arg0, err := t.state.manager.GetArg(m, 0)
	if err != nil {
		return nil, err
	}
	if r, ok := t.isNumeral(arg0); ok {
		return r, nil
	}
	return big.NewRat(1, 1), nil
}

// getNumVarsInMonomial returns the number of distinct variables of m.
func (t *SMTTheoryArith) getNumVarsInMonomial(m ast.ASTHandle) (uint32, error) {
	m, err := t.getMonomialBody(m)
	if err != nil {
		return 0, err
	}
	if !t.isOp(m, smtArithOpMul) {
		return 1, nil
	}
	args, err := t.args(m)
	if err != nil {
		return 0, err
	}
	numVars := uint32(0)
	var v ast.ASTHandle
	hasVar := false
	for _, curr := range args {
		if !hasVar || v != curr {
			numVars++
			v, hasVar = curr, true
		}
	}
	return numVars, nil
}

// getVarAndDegree returns the i-th variable of m and its power.
func (t *SMTTheoryArith) getVarAndDegree(m ast.ASTHandle, i uint32) (ast.ASTHandle, uint32, error) {
	m, err := t.getMonomialBody(m)
	if err != nil {
		return ast.ASTHandle{}, 0, err
	}
	if !t.isOp(m, smtArithOpMul) {
		return m, 1, nil
	}
	args, err := t.args(m)
	if err != nil {
		return ast.ASTHandle{}, 0, err
	}
	currIdx := uint32(0)
	var v ast.ASTHandle
	hasVar := false
	power := uint32(0)
	for _, arg := range args {
		switch {
		case !hasVar:
			v, hasVar, power = arg, true, 1
		case v == arg:
			power++
		default:
			if currIdx == i {
				return v, power, nil
			}
			currIdx++
			v, power = arg, 1
		}
	}
	return v, power, nil
}

// mkLeaf is m_dep_manager.mk_leaf(b).
func (t *SMTTheoryArith) mkLeaf(b smtArithBound) *smtArithmeticBoundDependency {
	t.depBounds[b.base()] = b
	return t.depManager.Leaf(b.base())
}

// mkIntervalForVar returns an interval using the bounds of v.
func (t *SMTTheoryArith) mkIntervalForVar(v SMTTheoryVar) (*smtOldInterval, error) {
	l, u := t.lower(v), t.upper(v)
	switch {
	case l != nil && u != nil:
		lv, uv := &l.base().value, &u.base().value
		return newSMTOldIntervalBounds(&t.depManager, &lv.rational, lv.infinitesimal.Sign() != 0, t.mkLeaf(l),
			&uv.rational, uv.infinitesimal.Sign() != 0, t.mkLeaf(u))
	case l != nil:
		lv := &l.base().value
		return newSMTOldIntervalHalfBound(&t.depManager, &lv.rational, lv.infinitesimal.Sign() != 0, true, t.mkLeaf(l))
	case u != nil:
		uv := &u.base().value
		return newSMTOldIntervalHalfBound(&t.depManager, &uv.rational, uv.infinitesimal.Sign() != 0, false, t.mkLeaf(u))
	}
	return newSMTOldInterval(&t.depManager)
}

// mkIntervalForExpr returns an interval for n using its bounds.
func (t *SMTTheoryArith) mkIntervalForExpr(n ast.ASTHandle) (*smtOldInterval, error) {
	if !t.hasVar(n) {
		return newSMTOldInterval(&t.depManager)
	}
	return t.mkIntervalForVar(t.expr2var(n))
}

func smtArithRatBitsize(q *big.Rat) uint {
	return uint(q.Num().BitLen() + q.Denom().BitLen())
}

// mulBoundOf sets target *= [lower(var), upper(var)]^power.
func (t *SMTTheoryArith) mulBoundOf(v ast.ASTHandle, power uint32, target *smtOldInterval) error {
	i, err := t.mkIntervalForVar(t.expr2var(v))
	if err != nil {
		return err
	}
	i.power(power)
	if err = target.mul(i); err != nil {
		return err
	}
	if target.lowerOpen || target.minusInfinity() {
		t.limitInc(1)
	} else {
		t.limitInc(smtArithRatBitsize(&target.lower.value))
	}
	if target.upperOpen || target.plusInfinity() {
		t.limitInc(1)
	} else {
		t.limitInc(smtArithRatBitsize(&target.upper.value))
	}
	return nil
}

// evaluateAsInterval evaluates n with interval arithmetic; internalized
// subexpressions use their bounds. Only +, *, and numerals are handled.
func (t *SMTTheoryArith) evaluateAsInterval(n ast.ASTHandle) (*smtOldInterval, error) {
	if t.hasVar(n) {
		return t.mkIntervalForExpr(n)
	}
	if t.isOp(n, smtArithOpAdd) {
		r, err := newSMTOldIntervalPoint(&t.depManager, new(big.Rat), nil, nil)
		if err != nil {
			return nil, err
		}
		args, err := t.args(n)
		if err != nil {
			return nil, err
		}
		for _, arg := range args {
			a, err := t.evaluateAsInterval(arg)
			if err != nil {
				return nil, err
			}
			if err = r.add(a); err != nil {
				return nil, err
			}
		}
		return r, nil
	}
	if t.isOp(n, smtArithOpMul) {
		coeff, err := t.getMonomialCoeff(n)
		if err != nil {
			return nil, err
		}
		r, err := newSMTOldIntervalPoint(&t.depManager, coeff, nil, nil)
		if err != nil {
			return nil, err
		}
		numVars, err := t.getNumVarsInMonomial(n)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < numVars; i++ {
			v, power, err := t.getVarAndDegree(n, i)
			if err != nil {
				return nil, err
			}
			it, err := t.evaluateAsInterval(v)
			if err != nil {
				return nil, err
			}
			it.power(power)
			if err = r.mul(it); err != nil {
				return nil, err
			}
		}
		return r, nil
	}
	if t.isOp(n, smtArithOpToReal) {
		arg, err := t.state.manager.GetArg(n, 0)
		if err != nil {
			return nil, err
		}
		return t.evaluateAsInterval(arg)
	}
	if val, ok := t.isNumeral(n); ok {
		return newSMTOldIntervalPoint(&t.depManager, val, nil, nil)
	}
	return newSMTOldInterval(&t.depManager)
}

// dependency2NewBound copies the justification stored in dep to newBound.
func (t *SMTTheoryArith) dependency2NewBound(dep *smtArithmeticBoundDependency, newBound smtArithDerivedBound) error {
	bounds := t.depManager.Linearize(dep, nil)
	clear(t.tmpLitSet)
	clear(t.tmpEqSet)
	zero := new(big.Rat)
	for _, b := range bounds {
		owner, found := t.depBounds[b]
		if !found {
			return ast.ErrASTDomain
		}
		if err := t.accumulateJustification(owner, newBound, zero, t.tmpLitSet, t.tmpEqSet); err != nil {
			return err
		}
	}
	return nil
}

// mkDerivedNlBound creates a new derived bound justified by dep.
func (t *SMTTheoryArith) mkDerivedNlBound(v SMTTheoryVar, coeff *smtArithmeticValue, k smtArithmeticBoundKind, dep *smtArithmeticBoundDependency) error {
	coeffNorm, err := t.normalizeBound(v, coeff, k)
	if err != nil {
		return err
	}
	newBound := &smtArithmeticDerivedBound{}
	newBound.variable, newBound.kind = v, k
	newBound.value.set(&coeffNorm)
	t.boundsToDelete = append(t.boundsToDelete, newBound)
	t.assertedBounds = append(t.assertedBounds, newBound)
	// copy justification to new bound
	return t.dependency2NewBound(dep, newBound)
}

// updateBoundsUsingInterval updates the bounds of v using i, reporting
// whether i improves them.
func (t *SMTTheoryArith) updateBoundsUsingInterval(v SMTTheoryVar, i *smtOldInterval) (bool, error) {
	r := false
	if !i.minusInfinity() {
		lv, err := i.lowerValue()
		if err != nil {
			return false, err
		}
		var newLower smtArithmeticValue
		newLower.setRational(lv)
		if i.lowerOpen {
			if t.isInt(v) {
				if newLower.isInt() {
					newLower.rational.Add(&newLower.rational, big.NewRat(1, 1))
				} else {
					newLower.setRational(smtInfCeil(&newLower))
				}
			} else {
				newLower.add(t.getEpsilon(v))
			}
		}
		oldLower := t.lower(v)
		if oldLower == nil || newLower.cmp(&oldLower.base().value) > 0 {
			if err = t.mkDerivedNlBound(v, &newLower, smtArithmeticLowerBound, i.lowerDep); err != nil {
				return false, err
			}
			r = true
		}
	}
	if !i.plusInfinity() {
		uv, err := i.upperValue()
		if err != nil {
			return false, err
		}
		var newUpper smtArithmeticValue
		newUpper.setRational(uv)
		if i.upperOpen {
			if t.isInt(v) {
				if newUpper.isInt() {
					newUpper.rational.Sub(&newUpper.rational, big.NewRat(1, 1))
				} else {
					newUpper.setRational(smtInfFloor(&newUpper))
				}
			} else {
				newUpper.sub(t.getEpsilon(v))
			}
		}
		oldUpper := t.upper(v)
		if oldUpper == nil || newUpper.cmp(&oldUpper.base().value) < 0 {
			if err = t.mkDerivedNlBound(v, &newUpper, smtArithmeticUpperBound, i.upperDep); err != nil {
				return false, err
			}
			r = true
		}
	}
	return r, nil
}

// propagateNlUpward uses the bounds of the variables to bound m.
func (t *SMTTheoryArith) propagateNlUpward(m ast.ASTHandle) (bool, error) {
	numVars, err := t.getNumVarsInMonomial(m)
	if err != nil {
		return false, err
	}
	newBounds, err := newSMTOldIntervalPoint(&t.depManager, big.NewRat(1, 1), nil, nil)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < numVars; i++ {
		v, power, err := t.getVarAndDegree(m, i)
		if err != nil {
			return false, err
		}
		if err = t.mulBoundOf(v, power, newBounds); err != nil {
			return false, err
		}
	}
	return t.updateBoundsUsingInterval(t.expr2var(m), newBounds)
}

// propagateNlDownward bounds the i-th variable of m from the bounds of m and
// the other variables. Roots are not supported: a power != 1 is skipped.
func (t *SMTTheoryArith) propagateNlDownward(m ast.ASTHandle, i uint32) (bool, error) {
	v, power, err := t.getVarAndDegree(m, i)
	if err != nil {
		return false, err
	}
	if power != 1 {
		return false, nil // TODO in the source: n-th root in interval.
	}
	numVars, err := t.getNumVarsInMonomial(m)
	if err != nil {
		return false, err
	}
	otherBounds, err := newSMTOldIntervalPoint(&t.depManager, big.NewRat(1, 1), nil, nil)
	if err != nil {
		return false, err
	}
	for j := uint32(0); j < numVars; j++ {
		w, pw, err := t.getVarAndDegree(m, j)
		if err != nil {
			return false, err
		}
		if w == v {
			continue
		}
		if err = t.mulBoundOf(w, pw, otherBounds); err != nil {
			return false, err
		}
	}
	if otherBounds.containsZero() {
		return false, nil // interval division requires a divisor without 0.
	}
	r, err := t.mkIntervalForExpr(m)
	if err != nil {
		return false, err
	}
	if err = r.div(otherBounds); err != nil {
		return false, err
	}
	return t.updateBoundsUsingInterval(t.expr2var(v), r)
}

// propagateNlBound uses monomial m: i == -1 bounds m from its variables,
// otherwise it bounds the i-th variable.
func (t *SMTTheoryArith) propagateNlBound(m ast.ASTHandle, i int) (bool, error) {
	if i == -1 {
		return t.propagateNlUpward(m)
	}
	return t.propagateNlDownward(m, uint32(i))
}

// propagateNlBoundsOf propagates bounds to m and all of its variables.
func (t *SMTTheoryArith) propagateNlBoundsOf(m ast.ASTHandle) (bool, error) {
	result, err := t.propagateNlUpward(m)
	if err != nil {
		return false, err
	}
	numVars, err := t.getNumVarsInMonomial(m)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < numVars; i++ {
		p, err := t.propagateNlDownward(m, i)
		if err != nil {
			return false, err
		}
		if p {
			t.stats.nlBounds++
			result = true
		}
	}
	return result, nil
}

// propagateNlBounds propagates bounds using the non linear monomials.
func (t *SMTTheoryArith) propagateNlBounds() (bool, error) {
	t.depManager.Reset()
	clear(t.depBounds)
	propagated := false
	for i := 0; i < len(t.nlMonomials); i++ {
		v := t.nlMonomials[i]
		m := t.var2expr(v)
		if !t.state.isRelevant(m) {
			continue
		}
		numBadVars, freeVarIdx, err := t.analyzeMonomial(m)
		if err != nil {
			return false, err
		}
		if numBadVars >= 2 {
			continue
		}
		isFreeM := t.isFree(t.expr2var(m))
		if numBadVars == 1 && isFreeM {
			continue
		}
		var p bool
		if numBadVars == 0 {
			if !isFreeM {
				if p, err = t.propagateNlBoundsOf(m); err != nil {
					return false, err
				}
				if p {
					propagated = true
				}
				continue
			}
			if p, err = t.propagateNlBound(m, -1); err != nil {
				return false, err
			}
		} else if p, err = t.propagateNlBound(m, freeVarIdx); err != nil {
			return false, err
		}
		if p {
			t.stats.nlBounds++
			propagated = true
		}
	}
	return propagated, nil
}

// getRationalValue is get_value(v, computed_epsilon): the value of v as a
// rational, computing epsilon on first use when v has an infinitesimal.
func (t *SMTTheoryArith) getRationalValue(v SMTTheoryVar, computedEpsilon *bool) *big.Rat {
	val := t.getValue(v)
	if val.infinitesimal.Sign() != 0 && !*computedEpsilon {
		t.computeEpsilon()
		*computedEpsilon = true
		t.modelDependsOnComputedEpsilon = true
	}
	r := new(big.Rat).Mul(&t.epsilon, &val.infinitesimal)
	return r.Add(r, &val.rational)
}

// checkMonomialAssignment: for the monomial x_1*...*x_n of v,
// get_value(x_1)*...*get_value(x_n) = get_value(v).
func (t *SMTTheoryArith) checkMonomialAssignment(v SMTTheoryVar, computedEpsilon *bool) (bool, error) {
	m := t.var2expr(v)
	args, err := t.args(m)
	if err != nil {
		return false, err
	}
	val := big.NewRat(1, 1)
	for _, arg := range args {
		curr := t.expr2var(arg)
		val.Mul(val, t.getRationalValue(curr, computedEpsilon))
	}
	return t.getRationalValue(v, computedEpsilon).Cmp(val) == 0, nil
}

// checkMonomialAssignments: every relevant monomial agrees with its factors.
func (t *SMTTheoryArith) checkMonomialAssignments() (bool, error) {
	computedEpsilon := false
	for _, v := range t.nlMonomials {
		n, err := t.Enode(v)
		if err != nil {
			return false, err
		}
		if !t.state.isRelevant(n.owner) {
			continue
		}
		ok, err := t.checkMonomialAssignment(v, &computedEpsilon)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// findNlVarForBranching selects an integer variable of a monomial with an
// invalid assignment, preferring small ranges, else at random. Free
// variables are not considered.
func (t *SMTTheoryArith) findNlVarForBranching() (SMTTheoryVar, error) {
	target := SMTNullTheoryVar
	bounded := false
	n := uint32(0)
	var rng big.Rat
	for _, v := range t.nlMonomials {
		if t.isReal(v) {
			continue
		}
		computedEpsilon := false
		ok, err := t.checkMonomialAssignment(v, &computedEpsilon)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		if ok {
			continue
		}
		args, err := t.args(t.var2expr(v))
		if err != nil {
			return SMTNullTheoryVar, err
		}
		for _, arg := range args {
			curr := t.state.context.enodes.get(arg).TheoryVar(t.FamilyID())
			if t.isFixed(curr) || !t.isInt(curr) {
				continue
			}
			if t.isBounded(curr) {
				var newRange big.Rat
				newRange.Sub(&t.upperBound(curr).rational, &t.lowerBound(curr).rational)
				if !bounded || newRange.Cmp(&rng) < 0 {
					target = curr
					rng.Set(&newRange)
					bounded = true
				}
			} else if !bounded {
				n++
				if t.random.Next()%n == 0 {
					target = curr
				}
			}
		}
	}
	return target, nil
}

// branchNlIntVar branches on an integer variable of an unsatisfied
// monomial: v <= l if v has a lower bound, else v >= u, else v = 0.
func (t *SMTTheoryArith) branchNlIntVar(v SMTTheoryVar) (result bool, err error) {
	t.stats.nlBranching++
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	var bound ast.ASTHandle
	switch {
	case t.lower(v) != nil:
		k, err := t.mkNumeral(&x, &t.lowerBound(v).rational, true)
		if err != nil {
			return false, err
		}
		bound, err = t.mkApp(&x, smtArithOpLE, t.var2expr(v), k)
		if err != nil {
			return false, err
		}
	case t.upper(v) != nil:
		k, err := t.mkNumeral(&x, &t.upperBound(v).rational, true)
		if err != nil {
			return false, err
		}
		bound, err = t.mkApp(&x, smtArithOpGE, t.var2expr(v), k)
		if err != nil {
			return false, err
		}
	default:
		zero, err := t.mkNumeral(&x, new(big.Rat), true)
		if err != nil {
			return false, err
		}
		bound, err = x.hold(m.MkEq(t.var2expr(v), zero))
		if err != nil {
			return false, err
		}
	}
	if err = t.state.Internalize(bound, true); err != nil {
		return false, err
	}
	if err = t.state.markRelevant(bound); err != nil {
		return false, err
	}
	l, err := t.state.Literal(bound)
	if err != nil {
		return false, err
	}
	// force the context to case split to true first, independently of the
	// phase selection strategy.
	if _, err = t.state.setScopedTrueFirst(l.Var()); err != nil {
		return false, err
	}
	return true, nil
}

// isMonomialLinear: at most one variable is not fixed, or a fixed variable
// is zero.
func (t *SMTTheoryArith) isMonomialLinear(m ast.ASTHandle) (bool, error) {
	args, err := t.args(m)
	if err != nil {
		return false, err
	}
	numNlVars := 0
	for _, arg := range args {
		v := t.expr2var(arg)
		if !t.isFixed(v) {
			numNlVars++
		} else if t.lowerBound(v).isZero() {
			return true, nil
		}
	}
	return numNlVars <= 1, nil
}

// getMonomialFixedVarProduct is the product of the values of the fixed
// variables of m.
func (t *SMTTheoryArith) getMonomialFixedVarProduct(m ast.ASTHandle) (*big.Rat, error) {
	args, err := t.args(m)
	if err != nil {
		return nil, err
	}
	r := big.NewRat(1, 1)
	for _, arg := range args {
		v := t.expr2var(arg)
		if t.isFixed(v) {
			r.Mul(r, &t.lowerBound(v).rational)
		}
	}
	return r, nil
}

// getMonomialNonFixedVar returns the first non fixed variable of m.
func (t *SMTTheoryArith) getMonomialNonFixedVar(m ast.ASTHandle) (ast.ASTHandle, bool, error) {
	args, err := t.args(m)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	for _, arg := range args {
		if !t.isFixed(t.expr2var(arg)) {
			return arg, true, nil
		}
	}
	return ast.ASTHandle{}, false, nil
}

// propagateLinearMonomial checks whether the monomial of v became linear and
// asserts the linear equality.
func (t *SMTTheoryArith) propagateLinearMonomial(v SMTTheoryVar) (result bool, err error) {
	if t.data[v].nonlinearPropagated {
		return false, nil // already propagated this monomial.
	}
	m := t.var2expr(v)
	linear, err := t.isMonomialLinear(m)
	if err != nil || !linear {
		return false, err
	}
	t.stats.nlLinear++
	t.data[v].nonlinearPropagated = true
	t.nlPropagated = append(t.nlPropagated, v)
	k, err := t.getMonomialFixedVarProduct(m)
	if err != nil {
		return false, err
	}
	var xn ast.ASTHandle
	hasXn := false
	if k.Sign() != 0 {
		if xn, hasXn, err = t.getMonomialNonFixedVar(m); err != nil {
			return false, err
		}
	}
	var newLower, newUpper *smtArithmeticDerivedBound
	if hasXn {
		// All but x_n are fixed: x_1*...*x_n = k*x_n with
		// k = lower(x_1)*...*lower(x_{n-1}); assert
		// (= (+ (* x_1 ... x_n) (* -k x_n)) 0).
		ex := smtArithExprs{m: t.state.manager}
		defer func() { err = errors.Join(err, ex.release()) }()
		k.Neg(k)
		kxn := xn
		if !smtRatIsOne(k) {
			kn, err := t.mkNumeral(&ex, k, t.isInt(v))
			if err != nil {
				return false, err
			}
			if kxn, err = t.mkApp(&ex, smtArithOpMul, kn, xn); err != nil {
				return false, err
			}
		}
		rhs, err := t.mkApp(&ex, smtArithOpAdd, m, kxn)
		if err != nil {
			return false, err
		}
		if !t.hasVar(rhs) {
			if err = t.state.Internalize(rhs, false); err != nil {
				return false, err
			}
			if err = t.state.markRelevant(rhs); err != nil {
				return false, err
			}
		}
		newV := t.expr2var(rhs)
		if newV == SMTNullTheoryVar {
			return false, ast.ErrASTDomain
		}
		newLower = &smtArithmeticDerivedBound{}
		newLower.variable, newLower.kind = newV, smtArithmeticLowerBound
		newUpper = &smtArithmeticDerivedBound{}
		newUpper.variable, newUpper.kind = newV, smtArithmeticUpperBound
	} else {
		// One of the x_i is zero, or all of them are fixed: (= (* x_1 ... x_n) k).
		newLower = &smtArithmeticDerivedBound{}
		newLower.variable, newLower.kind = v, smtArithmeticLowerBound
		newLower.value.setRational(k)
		newUpper = &smtArithmeticDerivedBound{}
		newUpper.variable, newUpper.kind = v, smtArithmeticUpperBound
		newUpper.value.setRational(k)
	}
	t.boundsToDelete = append(t.boundsToDelete, newLower)
	t.assertedBounds = append(t.assertedBounds, newLower)
	t.boundsToDelete = append(t.boundsToDelete, newUpper)
	t.assertedBounds = append(t.assertedBounds, newUpper)
	// The justification is the lower and upper bounds of all fixed variables.
	clear(t.tmpLitSet)
	clear(t.tmpEqSet)
	args, err := t.args(m)
	if err != nil {
		return false, err
	}
	zero := new(big.Rat)
	foundZero := false
	for _, arg := range args {
		if foundZero {
			break
		}
		fv := t.expr2var(arg)
		if !t.isFixed(fv) {
			continue
		}
		l, u := t.lower(fv), t.upper(fv)
		if l.base().value.isZero() {
			// if zero was found, then it is the explanation
			foundZero = true
			clear(t.tmpLitSet)
			clear(t.tmpEqSet)
			newLower.literals = newLower.literals[:0]
			newLower.equalities = newLower.equalities[:0]
		}
		if err = t.accumulateJustification(l, newLower, zero, t.tmpLitSet, t.tmpEqSet); err != nil {
			return false, err
		}
		if err = t.accumulateJustification(u, newLower, zero, t.tmpLitSet, t.tmpEqSet); err != nil {
			return false, err
		}
	}
	newUpper.literals = append(newUpper.literals, newLower.literals...)
	newUpper.equalities = append(newUpper.equalities, newLower.equalities...)
	return true, nil
}

// propagateLinearMonomials propagates the monomials that became linear. An
// index is used because propagation may extend m_nl_monomials.
func (t *SMTTheoryArith) propagateLinearMonomials() (bool, error) {
	p := false
	for i := 0; i < len(t.nlMonomials); i++ {
		r, err := t.propagateLinearMonomial(t.nlMonomials[i])
		if err != nil {
			return false, err
		}
		if r {
			p = true
		}
	}
	return p, nil
}

// ---- nl.h:1105-1731 (cross nested forms) ----

// isProblematicNonLinearRow: the same non-fixed variable occurs in two
// different monomials of the row.
func (t *SMTTheoryArith) isProblematicNonLinearRow(r *smtArithmeticRow) (bool, error) {
	seen := make(map[SMTTheoryVar]bool) // m_tmp_var_set
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		v := it.variable
		if t.isFixed(v) {
			continue
		}
		if t.isPureMonomial(t.var2expr(v)) {
			args, err := t.args(t.var2expr(v))
			if err != nil {
				return false, err
			}
			for _, arg := range args {
				if seen[t.expr2var(arg)] {
					return true, nil
				}
			}
			for _, arg := range args {
				if curr := t.expr2var(arg); !t.isFixed(curr) {
					seen[curr] = true
				}
			}
		} else {
			if seen[v] {
				return true, nil
			}
			seen[v] = true
		}
	}
	return false, nil
}

// isMixedRealInteger: the row mixes real and integer variables, so it
// cannot be converted back to a well-sorted expression.
func (t *SMTTheoryArith) isMixedRealInteger(r *smtArithmeticRow) bool {
	foundInt, foundReal := false, false
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		if t.isInt(it.variable) {
			foundInt = true
		}
		if t.isReal(it.variable) {
			foundReal = true
		}
		if foundInt && foundReal {
			return true
		}
	}
	return false
}

// isIntegerRow is is_integer(row): the row contains only integer variables.
func (t *SMTTheoryArith) isIntegerRow(r *smtArithmeticRow) bool {
	for i := range r.entries {
		if !r.entries[i].dead() && !t.isInt(r.entries[i].variable) {
			return false
		}
	}
	return true
}

// smtArithCoeffExpr is coeff_expr; smtArithVarNumOccs is var_num_occs.
type smtArithCoeffExpr struct {
	coeff *big.Rat
	e     ast.ASTHandle
}
type smtArithVarNumOccs struct {
	v    ast.ASTHandle
	occs uint32
}

// getPolynomialInfo stores in varinfo the non-fixed variables occurring in
// more than one monomial of p, with their number of occurrences, walking
// the member obj_map m_var2num_occs (reset per call, its capacity kept) in
// table order.
func (t *SMTTheoryArith) getPolynomialInfo(p []smtArithCoeffExpr) ([]smtArithVarNumOccs, error) {
	if t.var2NumOccs == nil {
		t.var2NumOccs = ast.NewObjMap(t.state.manager)
	}
	occs := t.var2NumOccs
	occs.Reset()
	addOcc := func(v ast.ASTHandle) error {
		if t.hasVar(v) && !t.isFixed(t.expr2var(v)) {
			n, _, err := occs.FindNum(v)
			if err != nil {
				return err
			}
			return occs.InsertNum(v, n+1)
		}
		return nil
	}
	for _, ce := range p {
		m := ce.e
		if t.isPureMonomial(m) {
			numVars, err := t.getNumVarsInMonomial(m)
			if err != nil {
				return nil, err
			}
			for i := uint32(0); i < numVars; i++ {
				v, _, err := t.getVarAndDegree(m, i)
				if err != nil {
					return nil, err
				}
				if err = addOcc(v); err != nil {
					return nil, err
				}
			}
		} else if _, ok := t.isNumeral(m); ok {
			continue
		} else if t.state.context.enodes.get(m) != nil {
			if err := addOcc(m); err != nil {
				return nil, err
			}
		} else {
			return nil, ast.ErrASTDomain // UNREACHABLE
		}
	}
	var varinfo []smtArithVarNumOccs
	for _, entry := range occs.Entries() {
		if entry.Num > 1 {
			varinfo = append(varinfo, smtArithVarNumOccs{v: entry.Key, occs: uint32(entry.Num)})
		}
	}
	return varinfo, nil
}

// p2expr converts p into an expression.
func (t *SMTTheoryArith) p2expr(p []smtArithCoeffExpr) (ast.ASTHandle, error) {
	x := &t.nlNewExprs
	args := make([]ast.ASTHandle, 0, len(p))
	for _, ce := range p {
		c, v := ce.coeff, ce.e
		if smtRatIsOne(c) {
			args = append(args, v)
			continue
		}
		isIntV, err := t.isIntSort(v)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		var m ast.ASTHandle
		if c2, ok := t.isNumeral(v); ok {
			m, err = t.mkNumeral(x, new(big.Rat).Mul(c, c2), isIntV && c.IsInt() && c2.IsInt())
		} else {
			var cn ast.ASTHandle
			if cn, err = t.mkNumeral(x, c, c.IsInt() && isIntV); err == nil {
				m, err = t.mkApp(x, smtArithOpMul, cn, v)
			}
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args = append(args, m)
	}
	return t.mkNaryAdd(x, args, false)
}

// power returns var^power.
func (t *SMTTheoryArith) power(v ast.ASTHandle, power uint32) (ast.ASTHandle, error) {
	r := v
	var err error
	for i := uint32(1); i < power; i++ {
		if r, err = t.mkApp(&t.nlNewExprs, smtArithOpMul, v, r); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return r, nil
}

// inMonovariateMonomials reports whether v occurs only in two monovariate
// monomials, returning their positions, coefficients and powers.
func (t *SMTTheoryArith) inMonovariateMonomials(p []smtArithCoeffExpr, v ast.ASTHandle) (i1 int, c1 *big.Rat, n1 uint32, i2 int, c2 *big.Rat, n2 uint32, ok bool, err error) {
	idx := 0
	set := func(i int, c *big.Rat, power uint32) bool {
		switch idx {
		case 0:
			c1, n1, i1, idx = c, power, i, 1
		case 1:
			c2, n2, i2, idx = c, power, i, 2
		default:
			return false
		}
		return true
	}
	for i, ce := range p {
		m := ce.e
		if t.isPureMonomial(m) {
			numVars, err := t.getNumVarsInMonomial(m)
			if err != nil {
				return 0, nil, 0, 0, nil, 0, false, err
			}
			for j := uint32(0); j < numVars; j++ {
				w, pw, err := t.getVarAndDegree(m, j)
				if err != nil {
					return 0, nil, 0, 0, nil, 0, false, err
				}
				if w == v {
					if numVars > 1 {
						return 0, nil, 0, 0, nil, 0, false, nil
					}
					if !set(i, ce.coeff, pw) {
						return 0, nil, 0, 0, nil, 0, false, nil
					}
				}
			}
		} else if m == v {
			if !set(i, ce.coeff, 1) {
				return 0, nil, 0, 0, nil, 0, false, nil
			}
		}
	}
	return i1, c1, n1, i2, c2, n2, idx == 2, nil
}

// getDegreeOf returns the degree of v in m.
func (t *SMTTheoryArith) getDegreeOf(m, v ast.ASTHandle) (uint32, error) {
	if m == v {
		return 1, nil
	}
	if t.isPureMonomial(m) {
		numVars, err := t.getNumVarsInMonomial(m)
		if err != nil {
			return 0, err
		}
		for i := uint32(0); i < numVars; i++ {
			w, pw, err := t.getVarAndDegree(m, i)
			if err != nil {
				return 0, err
			}
			if w == v {
				return pw, nil
			}
		}
	}
	return 0, nil
}

// getMinDegree returns the minimal degree of v in p.
func (t *SMTTheoryArith) getMinDegree(p []smtArithCoeffExpr, v ast.ASTHandle) (uint32, error) {
	d := ^uint32(0)
	for _, ce := range p {
		dm, err := t.getDegreeOf(ce.e, v)
		if err != nil {
			return 0, err
		}
		if dm < d {
			d = dm
		}
		if d == 0 {
			return 0, nil
		}
	}
	return d, nil
}

// factor divides m by v^d.
func (t *SMTTheoryArith) factor(m, v ast.ASTHandle, d uint32) (ast.ASTHandle, error) {
	if d == 0 {
		return m, nil
	}
	isIntV, err := t.isIntSort(v)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if m == v {
		return t.mkNumeral(&t.nlNewExprs, big.NewRat(1, 1), isIntV)
	}
	args, err := t.args(m)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	idx := uint32(0)
	var newArgs []ast.ASTHandle
	for _, arg := range args {
		if arg == v && idx < d {
			idx++
			continue
		}
		newArgs = append(newArgs, arg)
	}
	return t.mkNaryMul(&t.nlNewExprs, newArgs, isIntV)
}

// horner returns the horner extension of p with respect to v.
func (t *SMTTheoryArith) horner(p []smtArithCoeffExpr, v ast.ASTHandle) (ast.ASTHandle, error) {
	d, err := t.getMinDegree(p, v)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var e, r []smtArithCoeffExpr // monomials/x^d with degree d, and the rest
	for _, kv := range p {
		f, err := t.factor(kv.e, v, d)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		dm, err := t.getDegreeOf(kv.e, v)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if dm == d {
			e = append(e, smtArithCoeffExpr{kv.coeff, f})
		} else {
			r = append(r, smtArithCoeffExpr{kv.coeff, f})
		}
	}
	s, err := t.crossNested(e, ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(r) != 0 {
		q, err := t.horner(r, v)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if s, err = t.mkApp(&t.nlNewExprs, smtArithOpAdd, q, s); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	result := s
	if d != 0 {
		xd, err := t.power(v, d)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if result, err = t.mkApp(&t.nlNewExprs, smtArithOpMul, xd, s); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

// crossNested converts p into an equivalent cross nested expression, whose
// interval evaluation is more precise. A non-null v is used for the horner
// extension.
func (t *SMTTheoryArith) crossNested(p []smtArithCoeffExpr, v ast.ASTHandle) (ast.ASTHandle, error) {
	x := &t.nlNewExprs
	if v.IsNull() {
		varinfo, err := t.getPolynomialInfo(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if len(varinfo) == 0 {
			return t.p2expr(p)
		}
		v = varinfo[0].v
		max := varinfo[0].occs
		for _, vi := range varinfo[1:] {
			if vi.occs > max {
				v, max = vi.v, vi.occs
			}
		}
	}
	i1, a, n, i2, b, nm, ok, err := t.inMonovariateMonomials(p, v)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if ok {
		if nm < n {
			n, nm = nm, n
			a, b = b, a
		}
		m := nm - n
		if n%2 == m%2 && n >= m {
			// b*x^{n-m}*[(x^{m} + a/(2b))^2 - (a/2b)^2]
			// b*[(x^{m} + a/(2b))^2 - (a/2b)^2]  for n == m
			xm, err := t.power(v, m)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			a2b := new(big.Rat).Quo(a, new(big.Rat).Mul(big.NewRat(2, 1), b))
			isIntV, err := t.isIntSort(v)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			// a numeral of sort int cannot be a non-integer rational.
			if !isIntV || a2b.IsInt() {
				ma2b2 := new(big.Rat).Mul(a2b, a2b)
				ma2b2.Neg(ma2b2)
				a2bn, err := t.mkNumeral(x, a2b, isIntV)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				xmA2b, err := t.mkApp(x, smtArithOpAdd, a2bn, xm)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				xmA2b2, err := t.mkApp(x, smtArithOpMul, xmA2b, xmA2b)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				ma2b2n, err := t.mkNumeral(x, ma2b2, isIntV)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				rhs, err := t.mkApp(x, smtArithOpAdd, xmA2b2, ma2b2n)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				rhs2 := rhs
				if n > m {
					pw, err := t.power(v, n-m)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					if rhs2, err = t.mkApp(x, smtArithOpMul, pw, rhs); err != nil {
						return ast.ASTHandle{}, err
					}
				}
				newExpr := rhs2
				if !smtRatIsOne(b) {
					bn, err := t.mkNumeral(x, b, isIntV)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					if newExpr, err = t.mkApp(x, smtArithOpMul, bn, rhs2); err != nil {
						return ast.ASTHandle{}, err
					}
				}
				var rest []smtArithCoeffExpr
				for i := range p {
					if i != i1 && i != i2 {
						rest = append(rest, p[i])
					}
				}
				if len(rest) == 0 {
					return newExpr, nil
				}
				h, err := t.crossNested(rest, ast.ASTHandle{})
				if err != nil {
					return ast.ASTHandle{}, err
				}
				return t.mkApp(x, smtArithOpAdd, newExpr, h)
			}
		}
	}
	return t.horner(p, v)
}

// isCrossNestedConsistentPoly checks p against the known bounds using its
// cross nested forms.
func (t *SMTTheoryArith) isCrossNestedConsistentPoly(p []smtArithCoeffExpr) (bool, error) {
	varinfo, err := t.getPolynomialInfo(p)
	if err != nil || len(varinfo) == 0 {
		return err == nil, err
	}
	// std::stable_sort by var_num_occs_lt: more occurrences first.
	sort.SliceStable(varinfo, func(i, j int) bool { return varinfo[i].occs > varinfo[j].occs })
	for _, kv := range varinfo {
		if err = t.nlNewExprs.reset(); err != nil {
			return false, err
		}
		cn, err := t.crossNested(p, kv.v)
		if err != nil {
			return false, err
		}
		if cn.IsNull() {
			continue
		}
		i, err := t.evaluateAsInterval(cn)
		if err != nil {
			return false, err
		}
		var d *smtArithmeticBoundDependency
		if !i.minusInfinity() && (i.lower.value.Sign() > 0 || (i.lower.value.Sign() == 0 && i.lowerOpen)) {
			d = i.lowerDep
		} else if !i.plusInfinity() && (i.upper.value.Sign() < 0 || (i.upper.value.Sign() == 0 && i.upperOpen)) {
			d = i.upperDep
		}
		if d != nil {
			return false, t.setConflictDependency(d)
		}
	}
	return true, nil
}

// isCrossNestedConsistentRow checks the polynomial of the row.
func (t *SMTTheoryArith) isCrossNestedConsistentRow(r *smtArithmeticRow) (bool, error) {
	problematic, err := t.isProblematicNonLinearRow(r)
	if err != nil || !problematic {
		return err == nil, err
	}
	coercions, err := t.state.manager.IntRealCoercions()
	if err != nil {
		return false, err
	}
	if !coercions && t.isMixedRealInteger(r) {
		return true, nil // giving up: sort-incorrect expressions (see the source comment)
	}
	c := big.NewRat(1, 1)
	if t.isIntegerRow(r) {
		c.SetInt(r.denominatorLCM())
	}
	var p []smtArithCoeffExpr
	for i := range r.entries {
		it := &r.entries[i]
		if !it.dead() {
			p = append(p, smtArithCoeffExpr{new(big.Rat).Mul(&it.coefficient, c), t.var2expr(it.variable)})
		}
	}
	return t.isCrossNestedConsistentPoly(p)
}

// isCrossNestedConsistentCluster checks the rows of the non linear cluster.
func (t *SMTTheoryArith) isCrossNestedConsistentCluster(nlCluster []SMTTheoryVar) (bool, error) {
	for _, v := range nlCluster {
		if !t.isBase(v) {
			continue
		}
		t.stats.nlCrossNested++
		ok, err := t.isCrossNestedConsistentRow(&t.rows[t.getVarRow(v)])
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// ---- nl.h:1733-2461 (grobner basis and process_non_linear) ----

// Variable weights of init_grobner_var_order.
const (
	smtArithGbFixed            = 0
	smtArithGbQuotedFixed      = 1
	smtArithGbBounded          = 2
	smtArithGbQuotedBounded    = 3
	smtArithGbNotFree          = 4
	smtArithGbQuotedNotFree    = 5
	smtArithGbFree             = 6
	smtArithGbQuotedFree       = 7
	smtArithGbMaxDefaultWeight = 7
)

// smtArithGbDeps carries v_dependency values through the grobner solver:
// each bound is a leaf holding its index in bounds (the source's bound
// pointer), translated back to bound dependencies when a conflict or a new
// bound needs a justification.
type smtArithGbDeps struct {
	t      *SMTTheoryArith
	bounds []smtArithBound
	index  map[smtArithBound]int
}

func (d *smtArithGbDeps) close() error { return nil }

// leaf is m_dep_manager.mk_leaf(b): the leaf holds the bound's index.
func (d *smtArithGbDeps) leaf(b smtArithBound) (*grobner.Dependency, error) {
	i, found := d.index[b]
	if !found {
		i = len(d.bounds)
		d.bounds = append(d.bounds, b)
		d.index[b] = i
	}
	return grobner.DependencyLeaf(i), nil
}

// join is m_dep_manager.mk_join(a, b).
func (d *smtArithGbDeps) join(a, b *grobner.Dependency) (*grobner.Dependency, error) {
	return grobner.DependencyJoin(a, b), nil
}

// toBoundDependency translates a grobner dependency to the bound dependency
// with the same leaves, each bound once in linearize order.
func (d *smtArithGbDeps) toBoundDependency(dep *grobner.Dependency) (*smtArithmeticBoundDependency, error) {
	var result *smtArithmeticBoundDependency
	seen := make(map[int]bool)
	for _, i := range grobner.Linearize(dep) {
		if i < 0 || i >= len(d.bounds) {
			return nil, ast.ErrASTDomain
		}
		if seen[i] {
			continue
		}
		seen[i] = true
		leaf := d.t.mkLeaf(d.bounds[i])
		if result == nil {
			result = leaf
		} else {
			result = d.t.depManager.Join(result, leaf)
		}
	}
	return result, nil
}

// initGrobnerVarOrder: quoted free > free > quoted not free > not free >
// quoted bounded > bounded > quoted fixed > fixed.
func (t *SMTTheoryArith) initGrobnerVarOrder(nlCluster []SMTTheoryVar, gb *grobner.Solver) error {
	for _, v := range nlCluster {
		e := t.var2expr(v)
		quoted := t.isPureMonomial(e)
		weight := smtArithGbFree
		switch {
		case t.isFixed(v):
			weight = smtArithGbFixed
		case t.isBounded(v):
			weight = smtArithGbBounded
		case t.lower(v) != nil || t.upper(v) != nil:
			weight = smtArithGbNotFree
		}
		if quoted {
			weight++
		}
		if err := gb.SetWeight(e, weight); err != nil {
			return err
		}
	}
	return nil
}

// mkGbMonomial creates coeff*m with fixed variables replaced by their
// values; dep and alreadyFound collect the fixed variables used. It returns
// nil when the monomial simplifies to 0.
func (t *SMTTheoryArith) mkGbMonomial(c *big.Rat, m ast.ASTHandle, gb *grobner.Solver, deps *smtArithGbDeps, dep **grobner.Dependency, alreadyFound map[SMTTheoryVar]bool) (*grobner.Monomial, error) {
	var vars []ast.ASTHandle
	coeff := new(big.Rat).Set(c)
	procVar := func(v ast.ASTHandle) error {
		if r, ok := t.isNumeral(v); ok {
			coeff.Mul(coeff, r)
			return nil
		}
		w := t.expr2var(v)
		if !t.isFixed(w) {
			vars = append(vars, v)
			return nil
		}
		if !alreadyFound[w] {
			alreadyFound[w] = true
			l, err := deps.leaf(t.lower(w))
			if err != nil {
				return err
			}
			u, err := deps.leaf(t.upper(w))
			if err != nil {
				return err
			}
			lu, err := deps.join(l, u)
			if err != nil {
				return err
			}
			if *dep, err = deps.join(*dep, lu); err != nil {
				return err
			}
		}
		coeff.Mul(coeff, &t.lowerBound(w).rational)
		return nil
	}
	if t.isOp(m, smtArithOpMul) {
		mc, err := t.getMonomialCoeff(m)
		if err != nil {
			return nil, err
		}
		coeff.Mul(coeff, mc)
		if m, err = t.getMonomialBody(m); err != nil {
			return nil, err
		}
		if t.isOp(m, smtArithOpMul) {
			args, err := t.args(m)
			if err != nil {
				return nil, err
			}
			for _, arg := range args {
				if err = procVar(arg); err != nil {
					return nil, err
				}
			}
		} else if err = procVar(m); err != nil {
			return nil, err
		}
	} else if err := procVar(m); err != nil {
		return nil, err
	}
	if coeff.Sign() == 0 {
		return nil, nil
	}
	return gb.NewMonomial(coeff, vars)
}

// addRowToGb sends the row to the grobner object with fixed variables
// substituted.
func (t *SMTTheoryArith) addRowToGb(r *smtArithmeticRow, gb *grobner.Solver, deps *smtArithGbDeps) error {
	var monomials []*grobner.Monomial
	var dep *grobner.Dependency
	alreadyFound := make(map[SMTTheoryVar]bool) // m_tmp_var_set
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		newM, err := t.mkGbMonomial(&it.coefficient, t.var2expr(it.variable), gb, deps, &dep, alreadyFound)
		if err != nil {
			return err
		}
		if newM != nil {
			monomials = append(monomials, newM)
		}
	}
	return gb.AssertMonomialsZero(monomials, dep)
}

// addMonomialDefToGb adds (quote (* x_1 ... x_n)) = x_1 * ... * x_n for the
// pure monomial v, with fixed variables substituted.
func (t *SMTTheoryArith) addMonomialDefToGb(v SMTTheoryVar, gb *grobner.Solver, deps *smtArithGbDeps) error {
	var monomials []*grobner.Monomial
	var dep *grobner.Dependency
	alreadyFound := make(map[SMTTheoryVar]bool)
	m := t.var2expr(v)
	newM, err := t.mkGbMonomial(big.NewRat(1, 1), m, gb, deps, &dep, alreadyFound)
	if err != nil {
		return err
	}
	if newM != nil {
		monomials = append(monomials, newM)
	}
	coeff := big.NewRat(-1, 1)
	if t.isFixed(v) {
		l, err := deps.leaf(t.lower(v))
		if err != nil {
			return err
		}
		u, err := deps.leaf(t.upper(v))
		if err != nil {
			return err
		}
		lu, err := deps.join(l, u)
		if err != nil {
			return err
		}
		if dep, err = deps.join(dep, lu); err != nil {
			return err
		}
		coeff.Mul(coeff, &t.lowerBound(v).rational)
		if coeff.Sign() != 0 {
			c, err := gb.NewMonomial(coeff, nil)
			if err != nil {
				return err
			}
			monomials = append(monomials, c)
		}
	} else {
		c, err := gb.NewMonomial(coeff, []ast.ASTHandle{m})
		if err != nil {
			return err
		}
		monomials = append(monomials, c)
	}
	return gb.AssertMonomialsZero(monomials, dep)
}

// initGrobner initializes the grobner object from the rows and monomials of
// the non linear cluster.
func (t *SMTTheoryArith) initGrobner(nlCluster []SMTTheoryVar, gb *grobner.Solver, deps *smtArithGbDeps) error {
	if err := t.initGrobnerVarOrder(nlCluster, gb); err != nil {
		return err
	}
	for _, v := range nlCluster {
		if t.isBase(v) {
			if err := t.addRowToGb(&t.rows[t.getVarRow(v)], gb, deps); err != nil {
				return err
			}
		}
		if t.isPureMonomial(t.var2expr(v)) && !t.data[v].nonlinearPropagated && t.isFixed(v) {
			if err := t.addMonomialDefToGb(v, gb, deps); err != nil {
				return err
			}
		}
	}
	return nil
}

// mkIntervalForMonomial returns the interval of a grobner monomial.
func (t *SMTTheoryArith) mkIntervalForMonomial(m *grobner.Monomial) (*smtOldInterval, error) {
	r, err := newSMTOldIntervalPoint(&t.depManager, m.Coefficient(), nil, nil)
	if err != nil {
		return nil, err
	}
	var v ast.ASTHandle
	hasVar := false
	power := uint32(0)
	numVars := m.Degree()
	for i := uint32(0); i < numVars; i++ {
		curr := m.Variable(i)
		switch {
		case !hasVar:
			v, hasVar, power = curr, true, 1
		case curr == v:
			power++
		default:
			if err = t.mulBoundOf(v, power, r); err != nil {
				return nil, err
			}
			v, power = curr, 1
		}
	}
	if hasVar {
		if err = t.mulBoundOf(v, power, r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// setConflictDependency is set_conflict(v_dependency*).
func (t *SMTTheoryArith) setConflictDependency(d *smtArithmeticBoundDependency) error {
	ante := t.acquireAntecedents()
	defer t.releaseAntecedentsScope()
	b := &smtArithmeticDerivedBound{}
	b.variable, b.kind = SMTNullTheoryVar, smtArithmeticLowerBound
	if err := t.dependency2NewBound(d, b); err != nil {
		return err
	}
	return t.setConflictDerived(b, ante, "arith_nl")
}

// isInconsistentInterval reports whether I.lower <= -M_1 - ... - M_n <=
// I.upper is inconsistent, setting a conflict from the variable bounds.
func (t *SMTTheoryArith) isInconsistentInterval(I *smtOldInterval, monomials []*grobner.Monomial, dep *smtArithmeticBoundDependency) (bool, error) {
	r := I.clone()
	for _, m := range monomials {
		mi, err := t.mkIntervalForMonomial(m)
		if err != nil {
			return false, err
		}
		if err = r.add(mi); err != nil {
			return false, err
		}
		if r.minusInfinity() && r.plusInfinity() {
			return false, nil
		}
	}
	var intervalDeps *smtArithmeticBoundDependency
	conflict := false
	if !r.minusInfinity() && (r.lower.value.Sign() > 0 || (r.lower.value.Sign() == 0 && r.lowerOpen)) {
		intervalDeps, conflict = r.lowerDep, true
	} else if !r.plusInfinity() && (r.upper.value.Sign() < 0 || (r.upper.value.Sign() == 0 && r.upperOpen)) {
		intervalDeps, conflict = r.upperDep, true
	}
	// intervalDeps may be nil even when r does not contain 0.
	if conflict {
		joined := intervalDeps
		if joined == nil {
			joined = dep
		} else if dep != nil {
			joined = t.depManager.Join(intervalDeps, dep)
		}
		return true, t.setConflictDependency(joined)
	}
	return false, nil
}

// isInconsistentEq reports whether the equation is inconsistent, setting a
// conflict.
func (t *SMTTheoryArith) isInconsistentEq(eq *grobner.Equation, deps *smtArithGbDeps) (bool, error) {
	zero, err := newSMTOldIntervalPoint(&t.depManager, new(big.Rat), nil, nil)
	if err != nil {
		return false, err
	}
	dep, err := deps.toBoundDependency(eq.Dependency())
	if err != nil {
		return false, err
	}
	return t.isInconsistentInterval(zero, eq.Monomials(), dep)
}

// smtRatIsPerfectSquare is rational::is_perfect_square.
func smtRatIsPerfectSquare(q *big.Rat) (*big.Rat, bool) {
	if q.Sign() < 0 {
		return nil, false
	}
	n := new(big.Int).Sqrt(q.Num())
	if new(big.Int).Mul(n, n).Cmp(q.Num()) != 0 {
		return nil, false
	}
	d := new(big.Int).Sqrt(q.Denom())
	if new(big.Int).Mul(d, d).Cmp(q.Denom()) != 0 {
		return nil, false
	}
	return new(big.Rat).SetFrac(n, d), true
}

// smtArithIsPerfectSquareMonomial: c*M is a square; r is the root of c.
func smtArithIsPerfectSquareMonomial(m *grobner.Monomial) (*big.Rat, bool) {
	numVars := m.Degree()
	if numVars%2 == 1 {
		return nil, false
	}
	r, ok := smtRatIsPerfectSquare(m.Coefficient())
	if !ok {
		return nil, false
	}
	var v ast.ASTHandle
	hasVar := false
	power := uint32(0)
	for i := uint32(0); i < numVars; i++ {
		curr := m.Variable(i)
		switch {
		case !hasVar:
			v, hasVar, power = curr, true, 1
		case v == curr:
			power++
		default:
			if power%2 == 1 {
				return nil, false
			}
			v, power = curr, 1
		}
	}
	return r, power%2 == 0
}

// smtArithIsPerfectSquareTriple: m1m2 is (-2ab)*M1*M2 for m1Sq = a^2*M1*M1
// and m2Sq = b^2*M2*M2.
func smtArithIsPerfectSquareTriple(m1Sq *grobner.Monomial, a *big.Rat, m2Sq *grobner.Monomial, b *big.Rat, m1m2 *grobner.Monomial) bool {
	coeff := m1m2.Coefficient()
	if coeff.Sign() >= 0 {
		return false
	}
	c := big.NewRat(-2, 1)
	c.Mul(c, a)
	c.Mul(c, b)
	if coeff.Cmp(c) != 0 {
		return false
	}
	num1, num2, num12 := m1Sq.Degree(), m2Sq.Degree(), m1m2.Degree()
	if num1+num2 != num12*2 {
		return false
	}
	var i1, i2, i12 uint32
	for {
		var v1, v2, v12 ast.ASTHandle
		if i1 < num1 {
			v1 = m1Sq.Variable(i1)
		}
		if i2 < num2 {
			v2 = m2Sq.Variable(i2)
		}
		if i12 < num12 {
			v12 = m1m2.Variable(i12)
		}
		if v1.IsNull() && v2.IsNull() && v12.IsNull() {
			return true
		}
		if v12.IsNull() {
			return false
		}
		if v1 == v12 {
			i1 += 2
			i12++
		} else if v2 == v12 {
			i2 += 2
			i12++
		} else {
			return false
		}
	}
}

// isInconsistent2 eliminates perfect squares (M1 - M2)^2, replacing them
// with [0, oo) when that is more precise, and checks the equation.
func (t *SMTTheoryArith) isInconsistent2(eq *grobner.Equation, deps *smtArithGbDeps) (bool, error) {
	ms := eq.Monomials()
	num := len(ms)
	intervals := make([]*smtOldInterval, num)
	for i, m := range ms {
		it, err := t.mkIntervalForMonomial(m)
		if err != nil {
			return false, err
		}
		intervals[i] = it
	}
	deleted := make([]bool, num)
	var monomials []*grobner.Monomial
	for i := 0; i < num; i++ {
		m1 := ms[i]
		if deleted[i] {
			continue
		}
		a, ok := smtArithIsPerfectSquareMonomial(m1)
		if !ok {
			monomials = append(monomials, m1)
			continue
		}
		j := i + 1
		for ; j < num; j++ {
			if deleted[j] {
				continue
			}
			m2 := ms[j]
			b, ok := smtArithIsPerfectSquareMonomial(m2)
			if !ok {
				continue
			}
			// -2*root(m1)*root(m2) is smaller than m1.
			k := i + 1
			for ; k < num; k++ {
				if deleted[k] || !smtArithIsPerfectSquareTriple(m1, a, m2, b, ms[k]) {
					continue
				}
				I := intervals[i].clone()
				if err := I.add(intervals[j]); err != nil {
					return false, err
				}
				if err := I.add(intervals[k]); err != nil {
					return false, err
				}
				if I.minusInfinity() || I.lower.value.Sign() < 0 {
					// the lower bound improves when the square is eliminated
					deleted[i], deleted[j], deleted[k] = true, true, true
					break
				}
			}
			if k < num {
				break // found perfect square
			}
		}
		if j == num {
			// didn't find a perfect square (M1-M2)^2
			monomials = append(monomials, m1)
		}
	}
	if len(monomials) == num {
		return false, nil // didn't find any perfect square.
	}
	geZero, err := newSMTOldIntervalHalfBound(&t.depManager, new(big.Rat), false, true, nil)
	if err != nil {
		return false, err
	}
	dep, err := deps.toBoundDependency(eq.Dependency())
	if err != nil {
		return false, err
	}
	return t.isInconsistentInterval(geZero, monomials, dep)
}

func (t *SMTTheoryArith) monomial2expr(x *smtArithExprs, m *grobner.Monomial, isInt bool) (ast.ASTHandle, error) {
	var args []ast.ASTHandle
	if c := m.Coefficient(); !smtRatIsOne(c) {
		cn, err := t.mkNumeral(x, c, isInt)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args = append(args, cn)
	}
	args = append(args, m.Variables()...)
	return t.mkNaryMul(x, args, isInt)
}

// internalizeGbEq asserts the new equation in the simplex tableau.
func (t *SMTTheoryArith) internalizeGbEq(eq *grobner.Equation, deps *smtArithGbDeps) (result bool, err error) {
	isInt := false
	ms := eq.Monomials()
	for _, m := range ms {
		degree := m.Degree()
		if degree > t.params.NLArithMaxDegree {
			return false, nil
		}
		if degree > 0 {
			if isInt, err = t.isIntSort(m.Variable(0)); err != nil {
				return false, err
			}
		}
	}
	x := smtArithExprs{m: t.state.manager}
	defer func() { err = errors.Join(err, x.release()) }()
	k := new(big.Rat)
	var args []ast.ASTHandle
	for _, m := range ms {
		if m.Degree() == 0 {
			k.Sub(k, m.Coefficient())
			continue
		}
		e, err := t.monomial2expr(&x, m, isInt)
		if err != nil {
			return false, err
		}
		args = append(args, e)
	}
	pol, err := t.mkNaryAdd(&x, args, false)
	if err != nil {
		return false, err
	}
	sPol, err := t.rewrite(&x, pol)
	if err != nil {
		return false, err
	}
	if !t.hasVar(sPol) {
		if err = t.state.Internalize(sPol, false); err != nil {
			return false, err
		}
		if err = t.state.markRelevant(sPol); err != nil {
			return false, err
		}
	}
	// s_pol = k
	v := t.expr2var(sPol)
	if v == SMTNullTheoryVar {
		return false, ast.ErrASTDomain
	}
	dep, err := deps.toBoundDependency(eq.Dependency())
	if err != nil {
		return false, err
	}
	var kv smtArithmeticValue
	kv.setRational(k)
	if err = t.mkDerivedNlBound(v, &kv, smtArithmeticLowerBound, dep); err != nil {
		return false, err
	}
	return true, t.mkDerivedNlBound(v, &kv, smtArithmeticUpperBound, dep)
}

type smtArithGbResult uint8

const (
	smtArithGbProgress smtArithGbResult = iota
	smtArithGbNewEq
	smtArithGbFail
)

// computeGrobner reports progress when a conflict or new fixed variables
// were found.
func (t *SMTTheoryArith) computeGrobner(nlCluster []SMTTheoryVar) (result smtArithGbResult, err error) {
	if t.nlGbExhausted {
		return smtArithGbFail, nil
	}
	gb, err := grobner.New(t.state.manager)
	if err != nil {
		return smtArithGbFail, err
	}
	deps := &smtArithGbDeps{t: t, index: make(map[smtArithBound]int)}
	defer func() { err = errors.Join(err, gb.Close(), deps.close()) }()
	if err = t.initGrobner(nlCluster, gb, deps); err != nil {
		return smtArithGbFail, err
	}
	warn := false
	nextWeight := smtArithGbMaxDefaultWeight + 1 // next weight used during perturbation.
	for {
		r := false
		if err = gb.ComputeBasisInit(); err != nil {
			return smtArithGbFail, err
		}
		for !r && gb.NumNewEquations() < t.params.NLArithGBThreshold {
			if t.cancelFlag() {
				warn = true
				break
			}
			if r, err = gb.ComputeBasisStep(); err != nil {
				return smtArithGbFail, err
			}
		}
		t.stats.gbSimplify += uint32(gb.Stats.Simplify)
		t.stats.gbSuperpose += uint32(gb.Stats.Superpose)
		t.stats.gbNumProcessed += uint32(gb.Stats.NumProcessed)
		t.stats.gbComputeBasis++
		if !r && !warn {
			// m_nl_gb_exhausted is restored by the scope record (value_trail).
			t.nlGbExhausted = true
			warn = true
		}
		if t.cancelFlag() {
			return smtArithGbFail, nil
		}
		// Scan the basis for inconsistencies.
		eqs := gb.Equations()
		for _, eq := range eqs {
			if inc, err := t.isInconsistentEq(eq, deps); err != nil || inc {
				return smtArithGbProgress, err
			}
			if inc, err := t.isInconsistent2(eq, deps); err != nil || inc {
				return smtArithGbProgress, err
			}
		}
		// Equations x - k = 0 or x = 0 with x not fixed bound x.
		result = smtArithGbFail
		if t.params.NLArithGBEqs {
			for _, eq := range eqs {
				if !eq.IsLinearCombination() {
					ok, err := t.internalizeGbEq(eq, deps)
					if err != nil {
						return smtArithGbFail, err
					}
					if ok {
						result = smtArithGbNewEq
					}
				}
			}
		}
		if result != smtArithGbFail || !t.params.NLArithGBPerturbate || t.nlGbExhausted {
			return result, nil
		}
		// Change the variable order so the leading term changes. Only linear
		// equations are considered, and a weight changes at most once.
		modified := false
		for _, eq := range eqs {
			numMonomials := eq.NumMonomials()
			if numMonomials == 0 {
				continue // the equation 0 = 0 should have been discarded.
			}
			if eq.Monomial(0).Degree() != 1 {
				continue
			}
			for j := uint32(1); j < numMonomials; j++ {
				m := eq.Monomial(j)
				if m.Degree() != 1 {
					continue
				}
				v := m.Variable(0)
				if gb.Weight(v) > smtArithGbMaxDefaultWeight {
					continue // variable was already updated
				}
				if err = gb.SetWeight(v, nextWeight); err != nil {
					return smtArithGbFail, err
				}
				nextWeight++
				if err = gb.UpdateOrder(); err != nil {
					return smtArithGbFail, err
				}
				modified = true
				break
			}
			if modified {
				break
			}
		}
		if !modified {
			return result, nil
		}
	}
}

// maxMinNlVars maximizes/minimizes the variables of the monomials.
func (t *SMTTheoryArith) maxMinNlVars() (bool, error) {
	alreadyFound := make(map[SMTTheoryVar]bool)
	var vars []SMTTheoryVar
	for _, v := range t.nlMonomials {
		t.markVar(v, &vars, alreadyFound)
		args, err := t.args(t.var2expr(v))
		if err != nil {
			return false, err
		}
		for _, curr := range args {
			t.markVar(t.expr2var(curr), &vars, alreadyFound)
		}
	}
	return t.maxMinVars(vars)
}

// processNonLinear processes the non linear constraints.
func (t *SMTTheoryArith) processNonLinear() (SMTFinalCheckStatus, error) {
	t.modelDependsOnComputedEpsilon = false
	if len(t.nlMonomials) == 0 {
		return SMTFinalCheckDone, nil
	}
	ok, err := t.checkMonomialAssignments()
	if err != nil || ok {
		return SMTFinalCheckDone, err
	}
	if !t.params.NLArith {
		return SMTFinalCheckGiveUp, nil
	}
	if t.nlRounds > t.params.NLArithRounds {
		return SMTFinalCheckGiveUp, nil
	}
	// m_nl_rounds is restored by the scope record (value_trail).
	t.nlRounds++
	if err = t.elimQuasiBaseRows(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if err = t.moveNonBaseVarsToBounds(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if ok, err = t.makeFeasible(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if !ok {
		return SMTFinalCheckContinue, t.failed()
	}
	if ok, err = t.maxMinNlVars(); err != nil || !ok {
		return SMTFinalCheckContinue, err
	}
	done := func() SMTFinalCheckStatus {
		if t.liberalFinalCheck || !t.changedAssignment {
			return SMTFinalCheckDone
		}
		return SMTFinalCheckContinue
	}
	if ok, err = t.checkMonomialAssignments(); err != nil || ok {
		return done(), err
	}
	var vars []SMTTheoryVar
	if err = t.getNonLinearCluster(&vars); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	oldIdx := t.nlStrategyIdx
	// m_nl_strategy_idx is restored by the scope record (value_trail).
	for {
		progress := false
		switch t.nlStrategyIdx {
		case 0:
			p, err := t.propagateNlBounds()
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if p {
				if _, err = t.propagateCore(); err != nil {
					return SMTFinalCheckGiveUp, err
				}
				progress = true
			}
		case 1:
			consistent, err := t.isCrossNestedConsistentCluster(vars)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if !consistent {
				progress = true
			}
		case 2:
			if t.params.NLArithGB {
				r, err := t.computeGrobner(vars)
				if err != nil {
					return SMTFinalCheckGiveUp, err
				}
				switch r {
				case smtArithGbProgress:
					progress = true
				case smtArithGbNewEq:
					progress = true
					if _, err = t.propagateCore(); err != nil {
						return SMTFinalCheckGiveUp, err
					}
				}
			}
		case 3:
			if t.params.NLArithBranching {
				target, err := t.findNlVarForBranching()
				if err != nil {
					return SMTFinalCheckGiveUp, err
				}
				if target != SMTNullTheoryVar {
					branched, err := t.branchNlIntVar(target)
					if err != nil {
						return SMTFinalCheckGiveUp, err
					}
					if branched {
						progress = true
					}
				}
			}
		}
		t.nlStrategyIdx = (t.nlStrategyIdx + 1) % 4
		if progress {
			return SMTFinalCheckContinue, nil
		}
		if t.nlStrategyIdx == oldIdx {
			break
		}
	}
	if ok, err = t.checkMonomialAssignments(); err != nil || ok {
		return done(), err
	}
	return SMTFinalCheckGiveUp, nil
}
