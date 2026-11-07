package smtcore

// Source: src/smt/theory_arith.h (class layout) and
// src/smt/theory_arith_core.h:1610-2540 (assignment, add_row, pivoting,
// make_feasible, bound assertion). One engine serves theory_mi_arith and
// theory_i_arith: the two differ only in the epsilons and in whether
// infinitesimals can occur, which smtArithmeticValue already covers.

import (
	"errors"
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/heap"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/natset"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

const (
	smtArithBoundPropNone = 0 // BP_NONE
	smtArithPivotSmallest = 0 // ARITH_PIVOT_SMALLEST
	smtArithPivotGreatest = 1 // ARITH_PIVOT_GREATEST_ERROR
	smtArithPivotLeast    = 2 // ARITH_PIVOT_LEAST_ERROR
)

var errSMTArithEngineInvariant = errors.New("smtcore: arithmetic engine invariant violated")

// smtArithEngineBound is the polymorphic bound class: atom, eq_bound,
// derived_bound and justified_derived_bound. Every concrete bound is
// justified, as in the source.
type smtArithEngineBound interface {
	smtArithmeticBoundJustification
	core() *smtArithmeticBound
}

func (b *smtArithmeticBound) core() *smtArithmeticBound { return b }

// smtArithEngineHost is the part of smt::context the engine calls.
type smtArithEngineHost interface {
	// assignment is context::get_assignment(bool_var).
	assignment(SMTBoolVar) lbool.LBool
	// setConflict is theory_arith::set_conflict(...), ending in ctx.set_conflict.
	setConflict(ante *smtArithmeticAntecedents, rule string) error
	// assignBoundLiteral assigns l, justified by ante (assign_bound_literal).
	assignBoundLiteral(l SMTLiteral, ante *smtArithmeticAntecedents) error
	// mkAuxLemma adds the clause lits as an auxiliary lemma (ctx.mk_clause
	// with CLS_AUX_LEMMA); ante supplies the proof parameters.
	mkAuxLemma(lits []SMTLiteral, ante *smtArithmeticAntecedents) error
	// isIntSource is is_int_src: the variable's expression has sort Int.
	isIntSource(SMTTheoryVar) bool
	// isEqual is is_equal: the variables' enodes share a root.
	isEqual(x, y SMTTheoryVar) bool
	// propagateEqToCore is propagate_eq_to_core: assign x = y, justified by ante.
	propagateEqToCore(x, y SMTTheoryVar, ante *smtArithmeticAntecedents) error
	// limitInc is get_manager().limit().inc(n); it reports whether work
	// remains permitted.
	limitInc(n uint) bool
	cancelled() bool
}

type smtArithEngineStats struct {
	conflicts, addRows, pivots                     uint32
	assertLower, assertUpper, boundProps, fixedEqs uint32
	offsetEqs                                      uint32
}

type smtArithEngineScope struct {
	atomsLim, boundTrailLim, unassignedAtomsTrailLim uint32
	assertedBoundsLim, assertedQhead                 uint32
	boundsToDeleteLim                                uint32
	nlMonomialsLim, nlPropagatedLim                  uint32
	numVars                                          uint32
}

type smtArithEngine struct {
	host   smtArithEngineHost
	params *SMTArithState

	proofs     bool
	boundWatch SMTBoolVar // m_bound_watch, null when objective recording is off

	stats        smtArithEngineStats
	numConflicts uint32

	rows            []smtArithmeticRow
	deadRows        []int
	columns         []smtArithmeticColumn
	data            []smtArithmeticVariableData
	value, oldValue []smtArithmeticValue
	bounds          [2][]smtArithEngineBound
	varOccs         [][]*smtArithmeticAtom
	unassignedAtoms []uint32
	varPos          []int
	atoms           []*smtArithmeticAtom
	boolVarToAtom   map[SMTBoolVar]*smtArithmeticAtom
	assertedBounds  []smtArithEngineBound
	assertedQhead   uint32
	newAtoms        []*smtArithmeticAtom
	nlMonomials     []SMTTheoryVar
	fixedVarTable   map[smtArithFixedKey]SMTTheoryVar
	varOffsetToRow  map[smtArithOffsetKey]int
	tmpLitSet       map[int32]bool
	tmpEqSet        map[SMTEnodePair]bool
	nlPropagated    []SMTTheoryVar

	toPatch    *heap.Int
	leftBasis  *natset.Set
	blandsRule bool

	updateTrailStack   []uint32
	inUpdateTrailStack *natset.Set
	toCheck            []uint32
	inToCheck          *natset.Set

	tmp           smtArithmeticValue
	random        *z3rand.Generator
	eagerGCD      bool
	changedAssign bool

	boundTrail         []smtArithEngineBoundTrail
	unassignedTrail    []uint32
	boundsToDelete     []smtArithEngineBound
	scopes             []smtArithEngineScope
	antecedentsPool    [3]smtArithmeticAntecedents
	antecedentsIndex   int
	intEpsilon         smtArithmeticValue
	realEpsilon        smtArithmeticValue
	integerOnlyEpsilon bool
}

func newSMTArithEngine(host smtArithEngineHost, params *SMTArithState, integerOnly bool) (*smtArithEngine, error) {
	if host == nil || params == nil {
		return nil, ast.ErrASTDomain
	}
	e := &smtArithEngine{
		host:               host,
		params:             params,
		boundWatch:         -1,
		boolVarToAtom:      make(map[SMTBoolVar]*smtArithmeticAtom),
		fixedVarTable:      make(map[smtArithFixedKey]SMTTheoryVar),
		varOffsetToRow:     make(map[smtArithOffsetKey]int),
		tmpLitSet:          make(map[int32]bool),
		tmpEqSet:           make(map[SMTEnodePair]bool),
		leftBasis:          natset.New(0),
		inUpdateTrailStack: natset.New(0),
		inToCheck:          natset.New(0),
		random:             z3rand.New(params.RandomSeed),
		eagerGCD:           params.EagerGCD,
		integerOnlyEpsilon: integerOnly,
	}
	var err error
	e.toPatch, err = heap.New(1024, heap.Config{})
	if err != nil {
		return nil, err
	}
	// mi_ext: int epsilon is 1 and real epsilon is the infinitesimal 1.
	// i_ext: both are the rational 1.
	e.intEpsilon.rational.SetInt64(1)
	if integerOnly {
		e.realEpsilon.rational.SetInt64(1)
	} else {
		e.realEpsilon.infinitesimal.SetInt64(1)
	}
	return e, nil
}

func (e *smtArithEngine) close() error {
	if e.toPatch == nil {
		return nil
	}
	err := e.toPatch.Close()
	e.toPatch = nil
	return err
}

// ---- parameter accessors (theory_arith.h:524-540) ----

func (e *smtArithEngine) coeffsEnabled() bool { return e.proofs || e.boundWatch >= 0 }
func (e *smtArithEngine) relaxBounds() bool   { return e.params.StrongerLemmas }
func (e *smtArithEngine) propagationMode() uint32 {
	if e.numConflicts < e.params.PropagationThreshold {
		return e.params.BoundProp
	}
	return smtArithBoundPropNone
}
func (e *smtArithEngine) numVars() int { return len(e.columns) }

// ---- variable queries (theory_arith.h:543-585) ----

func (e *smtArithEngine) varKind(v SMTTheoryVar) smtArithmeticVariableKind { return e.data[v].kind }
func (e *smtArithEngine) isBase(v SMTTheoryVar) bool                       { return e.data[v].kind == smtArithmeticBase }
func (e *smtArithEngine) isQuasiBase(v SMTTheoryVar) bool {
	return e.data[v].kind == smtArithmeticQuasiBase
}
func (e *smtArithEngine) isNonBase(v SMTTheoryVar) bool {
	return e.data[v].kind == smtArithmeticNonBase
}
func (e *smtArithEngine) varRow(v SMTTheoryVar) int  { return e.data[v].rowID }
func (e *smtArithEngine) isInt(v SMTTheoryVar) bool  { return e.data[v].integer }
func (e *smtArithEngine) isReal(v SMTTheoryVar) bool { return !e.data[v].integer }

func (e *smtArithEngine) getBound(v SMTTheoryVar, upper bool) smtArithEngineBound {
	if upper {
		return e.bounds[1][v]
	}
	return e.bounds[0][v]
}
func (e *smtArithEngine) lower(v SMTTheoryVar) smtArithEngineBound { return e.bounds[0][v] }
func (e *smtArithEngine) upper(v SMTTheoryVar) smtArithEngineBound { return e.bounds[1][v] }

func (e *smtArithEngine) epsilon(v SMTTheoryVar) *smtArithmeticValue {
	if e.isReal(v) {
		return &e.realEpsilon
	}
	return &e.intEpsilon
}

// impliedValue is get_implied_value: the value of a base or quasi-base
// variable computed from its row. The result aliases e.tmp, as in the source.
func (e *smtArithEngine) impliedValue(v SMTTheoryVar) *smtArithmeticValue {
	sum := &e.tmp
	sum.reset()
	row := &e.rows[e.varRow(v)]
	for i := range row.entries {
		entry := &row.entries[i]
		if !entry.dead() && entry.variable != v {
			sum.addMul(&entry.coefficient, &e.value[entry.variable])
		}
	}
	sum.neg()
	return sum
}

// impliedOldValue is get_implied_old_value.
func (e *smtArithEngine) impliedOldValue(v SMTTheoryVar, result *smtArithmeticValue) bool {
	diff := false
	result.reset()
	row := &e.rows[e.varRow(v)]
	for i := range row.entries {
		entry := &row.entries[i]
		if entry.dead() || entry.variable == v {
			continue
		}
		v2 := entry.variable
		if e.inUpdateTrailStack.Contains(uint32(v2)) {
			result.addMul(&entry.coefficient, &e.oldValue[v2])
			diff = true
		} else {
			result.addMul(&entry.coefficient, &e.value[v2])
		}
	}
	result.neg()
	return diff
}

func (e *smtArithEngine) getValue(v SMTTheoryVar) *smtArithmeticValue {
	if e.isQuasiBase(v) {
		return e.impliedValue(v)
	}
	return &e.value[v]
}

func (e *smtArithEngine) belowLower(v SMTTheoryVar) bool {
	l := e.lower(v)
	return l != nil && e.getValue(v).cmp(&l.core().value) < 0
}
func (e *smtArithEngine) aboveUpper(v SMTTheoryVar) bool {
	u := e.upper(v)
	return u != nil && e.getValue(v).cmp(&u.core().value) > 0
}
func (e *smtArithEngine) belowUpper(v SMTTheoryVar) bool {
	u := e.upper(v)
	return u == nil || e.getValue(v).cmp(&u.core().value) < 0
}
func (e *smtArithEngine) aboveLower(v SMTTheoryVar) bool {
	l := e.lower(v)
	return l == nil || e.getValue(v).cmp(&l.core().value) > 0
}
func (e *smtArithEngine) atLower(v SMTTheoryVar) bool {
	l := e.lower(v)
	return l != nil && e.getValue(v).cmp(&l.core().value) == 0
}
func (e *smtArithEngine) atUpper(v SMTTheoryVar) bool {
	u := e.upper(v)
	return u != nil && e.getValue(v).cmp(&u.core().value) == 0
}
func (e *smtArithEngine) atBound(v SMTTheoryVar) bool { return e.atLower(v) || e.atUpper(v) }
func (e *smtArithEngine) isFree(v SMTTheoryVar) bool  { return e.lower(v) == nil && e.upper(v) == nil }
func (e *smtArithEngine) isNonFree(v SMTTheoryVar) bool {
	return e.lower(v) != nil || e.upper(v) != nil
}

// isFixed is theory_arith_aux.h is_fixed: both bounds present and equal.
func (e *smtArithEngine) isFixed(v SMTTheoryVar) bool {
	l, u := e.lower(v), e.upper(v)
	return l != nil && u != nil && l.core().value.cmp(&u.core().value) == 0
}

// ---- assignment management (core.h:1828-1903) ----

func (e *smtArithEngine) saveValue(v SMTTheoryVar) {
	if !e.inUpdateTrailStack.Contains(uint32(v)) {
		e.inUpdateTrailStack.Insert(uint32(v))
		e.oldValue[v].set(&e.value[v])
		e.updateTrailStack = append(e.updateTrailStack, uint32(v))
	}
	e.changedAssign = true
}

func (e *smtArithEngine) discardUpdateTrail() {
	e.inUpdateTrailStack.Reset()
	e.updateTrailStack = e.updateTrailStack[:0]
}

func (e *smtArithEngine) restoreAssignment() {
	for _, v := range e.updateTrailStack {
		e.value[v].set(&e.oldValue[v])
	}
	e.updateTrailStack = e.updateTrailStack[:0]
	e.inUpdateTrailStack.Reset()
}

// updateValueCore is m_value[v] += delta.
func (e *smtArithEngine) updateValueCore(v SMTTheoryVar, delta *smtArithmeticValue) error {
	e.saveValue(v)
	e.value[v].add(delta)
	if e.isBase(v) && !e.toPatch.Contains(int32(v)) && (e.belowLower(v) || e.aboveUpper(v)) {
		if err := e.toPatch.Insert(int32(v)); err != nil {
			return err
		}
	}
	e.host.limitInc(1)
	return nil
}

// updateValue is m_value[v] += delta and the induced change of the base
// variables whose rows contain v.
func (e *smtArithEngine) updateValue(v SMTTheoryVar, delta *smtArithmeticValue) error {
	if err := e.updateValueCore(v, delta); err != nil {
		return err
	}
	column := &e.columns[v]
	if err := column.compressIfNeeded(e.rows); err != nil {
		return err
	}
	var delta2 smtArithmeticValue
	for i := range column.entries {
		entry := column.entries[i]
		if entry.dead() {
			continue
		}
		row := &e.rows[entry.rowID]
		s := row.base
		if s != SMTNullTheoryVar && !e.isQuasiBase(s) {
			delta2.set(delta)
			delta2.mulRat(&row.entries[entry.rowIndex].coefficient)
			delta2.neg()
			if err := e.updateValueCore(s, &delta2); err != nil {
				return err
			}
		}
	}
	return nil
}

// setValue is set_value: update_value(v, new_val - m_value[v]).
func (e *smtArithEngine) setValue(v SMTTheoryVar, newValue *smtArithmeticValue) error {
	var delta smtArithmeticValue
	delta.set(newValue)
	delta.sub(&e.value[v])
	return e.updateValue(v, &delta)
}

// ---- add_row (core.h:1721-1820) ----

// addRow sets rows[rid1] <- rows[rid1] + coeff * rows[rid2].
func (e *smtArithEngine) addRow(rid1 int, coeff *big.Rat, rid2 int, applyGCDTest bool) error {
	e.stats.addRows++
	if e.propagationMode() != smtArithBoundPropNone {
		e.markRowForBoundProp(rid1)
	}
	r1, r2 := &e.rows[rid1], &e.rows[rid2]
	if err := r1.compressIfNeeded(e.columns); err != nil {
		return err
	}
	if err := r2.compressIfNeeded(e.columns); err != nil {
		return err
	}
	if err := r1.saveVariablePositions(e.varPos); err != nil {
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
		pos := e.varPos[v]
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
			colIdx, cEntry := e.columns[v].addEntry()
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
				if err := e.columns[v].deleteEntry(colIdx); err != nil {
					return err
				}
			}
			e.varPos[v] = -1
		}
	}
	if err := r1.resetVariablePositions(e.varPos); err != nil {
		return err
	}
	if applyGCDTest {
		v := r1.base
		if e.isInt(v) && !e.getValue(v).isInt() {
			if _, err := e.gcdTest(rid1); err != nil {
				return err
			}
		}
	}
	return nil
}

// addRows sets r1 <- r1 + sum_i a_xs[i].coeff * row(a_xs[i].var).
func (e *smtArithEngine) addRows(r1 int, monomials []smtArithLinearMonomial) error {
	for i := range monomials {
		m := &monomials[i]
		if err := e.addRow(r1, &m.coefficient, e.varRow(m.variable), false); err != nil {
			return err
		}
	}
	e.host.limitInc(uint(len(monomials)))
	return nil
}

// smtArithLinearMonomial is theory_arith::linear_monomial.
type smtArithLinearMonomial struct {
	coefficient big.Rat
	variable    SMTTheoryVar
}

// ---- pivoting (core.h:1905-2053) ----

// pivot makes x_j the base variable of the row owned by x_i; a_ij is the
// coefficient of x_j in that row. lazy eliminates x_j only from base rows.
func (e *smtArithEngine) pivot(lazy bool, xi, xj SMTTheoryVar, aij *big.Rat, applyGCDTest bool) error {
	e.stats.pivots++
	rowID := e.varRow(xi)
	row := &e.rows[rowID]
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
	e.host.limitInc(uint(row.size))
	e.data[xi].rowID = -1
	e.data[xj].rowID = rowID
	if row.base != xi {
		return errSMTArithEngineInvariant
	}
	row.base = xj
	e.data[xi].kind = smtArithmeticNonBase
	e.data[xj].kind = smtArithmeticBase
	return e.eliminate(lazy, xj, applyGCDTest)
}

// eliminate removes x_i from every row other than the one it owns. When
// lazy, only base rows are touched.
func (e *smtArithEngine) eliminate(lazy bool, xi SMTTheoryVar, applyGCDTest bool) error {
	rowID := e.varRow(xi)
	column := &e.columns[xi]
	sPos := -1
	var akj big.Rat
	for i := 0; i < len(column.entries); i++ {
		entry := column.entries[i]
		if entry.dead() {
			continue
		}
		if entry.rowID != rowID {
			r2 := &e.rows[entry.rowID]
			s2 := r2.base
			if s2 != SMTNullTheoryVar && (!lazy || e.isBase(s2)) {
				akj.Neg(&r2.entries[entry.rowIndex].coefficient)
				if err := e.addRow(entry.rowID, &akj, rowID, applyGCDTest); err != nil {
					return err
				}
				e.host.limitInc(uint(e.rows[rowID].size+r2.size) * uint(akj.Num().BitLen()/32+1))
			}
		} else {
			sPos = i
		}
	}
	column = &e.columns[xi]
	if column.size == 1 {
		// When lazy pivoting is used, c may not be a singleton after pivoting.
		return column.compressSingleton(e.rows, sPos)
	}
	if !lazy {
		return errSMTArithEngineInvariant
	}
	return nil
}

// updateAndPivot sets x_j to be the new base variable of the row owned by x_i,
// moving x_i to x_iNewVal.
func (e *smtArithEngine) updateAndPivot(xi, xj SMTTheoryVar, aij *big.Rat, xiNewVal *smtArithmeticValue) error {
	var theta smtArithmeticValue
	theta.set(&e.value[xi])
	theta.sub(xiNewVal)
	theta.quoRat(aij)
	if err := e.updateValue(xj, &theta); err != nil {
		return err
	}
	if !e.toPatch.Contains(int32(xj)) && (e.belowLower(xj) || e.aboveUpper(xj)) {
		if err := e.toPatch.Insert(int32(xj)); err != nil {
			return err
		}
	}
	return e.pivot(true, xi, xj, aij, e.eagerGCD)
}

// numNonFreeDepVars is get_num_non_free_dep_vars.
func (e *smtArithEngine) numNonFreeDepVars(v SMTTheoryVar, bestSoFar int) int {
	result := 0
	if e.isNonFree(v) {
		result = 1
	}
	column := &e.columns[v]
	for i := range column.entries {
		entry := &column.entries[i]
		if entry.dead() {
			continue
		}
		s := e.rows[entry.rowID].base
		if s != SMTNullTheoryVar && e.isBase(s) {
			if e.isNonFree(s) {
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
func (e *smtArithEngine) selectBlandsPivotCore(xi SMTTheoryVar, isBelow bool) (SMTTheoryVar, *big.Rat) {
	max := SMTTheoryVar(e.numVars())
	result := max
	var out *big.Rat
	row := &e.rows[e.varRow(xi)]
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
		if xi != xj && ((isPos && e.aboveLower(xj)) || (isNeg && e.belowUpper(xj))) {
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
func (e *smtArithEngine) selectPivotCore(isBelow bool, xi SMTTheoryVar) (SMTTheoryVar, *big.Rat) {
	max := SMTTheoryVar(e.numVars())
	result := max
	var out *big.Rat
	row := &e.rows[e.varRow(xi)]
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
		if xi != xj && ((isPos && e.aboveLower(xj)) || (isNeg && e.belowUpper(xj))) {
			num := e.numNonFreeDepVars(xj, bestSoFar)
			colSize := e.columns[xj].size
			if num < bestSoFar || (num == bestSoFar && colSize < bestColSize) {
				result, out = xj, &it.coefficient
				bestSoFar, bestColSize = num, colSize
				n = 1
			} else if num == bestSoFar && colSize == bestColSize {
				n++
				if e.random.Next()%n == 0 {
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

func (e *smtArithEngine) selectPivot(xi SMTTheoryVar, isBelow bool) (SMTTheoryVar, *big.Rat) {
	if e.blandsRule {
		return e.selectBlandsPivotCore(xi, isBelow)
	}
	return e.selectPivotCore(isBelow, xi)
}

// ---- make feasible (core.h:2188-2337) ----

// makeVarFeasible assumes x_i is a base variable and reports false, with a
// conflict set, when it cannot be made feasible.
func (e *smtArithEngine) makeVarFeasible(xi SMTTheoryVar) (bool, error) {
	var isBelow bool
	switch {
	case e.belowLower(xi):
		isBelow = true
	case e.aboveUpper(xi):
		isBelow = false
	default:
		return true, nil
	}
	xj, aij := e.selectPivot(xi, isBelow)
	if xj != SMTNullTheoryVar {
		// aij points into row storage that the pivot rewrites.
		a := new(big.Rat).Set(aij)
		target := &e.getBound(xi, !isBelow).core().value
		var newValue smtArithmeticValue
		newValue.set(target)
		return true, e.updateAndPivot(xi, xj, a, &newValue)
	}
	return false, e.signRowConflict(xi, isBelow)
}

// selectLGErrorVar is select_lg_error_var: the variable in m_to_patch with
// the least (greatest) error, removed from the heap.
func (e *smtArithEngine) selectLGErrorVar(least bool) (SMTTheoryVar, error) {
	best := SMTNullTheoryVar
	var bestError, currError smtArithmeticValue
	values, err := e.toPatch.AppendValues(nil)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	for _, raw := range values {
		v := SMTTheoryVar(raw)
		switch {
		case e.belowLower(v):
			currError.set(&e.lower(v).core().value)
			currError.sub(e.getValue(v))
		case e.aboveUpper(v):
			currError.set(e.getValue(v))
			currError.sub(&e.upper(v).core().value)
		default:
			continue
		}
		var zero smtArithmeticValue
		if currError.cmp(&zero) <= 0 {
			return SMTNullTheoryVar, errSMTArithEngineInvariant
		}
		c := currError.cmp(&bestError)
		if best == SMTNullTheoryVar || (!least && c > 0) || (least && c < 0) {
			best = v
			bestError.set(&currError)
		}
	}
	if best == SMTNullTheoryVar {
		return best, e.toPatch.Clear()
	}
	return best, e.toPatch.Erase(int32(best))
}

func (e *smtArithEngine) selectSmallestVar() (SMTTheoryVar, error) {
	v, err := e.toPatch.EraseMin()
	return SMTTheoryVar(v), err
}

func (e *smtArithEngine) selectVarToFix() (SMTTheoryVar, error) {
	if e.blandsRule {
		return e.selectSmallestVar()
	}
	switch e.params.PivotStrategy {
	case smtArithPivotGreatest:
		return e.selectLGErrorVar(false)
	case smtArithPivotLeast:
		return e.selectLGErrorVar(true)
	default:
		return e.selectSmallestVar()
	}
}

// makeFeasible reports whether every variable in m_to_patch could be patched.
func (e *smtArithEngine) makeFeasible() (bool, error) {
	e.leftBasis.Reset()
	e.blandsRule = false
	numRepeated := uint32(0)
	for !e.toPatch.Empty() {
		v, err := e.selectVarToFix()
		if err != nil {
			return false, err
		}
		if v == SMTNullTheoryVar {
			break // all variables were satisfied
		}
		if !e.blandsRule {
			if e.leftBasis.Contains(uint32(v)) {
				numRepeated++
				if numRepeated > e.params.BlandsRuleThreshold {
					e.blandsRule = true
				}
			} else {
				e.leftBasis.Insert(uint32(v))
			}
		}
		ok, err := e.makeVarFeasible(v)
		if err != nil || !ok {
			return false, err
		}
		if e.host.cancelled() {
			return true, nil
		}
	}
	return true, nil
}

// ---- conflicts (core.h:2339-2393, 2524-2537, 3056-3100) ----

// signRowConflict: the row of x_i implies a bound on x_i that contradicts
// the bound it already has.
func (e *smtArithEngine) signRowConflict(xi SMTTheoryVar, isBelow bool) error {
	var delta smtArithmeticValue
	row := &e.rows[e.varRow(xi)]
	idx := row.indexOf(xi)
	if idx < 0 {
		return errSMTArithEngineInvariant
	}
	var b smtArithEngineBound
	if isBelow {
		b = e.lower(xi)
		if e.relaxBounds() {
			delta.set(&b.core().value)
			delta.sub(e.getValue(xi))
			delta.sub(e.epsilon(xi))
			if delta.sign() < 0 {
				delta.reset()
			}
		}
	} else {
		b = e.upper(xi)
		if e.relaxBounds() {
			delta.set(e.getValue(xi))
			delta.sub(&b.core().value)
			delta.sub(e.epsilon(xi))
			if delta.sign() < 0 {
				delta.reset()
			}
		}
	}
	ante := e.newAntecedents()
	defer e.releaseAntecedents()
	if err := e.explainBound(row, idx, !isBelow, &delta, ante); err != nil {
		return err
	}
	one := big.NewRat(1, 1)
	if err := b.pushJustification(ante, one, e.coeffsEnabled()); err != nil {
		return err
	}
	return e.setConflict(ante, "farkas")
}

// signBoundConflict: bounds b1 and b2 on one variable are contradictory.
func (e *smtArithEngine) signBoundConflict(b1, b2 smtArithEngineBound) error {
	if b1.core().variable != b2.core().variable {
		return errSMTArithEngineInvariant
	}
	ante := e.newAntecedents()
	defer e.releaseAntecedents()
	one := big.NewRat(1, 1)
	if err := b1.pushJustification(ante, one, e.coeffsEnabled()); err != nil {
		return err
	}
	if err := b2.pushJustification(ante, one, e.coeffsEnabled()); err != nil {
		return err
	}
	return e.setConflict(ante, "farkas")
}

func (e *smtArithEngine) setConflict(ante *smtArithmeticAntecedents, rule string) error {
	if len(ante.literals) == 0 && len(ante.equalities) == 0 {
		return errSMTArithEngineInvariant
	}
	e.stats.conflicts++
	e.numConflicts++
	return e.host.setConflict(ante, rule)
}

// newAntecedents / releaseAntecedents are theory_arith::antecedents: a stack
// of three reusable records.
func (e *smtArithEngine) newAntecedents() *smtArithmeticAntecedents {
	a := &e.antecedentsPool[e.antecedentsIndex]
	a.reset()
	e.antecedentsIndex++
	return a
}
func (e *smtArithEngine) releaseAntecedents() { e.antecedentsIndex-- }

// ---- assert bound (core.h:2395-2522) ----

func (e *smtArithEngine) assertLower(b smtArithEngineBound) (bool, error) {
	c := b.core()
	v := c.variable
	k := &c.value
	u, l := e.upper(v), e.lower(v)
	if u != nil && k.cmp(&u.core().value) > 0 {
		return false, e.signBoundConflict(u, b)
	}
	if l != nil && k.cmp(&l.core().value) <= 0 {
		return true, nil // redundant
	}
	switch e.varKind(v) {
	case smtArithmeticQuasiBase:
		if err := e.quasiBaseRow2BaseRow(e.varRow(v)); err != nil {
			return false, err
		}
		fallthrough
	case smtArithmeticBase:
		if !e.toPatch.Contains(int32(v)) && e.getValue(v).cmp(k) < 0 {
			if err := e.toPatch.Insert(int32(v)); err != nil {
				return false, err
			}
		}
	case smtArithmeticNonBase:
		if e.getValue(v).cmp(k) < 0 {
			if err := e.setValue(v, k); err != nil {
				return false, err
			}
		}
	}
	e.boundTrail = append(e.boundTrail, smtArithEngineBoundTrail{variable: v, upper: false, old: l})
	if err := e.setBound(b, false); err != nil {
		return false, err
	}
	if e.propagationMode() != smtArithBoundPropNone {
		e.markRowsForBoundProp(v)
	}
	return true, nil
}

func (e *smtArithEngine) assertUpper(b smtArithEngineBound) (bool, error) {
	c := b.core()
	v := c.variable
	k := &c.value
	u, l := e.upper(v), e.lower(v)
	if l != nil && k.cmp(&l.core().value) < 0 {
		return false, e.signBoundConflict(l, b)
	}
	if u != nil && k.cmp(&u.core().value) >= 0 {
		return true, nil // redundant
	}
	switch e.varKind(v) {
	case smtArithmeticQuasiBase:
		if err := e.quasiBaseRow2BaseRow(e.varRow(v)); err != nil {
			return false, err
		}
		fallthrough
	case smtArithmeticBase:
		if !e.toPatch.Contains(int32(v)) && e.getValue(v).cmp(k) > 0 {
			if err := e.toPatch.Insert(int32(v)); err != nil {
				return false, err
			}
		}
	case smtArithmeticNonBase:
		if e.getValue(v).cmp(k) > 0 {
			if err := e.setValue(v, k); err != nil {
				return false, err
			}
		}
	}
	e.boundTrail = append(e.boundTrail, smtArithEngineBoundTrail{variable: v, upper: true, old: u})
	if err := e.setBound(b, true); err != nil {
		return false, err
	}
	if e.propagationMode() != smtArithBoundPropNone {
		e.markRowsForBoundProp(v)
	}
	return true, nil
}

// setBound installs new_bound and, when that makes the variable fixed,
// invokes fixed_var_eh (theory_arith_aux.h set_bound).
func (e *smtArithEngine) setBound(b smtArithEngineBound, upper bool) error {
	v := b.core().variable
	if upper != (b.core().kind == smtArithmeticUpperBound) {
		return errSMTArithEngineInvariant
	}
	if upper {
		e.bounds[1][v] = b
	} else {
		e.bounds[0][v] = b
	}
	if e.propagateEqs() && e.isFixed(v) {
		return e.fixedVarEh(v)
	}
	return nil
}

// assertBound asserts b and reports false, with a conflict set, when it
// contradicts a known bound.
func (e *smtArithEngine) assertBound(b smtArithEngineBound) (bool, error) {
	c := b.core()
	v := c.variable
	if c.atom {
		if e.unassignedAtoms[v] == 0 {
			return false, errSMTArithEngineInvariant
		}
		e.unassignedTrail = append(e.unassignedTrail, uint32(v))
		e.unassignedAtoms[v]--
	}
	if c.kind == smtArithmeticLowerBound {
		e.stats.assertLower++
		return e.assertLower(b)
	}
	e.stats.assertUpper++
	return e.assertUpper(b)
}

// smtArithEngineBoundTrail is theory_arith::bound_trail: the bound a
// variable had before it was replaced, restored on backtracking.
type smtArithEngineBoundTrail struct {
	variable SMTTheoryVar
	upper    bool
	old      smtArithEngineBound
}
