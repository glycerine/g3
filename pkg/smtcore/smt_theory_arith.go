package smtcore

// Source: src/smt/theory_arith.h. SMTTheoryArith is theory_arith<Ext>. It
// embeds SMTTheoryBase, the port of class theory, and reaches the context
// through t.state where the source calls get_context(). The template
// parameter Ext is a field: integerExt selects i_ext (inf_numeral is a
// rational), otherwise mi_ext (inf_numeral is an inf_rational). numeral is
// big.Rat and inf_numeral is smtArithmeticValue in both.
//
// row, column, row_entry and col_entry are theory_arith_tableau.go; bound,
// atom, eq_bound, derived_bound, justified_derived_bound, antecedents_t,
// var_data and the kind enums are theory_arith_bounds.go.

import (
	"errors"
	"math/big"
	"os"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/heap"
	"github.com/glycerine/g3/pkg/util/natset"
	"github.com/glycerine/g3/pkg/util/numeral"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

// theory_arith_stats (theory_arith.h:44-54).
type smtTheoryArithStats struct {
	conflicts, addRows, pivots, diseqCs, gomoryCuts, branches, gcdTests    uint32
	assertLower, assertUpper, assertDiseq, core2thEqs, core2thDiseqs       uint32
	th2coreEqs, th2coreDiseqs, boundProps, offsetEqs, fixedEqs, offlineEqs uint32
	maxMin                                                                 uint32
	gbSimplify, gbSuperpose, gbComputeBasis, gbNumProcessed                uint32
	nlBranching, nlLinear, nlBounds, nlCrossNested                         uint32
}

// smtArithBound is the virtual interface of class bound (theory_arith.h:272).
// *smtArithmeticBound is the base class; atom, eq_bound, derived_bound and
// justified_derived_bound override has_justification and push_justification.
type smtArithBound interface {
	smtArithmeticBoundJustification
	base() *smtArithmeticBound
	hasJustification() bool
}

func (b *smtArithmeticBound) base() *smtArithmeticBound { return b }

// bound::has_justification and bound::push_justification: a plain bound
// has no justification.
func (b *smtArithmeticBound) hasJustification() bool { return false }
func (b *smtArithmeticBound) pushJustification(*smtArithmeticAntecedents, *big.Rat, bool) error {
	return nil
}
func (a *smtArithmeticAtom) hasJustification() bool                  { return true }
func (b *smtArithmeticEqualityBound) hasJustification() bool         { return true }
func (b *smtArithmeticDerivedBound) hasJustification() bool          { return true }
func (b *smtArithmeticJustifiedDerivedBound) hasJustification() bool { return true }

// derived_bound's virtual push_lit/push_eq, overridden by
// justified_derived_bound.
type smtArithDerivedBound interface {
	smtArithBound
	pushLiteral(SMTLiteral, *big.Rat) error
	pushEquality(SMTEnodePair, *big.Rat) error
	derived() *smtArithmeticDerivedBound
}

func (b *smtArithmeticDerivedBound) derived() *smtArithmeticDerivedBound { return b }

// linear_monomial (theory_arith.h:97).
type smtArithLinearMonomial struct {
	coeff big.Rat
	v     SMTTheoryVar
}

// bound_trail (theory_arith.h:412).
type smtArithBoundTrail struct {
	v        SMTTheoryVar
	upper    bool
	oldBound smtArithBound
}

// scope (theory_arith.h:479).
type smtArithScope struct {
	atomsLim                uint32
	boundTrailLim           uint32
	unassignedAtomsTrailLim uint32
	assertedBoundsLim       uint32
	assertedQheadOld        uint32
	boundsToDeleteLim       uint32
	nlMonomialsLim          uint32
	nlPropagatedLim         uint32
	// value_trail entries
	foundUnsupportedOp, foundUnderspecifiedOp bool
	finalCheckIdx                             uint32
	assumeEqCandidatesLim, assumeEqHead       uint32
	nlRounds, nlStrategyIdx                   uint32
	nlGbExhausted                             bool
}

// value_sort_pair and var_offset keys of m_fixed_var_table and
// m_var_offset2row_id (theory_arith.h:855-863).
type smtArithValueSortPair struct {
	value string
	isInt bool
}
type smtArithVarOffset struct {
	v      SMTTheoryVar
	offset string
}

const smtArithDeadRowID = -1 // dead_row_id

type SMTTheoryArith struct {
	*SMTTheoryBase
	integerExt bool // Ext == i_ext

	stats                 smtTheoryArithStats
	params                SMTArithState
	foundUnsupportedOp    bool
	foundUnderspecifiedOp bool
	arithEqAdapter        *smtArithmeticEqualityAdapter
	rows                  []smtArithmeticRow
	deadRows              []int
	columns               []smtArithmeticColumn
	data                  []smtArithmeticVariableData
	value                 []smtArithmeticValue
	oldValue              []smtArithmeticValue
	bounds                [2][]smtArithBound
	varOccs               [][]*smtArithmeticAtom
	unassignedAtoms       []uint32
	boolVar2Atom          []*smtArithmeticAtom
	varPos                []int
	atoms                 []*smtArithmeticAtom
	assertedBounds        []smtArithBound
	assertedQhead         uint32
	newAtoms              []*smtArithmeticAtom
	nlMonomials           []SMTTheoryVar
	nlPropagated          []SMTTheoryVar
	rowVars               []map[SMTTheoryVar]bool
	rowVarsTop            uint32
	toPatch               *heap.Int
	leftBasis             *natset.Set
	blandsRule            bool
	updateTrailStack      []uint32
	inUpdateTrailStack    *natset.Set
	toCheck               []uint32
	inToCheck             *natset.Set
	tmp                   smtArithmeticValue
	random                *z3rand.Generator
	numConflicts          uint32
	branchCutCounter      uint32
	eagerGCD              bool
	finalCheckIdx         uint32
	boundTrail            []smtArithBoundTrail
	unassignedAtomsTrail  []uint32
	boundsToDelete        []smtArithBound
	scopes                []smtArithScope
	tmpLiteralVector2     []SMTLiteral
	antecedents           [3]smtArithmeticAntecedents
	antecedentsIndex      uint32
	liberalFinalCheck     bool
	changedAssignment     bool
	fixedVarTable         map[smtArithValueSortPair]SMTTheoryVar
	varOffset2RowID       map[smtArithVarOffset]int
	tmpLitSet             map[int32]bool
	tmpEqSet              map[SMTEnodePair]bool
	boundWatch            SMTBoolVar
	intEpsilon            smtArithmeticValue // Ext::m_int_epsilon
	realEpsilon           smtArithmeticValue // Ext::m_real_epsilon

	tmpRow             smtArithmeticRow                    // m_tmp_row
	esNumerals         *numeral.RationalManager            // m_es_num_manager
	depManager         smtArithmeticBoundDependencyManager // m_dep_manager
	depBounds          map[*smtArithmeticBound]smtArithBound
	nlNewExprs         smtArithExprs // m_nl_new_exprs
	varValueTable      map[smtArithVarValueKey]SMTTheoryVar
	assumeEqCandidates [][2]SMTTheoryVar
	assumeEqHead       uint32
	upperBoundRecorded smtArithmeticValue // m_upper_bound

	// model generation
	factory                       *protomodel.ArithmeticFactory
	epsilon                       big.Rat
	modelDependsOnComputedEpsilon bool

	// non linear
	nlRounds      uint32
	nlGbExhausted bool
	nlStrategyIdx uint32
}

func (t *SMTTheoryArith) getNumVars() int { return len(t.columns) }

// ---- inline members of theory_arith.h, in header order ----

func (t *SMTTheoryArith) proofsEnabled() bool {
	proofs, err := t.state.manager.ProofsEnabled()
	return err == nil && proofs
}
func (t *SMTTheoryArith) coeffsEnabled() bool {
	return t.proofsEnabled() || t.boundWatch != SMTNullBoolVar
}

func (t *SMTTheoryArith) hasVar(e ast.ASTHandle) bool {
	n := t.state.context.enodes.get(e)
	return n != nil && n.TheoryVar(t.FamilyID()) != SMTNullTheoryVar
}
func (t *SMTTheoryArith) expr2var(e ast.ASTHandle) SMTTheoryVar {
	return t.state.context.enodes.get(e).TheoryVar(t.FamilyID())
}
func (t *SMTTheoryArith) var2expr(v SMTTheoryVar) ast.ASTHandle {
	n, _ := t.Enode(v)
	return n.owner
}

func (t *SMTTheoryArith) lazyPivotingLvl() uint32 { return t.params.LazyPivotingLevel }
func (t *SMTTheoryArith) propagateEqs() bool {
	return t.params.PropagateEqs && t.numConflicts < t.params.PropagationThreshold
}
func (t *SMTTheoryArith) propagateDiseqs() bool       { return false }
func (t *SMTTheoryArith) randomInitialValue() bool    { return t.params.RandomInitialValue }
func (t *SMTTheoryArith) randomLower() int32          { return t.params.RandomLower }
func (t *SMTTheoryArith) randomUpper() int32          { return t.params.RandomUpper }
func (t *SMTTheoryArith) blandsRuleThreshold() uint32 { return t.params.BlandsRuleThreshold }
func (t *SMTTheoryArith) propagationMode() uint32 {
	if t.numConflicts < t.params.PropagationThreshold {
		return t.params.BoundProp
	}
	return smtArithBPNone
}
func (t *SMTTheoryArith) adaptive() bool { return t.params.Adaptive }
func (t *SMTTheoryArith) adaptiveAssertionThreshold() float64 {
	return t.params.AdaptiveAssertionThreshold
}
func (t *SMTTheoryArith) maxLemmaSize() uint32                                { return t.params.MaxLemmaSize }
func (t *SMTTheoryArith) smallLemmaSize() uint32                              { return t.params.SmallLemmaSize }
func (t *SMTTheoryArith) relaxBounds() bool                                   { return t.params.StrongerLemmas }
func (t *SMTTheoryArith) skipBigCoeffs() bool                                 { return t.params.SkipRowsWithBigCoeffs }
func (t *SMTTheoryArith) getNumConflicts() uint32                             { return t.numConflicts }
func (t *SMTTheoryArith) getVarKind(v SMTTheoryVar) smtArithmeticVariableKind { return t.data[v].kind }
func (t *SMTTheoryArith) isBase(v SMTTheoryVar) bool                          { return t.getVarKind(v) == smtArithmeticBase }
func (t *SMTTheoryArith) isQuasiBase(v SMTTheoryVar) bool {
	return t.getVarKind(v) == smtArithmeticQuasiBase
}
func (t *SMTTheoryArith) isNonBase(v SMTTheoryVar) bool {
	return t.getVarKind(v) == smtArithmeticNonBase
}
func (t *SMTTheoryArith) setVarKind(v SMTTheoryVar, k smtArithmeticVariableKind) { t.data[v].kind = k }
func (t *SMTTheoryArith) getVarRow(v SMTTheoryVar) int                           { return t.data[v].rowID }
func (t *SMTTheoryArith) setVarRow(v SMTTheoryVar, rID int)                      { t.data[v].rowID = rID }
func (t *SMTTheoryArith) isInt(v SMTTheoryVar) bool                              { return t.data[v].integer }
func (t *SMTTheoryArith) isReal(v SMTTheoryVar) bool                             { return !t.isInt(v) }

// is_int_src: m_util.is_int(var2expr(v)), the sort of the expression is Int.
func (t *SMTTheoryArith) isIntSrc(v SMTTheoryVar) bool {
	isInt, _ := t.isIntSort(t.var2expr(v))
	return isInt
}
func (t *SMTTheoryArith) isRealSrc(v SMTTheoryVar) bool { return !t.isIntSrc(v) }

func (t *SMTTheoryArith) getQuasiBaseValue(v SMTTheoryVar) *smtArithmeticValue {
	return t.getImpliedValue(v)
}
func (t *SMTTheoryArith) getValue(v SMTTheoryVar) *smtArithmeticValue {
	if t.isQuasiBase(v) {
		return t.getQuasiBaseValue(v)
	}
	return &t.value[v]
}
func (t *SMTTheoryArith) getBound(v SMTTheoryVar, upper bool) smtArithBound {
	if upper {
		return t.bounds[1][v]
	}
	return t.bounds[0][v]
}
func (t *SMTTheoryArith) lower(v SMTTheoryVar) smtArithBound { return t.bounds[0][v] }
func (t *SMTTheoryArith) upper(v SMTTheoryVar) smtArithBound { return t.bounds[1][v] }
func (t *SMTTheoryArith) lowerBound(v SMTTheoryVar) *smtArithmeticValue {
	return &t.lower(v).base().value
}
func (t *SMTTheoryArith) upperBound(v SMTTheoryVar) *smtArithmeticValue {
	return &t.upper(v).base().value
}
func (t *SMTTheoryArith) belowLower(v SMTTheoryVar) bool {
	l := t.lower(v)
	return l != nil && t.getValue(v).cmp(&l.base().value) < 0
}
func (t *SMTTheoryArith) aboveUpper(v SMTTheoryVar) bool {
	u := t.upper(v)
	return u != nil && t.getValue(v).cmp(&u.base().value) > 0
}
func (t *SMTTheoryArith) belowUpper(v SMTTheoryVar) bool {
	u := t.upper(v)
	return u == nil || t.getValue(v).cmp(&u.base().value) < 0
}
func (t *SMTTheoryArith) aboveLower(v SMTTheoryVar) bool {
	l := t.lower(v)
	return l == nil || t.getValue(v).cmp(&l.base().value) > 0
}
func (t *SMTTheoryArith) atLower(v SMTTheoryVar) bool {
	l := t.lower(v)
	return l != nil && t.getValue(v).cmp(&l.base().value) == 0
}
func (t *SMTTheoryArith) atUpper(v SMTTheoryVar) bool {
	u := t.upper(v)
	return u != nil && t.getValue(v).cmp(&u.base().value) == 0
}
func (t *SMTTheoryArith) isFree(v SMTTheoryVar) bool { return t.lower(v) == nil && t.upper(v) == nil }
func (t *SMTTheoryArith) isNonFree(v SMTTheoryVar) bool {
	return t.lower(v) != nil || t.upper(v) != nil
}
func (t *SMTTheoryArith) isBounded(v SMTTheoryVar) bool {
	return t.lower(v) != nil && t.upper(v) != nil
}
func (t *SMTTheoryArith) setBoundCore(v SMTTheoryVar, newBound smtArithBound, upper bool) {
	if upper {
		t.bounds[1][v] = newBound
	} else {
		t.bounds[0][v] = newBound
	}
}
func (t *SMTTheoryArith) restoreBound(v SMTTheoryVar, newBound smtArithBound, upper bool) {
	t.setBoundCore(v, newBound, upper)
}
func (t *SMTTheoryArith) getEpsilon(v SMTTheoryVar) *smtArithmeticValue {
	if t.isReal(v) {
		return &t.realEpsilon
	}
	return &t.intEpsilon
}

// bool_var -> atom mapping (theory_arith.h:691-715, the non-SPARSE_MAP form).
func (t *SMTTheoryArith) insertBv2a(bv SMTBoolVar, a *smtArithmeticAtom) {
	for int(bv) >= len(t.boolVar2Atom) {
		t.boolVar2Atom = append(t.boolVar2Atom, nil)
	}
	t.boolVar2Atom[bv] = a
}
func (t *SMTTheoryArith) eraseBv2a(bv SMTBoolVar) { t.boolVar2Atom[bv] = nil }
func (t *SMTTheoryArith) getBv2a(bv SMTBoolVar) *smtArithmeticAtom {
	if int(bv) < len(t.boolVar2Atom) {
		return t.boolVar2Atom[bv]
	}
	return nil
}

func (t *SMTTheoryArith) setValue(v SMTTheoryVar, newVal *smtArithmeticValue) error {
	var delta smtArithmeticValue
	delta.set(newVal)
	delta.sub(&t.value[v])
	return t.updateValue(v, &delta)
}

func (t *SMTTheoryArith) selectGreatestErrorVar() (SMTTheoryVar, error) {
	return t.selectLgErrorVar(false)
}
func (t *SMTTheoryArith) selectLeastErrorVar() (SMTTheoryVar, error) { return t.selectLgErrorVar(true) }

func (t *SMTTheoryArith) isEqual(x, y SMTTheoryVar) bool {
	nx, _ := t.Enode(x)
	ny, _ := t.Enode(y)
	return nx.Root() == ny.Root()
}

func (t *SMTTheoryArith) pushBoundTrail(v SMTTheoryVar, oldBound smtArithBound, isUpper bool) {
	t.boundTrail = append(t.boundTrail, smtArithBoundTrail{v: v, upper: isUpper, oldBound: oldBound})
}
func (t *SMTTheoryArith) pushDecUnassignedAtomsTrail(v SMTTheoryVar) {
	t.unassignedAtomsTrail = append(t.unassignedAtomsTrail, uint32(v))
}

// ---- Ext (theory_arith.h:1155-1191) ----

func (t *SMTTheoryArith) initExt() {
	t.intEpsilon.reset()
	t.realEpsilon.reset()
	t.intEpsilon.rational.SetInt64(1)
	if t.integerExt {
		t.realEpsilon.rational.SetInt64(1) // i_ext: m_real_epsilon(1)
	} else {
		t.realEpsilon.infinitesimal.SetInt64(1) // mi_ext: inf_rational(0, true)
	}
}

const smtArithBPNone = 0 // BP_NONE

// arith_pivot_strategy
const (
	smtArithPivotSmallest      = 0 // ARITH_PIVOT_SMALLEST
	smtArithPivotGreatestError = 1 // ARITH_PIVOT_GREATEST_ERROR
	smtArithPivotLeastError    = 2 // ARITH_PIVOT_LEAST_ERROR
)

// ---- constructor (core.h:1670-1713) ----

// NewSMTTheoryArith creates theory_i_arith when integerExt is set, otherwise
// theory_mi_arith.
func NewSMTTheoryArith(family ast.FamilyID, integerExt bool) (*SMTTheoryArith, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	t := &SMTTheoryArith{
		SMTTheoryBase:      base,
		integerExt:         integerExt,
		boundWatch:         SMTNullBoolVar,
		leftBasis:          natset.New(0),
		inUpdateTrailStack: natset.New(0),
		inToCheck:          natset.New(0),
		liberalFinalCheck:  true,
		fixedVarTable:      make(map[smtArithValueSortPair]SMTTheoryVar),
		varOffset2RowID:    make(map[smtArithVarOffset]int),
		tmpLitSet:          make(map[int32]bool),
		tmpEqSet:           make(map[SMTEnodePair]bool),
		varValueTable:      make(map[smtArithVarValueKey]SMTTheoryVar),
		depBounds:          make(map[*smtArithmeticBound]smtArithBound),
	}
	if t.toPatch, err = heap.New(1024, heap.Config{}); err != nil {
		return nil, err
	}
	if t.arithEqAdapter, err = newSMTArithmeticEqualityAdapter(base); err != nil {
		return nil, err
	}
	t.arithEqAdapter.source = true
	t.initExt()
	return t, t.ConfigureArithmetic(smtArithDefaultState())
}

// ConfigureArithmetic sets m_params; the random seed and eager gcd follow
// the constructor and setup().
func (t *SMTTheoryArith) ConfigureArithmetic(params SMTArithState) error {
	t.params = params
	t.random = z3rand.New(params.RandomSeed)
	t.eagerGCD = params.EagerGCD
	t.arithEqAdapter.lazy = params.LazyAdapter
	return nil
}

func (t *SMTTheoryArith) Initialize(state *SMTInternalizerState) error {
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	t.nlNewExprs = smtArithExprs{m: state.manager}
	return nil
}

// DefaultInternalizer: the source returns false.
func (t *SMTTheoryArith) DefaultInternalizer() bool { return false }

// Setup is theory_arith::setup: reseed, then theory::setup.
func (t *SMTTheoryArith) Setup() error {
	t.random.SetSeed(t.params.RandomSeed)
	return t.SMTTheoryBase.Setup()
}

func (t *SMTTheoryArith) Close() (result error) {
	if t.toPatch != nil {
		result = t.toPatch.Close()
		t.toPatch = nil
	}
	result = errors.Join(result, t.nlNewExprs.release())
	if t.esNumerals != nil {
		result = errors.Join(result, t.esNumerals.CloseChecked())
		t.esNumerals = nil
	}
	return result
}

// smtTheoryArithPortEnabled selects SMTTheoryArith in place of the replay
// arithmetic theory (G3_THEORY_ARITH=port) while the port is compared with
// the C++ build.
func smtTheoryArithPortEnabled() bool { return os.Getenv("G3_THEORY_ARITH") == "port" }

// smtSetupUsesTheoryLRA: smt_setup.cpp selects theory_lra (setup_r_arith),
// not theory_arith, for these logics.
func smtSetupUsesTheoryLRA(logic string) bool { return logic == "QF_LRA" || logic == "QF_UFLRA" }
