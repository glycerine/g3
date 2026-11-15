package smtcore

// Source: src/smt/theory_wmaxsat.cpp weighted slack variables, exact cost
// bounds, conflict construction, and blocking clauses.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/sorting"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type smtWMaxVariable struct {
	boolean  int32         // m_var2bool
	expr     ast.ASTHandle // m_vars
	weight   big.Rat       // m_rweights
	enabled  bool          // m_enabled
	assigned bool          // m_assigned
}

// smtWMaxTrailEntry undoes one assign_eh: the numeral_trail on m_zcost,
// the push_back_vector on m_costs and the value_trail on m_assigned[tv],
// which assign_eh always pushes together.
type smtWMaxTrailEntry struct {
	variable int     // tv
	oldCost  big.Int // m_old_values entry
}

type smtWMaxStatistics struct {
	blocks       uint32 // m_num_blocks
	propagations uint32 // m_num_propagations
}

// SMTWeightedMaxTheory is theory_wmaxsat, the weighted slack-variable theory
// used by src/opt/wmax.cpp. Costs are kept as in the source: integer weights
// scaled by the lcm m_den of the rational weights (normalize), a
// trail-restored running cost m_zcost over the asserted slack variables
// m_costs, and conflicts raised only from assign_eh.
type SMTWeightedMaxTheory struct {
	*SMTTheoryBase
	filter       *tactic.FilterModelConverter
	variables    []smtWMaxVariable
	byBoolean    map[int32]int // m_bool2var
	zweights     []big.Int     // m_zweights
	costs        []int         // m_costs
	costSave     []int         // m_cost_save
	rminCost     big.Rat       // m_rmin_cost
	zcost        big.Int       // m_zcost
	zminCost     big.Int       // m_zmin_cost
	den          big.Int       // m_den
	foundOptimal bool          // m_found_optimal
	propagate    bool          // m_propagate
	canPropagate bool          // m_can_propagate
	normalizing  bool          // m_normalize
	trail        []smtWMaxTrailEntry
	scopeLimits  []smtWMaxScope
	stats        smtWMaxStatistics
}

type smtWMaxScope struct {
	variables int
	trail     int
}

func NewSMTWeightedMaxTheory(family ast.FamilyID, filter *tactic.FilterModelConverter) (*SMTWeightedMaxTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	if filter == nil {
		return nil, ast.ErrASTHandle
	}
	t := &SMTWeightedMaxTheory{SMTTheoryBase: base, filter: filter, byBoolean: make(map[int32]int)}
	t.den.SetInt64(1) // m_den(1)
	return t, nil
}

func (t *SMTWeightedMaxTheory) AddWeighted(formula ast.ASTHandle, weight *big.Rat) (ast.ASTHandle, ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if weight == nil || weight.Sign() <= 0 {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ErrASTDomain
	}
	// theory_wmaxsat::assert_weighted asserts its disjunction before
	// register_var; context::assert_expr starts with pop_to_base_lvl, so the
	// slack variable is created at the base level.
	if err = state.popToBaseLevel(); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	manager := state.manager
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	slack, err := manager.MkFreshConst("w", boolean)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	slackRef, err := manager.NewRef(slack)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	defer manager.DelRef(&slackRef)
	declaration, err := manager.GetDecl(slack)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = t.filter.Insert(declaration); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	variable, err := state.MkBoolVar(slack)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	assertion, err := manager.MkOr(slack, formula)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	// m_rweights.push_back(w); m_vars.push_back(var); m_assigned.push_back(false);
	// m_enabled.push_back(true); m_normalize = true; register_var(var, true).
	entry := smtWMaxVariable{boolean: variable, expr: slack, enabled: true}
	entry.weight.Set(weight)
	t.byBoolean[variable] = len(t.variables)
	t.variables = append(t.variables, entry)
	t.normalizing = true
	return slack, assertion, nil
}

// InitMinCost is theory_wmaxsat::init_min_cost.
func (t *SMTWeightedMaxTheory) InitMinCost(value *big.Rat) error {
	if value == nil || value.Sign() < 0 {
		return ast.ErrASTDomain
	}
	t.rminCost.Set(value)
	t.zminCost.Set(smtWMaxScaledNumerator(&t.rminCost, &t.den))
	return nil
}

// smtWMaxScaledNumerator is (r * m_den).to_mpq().numerator().
func smtWMaxScaledNumerator(r *big.Rat, den *big.Int) *big.Int {
	var scaled big.Rat
	scaled.Mul(r, new(big.Rat).SetInt(den))
	return new(big.Int).Set(scaled.Num())
}

// normalize is theory_wmaxsat::normalize. m_sorted_vars and
// m_max_unassigned_index only serve max_unassigned_is_blocked, which the
// source never calls, so they are not kept.
func (t *SMTWeightedMaxTheory) normalize() {
	t.den.SetInt64(1)
	for i := range t.variables {
		if t.variables[i].enabled {
			t.den.Set(smtWMaxLCM(&t.den, t.variables[i].weight.Denom()))
		}
	}
	t.den.Set(smtWMaxLCM(&t.den, t.rminCost.Denom()))
	t.zweights = t.zweights[:0]
	for i := range t.variables {
		t.zweights = append(t.zweights, *smtWMaxScaledNumerator(&t.variables[i].weight, &t.den))
	}
	t.zcost.SetInt64(0)
	t.zminCost.Set(smtWMaxScaledNumerator(&t.rminCost, &t.den))
	t.normalizing = false
}

func smtWMaxLCM(a, b *big.Int) *big.Int {
	var gcd, product big.Int
	gcd.GCD(nil, nil, a, b)
	product.Mul(a, b)
	product.Abs(&product)
	return product.Quo(&product, &gcd)
}

// InitializeSearch is theory_wmaxsat::init_search_eh.
func (t *SMTWeightedMaxTheory) InitializeSearch() error {
	t.propagate = true
	return nil
}

func (t *SMTWeightedMaxTheory) CanPropagate() bool {
	return t.propagate || t.canPropagate
}

// Propagate is theory_wmaxsat::propagate().
func (t *SMTWeightedMaxTheory) Propagate() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	for i := 0; t.propagate && i < len(t.variables); i++ {
		boolean := t.variables[i].boolean
		value, err := state.Assignment(NewSMTLiteral(boolean, false))
		if err != nil {
			return err
		}
		if value == lbool.True {
			if err = t.Assign(boolean, true); err != nil {
				return err
			}
		}
	}
	t.propagate = false
	t.canPropagate = false
	return nil
}

// Assign is theory_wmaxsat::assign_eh.
func (t *SMTWeightedMaxTheory) Assign(variable int32, truth bool) error {
	if !truth {
		return nil
	}
	if t.normalizing {
		t.normalize()
	}
	tv, found := t.byBoolean[variable]
	if !found {
		return ast.ErrASTDomain
	}
	entry := &t.variables[tv]
	if entry.assigned || !entry.enabled {
		return nil
	}
	// numeral_trail(m_zcost), push_back_vector(m_costs), value_trail(m_assigned[tv]).
	trail := smtWMaxTrailEntry{variable: tv}
	trail.oldCost.Set(&t.zcost)
	t.trail = append(t.trail, trail)
	t.zcost.Add(&t.zcost, &t.zweights[tv])
	t.costs = append(t.costs, tv)
	entry.assigned = true
	if t.zcost.Cmp(&t.zminCost) >= 0 {
		return t.block()
	}
	t.canPropagate = true
	return nil
}

// smtWMaxCostOrder is compare_cost: m_zweights[v] > m_zweights[w].
type smtWMaxCostOrder struct {
	costs    []int
	zweights []big.Int
}

func (s smtWMaxCostOrder) Len() int { return len(s.costs) }
func (s smtWMaxCostOrder) Less(i, j int) bool {
	return s.zweights[s.costs[i]].Cmp(&s.zweights[s.costs[j]]) > 0
}
func (s smtWMaxCostOrder) Swap(i, j int) { s.costs[i], s.costs[j] = s.costs[j], s.costs[i] }

// sortedCosts is svector<theory_var> costs(m_costs);
// std::sort(costs.begin(), costs.end(), compare_cost), through the approved
// libstdc++ std::sort port (see pkg/util/sorting/libstdcxx_sort.go).
func (t *SMTWeightedMaxTheory) sortedCosts() []int {
	costs := append([]int(nil), t.costs...)
	sorting.LibstdcxxSort(smtWMaxCostOrder{costs: costs, zweights: t.zweights})
	return costs
}

// block is theory_wmaxsat::block.
func (t *SMTWeightedMaxTheory) block() error {
	if len(t.variables) == 0 {
		return nil
	}
	t.stats.blocks++
	state, err := t.State()
	if err != nil {
		return err
	}
	costs := t.sortedCosts()
	var weight big.Int
	literals := make([]SMTLiteral, 0, len(costs))
	for i := 0; i < len(costs) && weight.Cmp(&t.zminCost) < 0; i++ {
		weight.Add(&weight, &t.zweights[costs[i]])
		literals = append(literals, NewSMTLiteral(t.variables[costs[i]].boolean, false))
	}
	return state.SetTheoryConflict(t.FamilyID(), literals)
}

// Cost is theory_wmaxsat::get_cost: m_zcost / m_den.
func (t *SMTWeightedMaxTheory) Cost() (*big.Rat, error) {
	return new(big.Rat).SetFrac(&t.zcost, &t.den), nil
}

// IsOptimal is theory_wmaxsat::is_optimal.
func (t *SMTWeightedMaxTheory) IsOptimal() (bool, error) {
	return !t.foundOptimal || t.zcost.Cmp(&t.zminCost) < 0, nil
}

// MakeBlock is theory_wmaxsat::mk_block.
func (t *SMTWeightedMaxTheory) MakeBlock() (ast.ASTHandle, error) {
	t.stats.blocks++
	costs := t.sortedCosts()
	var weight big.Int
	literals := make([]ast.ASTHandle, 0, len(costs))
	refs := make([]ast.ASTRef, 0, len(costs))
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			_ = t.state.manager.DelRef(&refs[i])
		}
	}()
	for i := 0; i < len(costs) && weight.Cmp(&t.zminCost) < 0; i++ {
		tv := costs[i]
		entry := &t.variables[tv]
		if !entry.enabled {
			continue
		}
		weight.Add(&weight, &t.zweights[tv])
		negated, cause := t.state.manager.MkNot(entry.expr)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		ref, cause := t.state.manager.NewRef(negated)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		refs = append(refs, ref)
		literals = append(literals, negated)
	}
	if optimal, _ := t.IsOptimal(); optimal {
		t.foundOptimal = true
		t.costSave = append(t.costSave[:0], t.costs...)
	}
	return t.state.manager.MkOrArgs(literals)
}

func (t *SMTWeightedMaxTheory) DisableVariable(expression ast.ASTHandle) error {
	for i := range t.variables {
		if t.variables[i].expr == expression {
			t.variables[i].enabled = false
			return nil
		}
	}
	return ast.ErrASTDomain
}

// ResetLocal is theory_wmaxsat::reset_local. The source leaves m_costs and
// the trail entries of the last search in place; they are popped by the next
// pop_to_base_lvl before they are read again, except for entries made at the
// base level, which would index the reset vectors (undefined behavior in the
// source). They are dropped here instead.
func (t *SMTWeightedMaxTheory) ResetLocal() error {
	if _, err := t.State(); err != nil {
		return err
	}
	t.variables = nil
	t.rminCost.SetInt64(0)
	t.zweights = nil
	t.zcost.SetInt64(0)
	t.zminCost.SetInt64(0)
	t.costSave = nil
	t.byBoolean = make(map[int32]int)
	t.propagate = false
	t.canPropagate = false
	t.foundOptimal = false
	t.costs = nil
	t.trail = nil
	for i := range t.scopeLimits {
		t.scopeLimits[i] = smtWMaxScope{}
	}
	return nil
}

func (t *SMTWeightedMaxTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.scopeLimits = append(t.scopeLimits, smtWMaxScope{variables: len(t.variables), trail: len(t.trail)})
	return nil
}

func (t *SMTWeightedMaxTheory) PopScopes(scopes uint32) error {
	if scopes == 0 || scopes > uint32(len(t.scopeLimits)) {
		return ast.ErrASTDomain
	}
	level := len(t.scopeLimits) - int(scopes)
	limit := t.scopeLimits[level]
	if err := t.SMTTheoryBase.PopScopes(scopes); err != nil {
		return err
	}
	// Undo the assign_eh trail entries in reverse order.
	for i := len(t.trail) - 1; i >= limit.trail; i-- {
		entry := &t.trail[i]
		if entry.variable < len(t.variables) {
			t.variables[entry.variable].assigned = false
		}
		t.costs = t.costs[:len(t.costs)-1]
		t.zcost.Set(&entry.oldCost)
	}
	t.trail = t.trail[:limit.trail]
	for i := limit.variables; i < len(t.variables); i++ {
		delete(t.byBoolean, t.variables[i].boolean)
	}
	t.variables = t.variables[:limit.variables]
	if len(t.zweights) > limit.variables {
		t.zweights = t.zweights[:limit.variables]
	}
	t.scopeLimits = t.scopeLimits[:level]
	return nil
}

func (t *SMTWeightedMaxTheory) CollectStatistics(target *statistics.Statistics) error {
	if target == nil {
		return ast.ErrASTHandle
	}
	target.UpdateUint("wmaxsat num blocks", t.stats.blocks)
	target.UpdateUint("wmaxsat num props", t.stats.propagations)
	return nil
}

func (*SMTWeightedMaxTheory) InternalizeAtom(ast.ASTHandle, bool) (bool, error) { return false, nil }
func (*SMTWeightedMaxTheory) InternalizeEquality(ast.ASTHandle, int32) error    { return nil }
func (*SMTWeightedMaxTheory) InternalizeTerm(ast.ASTHandle) (bool, error)       { return false, nil }
func (*SMTWeightedMaxTheory) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error {
	return nil
}
func (*SMTWeightedMaxTheory) UseDisequalities() bool                          { return false }
func (*SMTWeightedMaxTheory) NewEquality(SMTTheoryVar, SMTTheoryVar) error    { return nil }
func (*SMTWeightedMaxTheory) NewDisequality(SMTTheoryVar, SMTTheoryVar) error { return nil }

// FinalCheck is theory_wmaxsat::final_check_eh: all blocking happens in
// assign_eh.
func (t *SMTWeightedMaxTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t.normalizing {
		t.normalize()
	}
	return SMTFinalCheckDone, nil
}

// Name is theory::get_name; theory_wmaxsat does not override it
// (AUDIT.md FUZZ-F5).
func (*SMTWeightedMaxTheory) Name() string { return "unknown" }
