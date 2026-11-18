package simplex

// Source: src/math/simplex/simplex.h, simplex_def.h. As in the source, the
// solver owns its numeral managers (simplex's m and em): its coefficients
// and mpq_inf values are rationals of a private rational manager, created
// from the caller's numeral service by NewSolver and closed by Close.
// Values cross the API on the service's manager (the AST manager's
// rational manager), copied with CopyFrom.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

// solverValue is the solver's eps_numeral (mpq_inf): Main + Epsilon*eps, on
// the solver's private manager.
type solverValue struct{ Main, Epsilon numeral.Rational }

func newSolverValue() solverValue {
	return solverValue{Main: numeral.NewRational(), Epsilon: numeral.NewRational()}
}

func (o *ratCalc) epsDel(a *solverValue) {
	o.del(&a.Epsilon)
	o.del(&a.Main)
}
func (o *ratCalc) epsSet(a, b *solverValue) {
	o.set(&a.Main, &b.Main)
	o.set(&a.Epsilon, &b.Epsilon)
}
func (o *ratCalc) epsClone(b *solverValue) solverValue {
	a := newSolverValue()
	o.epsSet(&a, b)
	return a
}
func (o *ratCalc) epsAdd(a, b *solverValue) {
	o.add(&a.Main, &a.Main, &b.Main)
	o.add(&a.Epsilon, &a.Epsilon, &b.Epsilon)
}
func (o *ratCalc) epsSub(a, b *solverValue) {
	o.sub(&a.Main, &a.Main, &b.Main)
	o.sub(&a.Epsilon, &a.Epsilon, &b.Epsilon)
}

// epsAddMul is a += b*r.
func (o *ratCalc) epsAddMul(a, b *solverValue, r *numeral.Rational) {
	o.addMul(&a.Main, &a.Main, &b.Main, r)
	o.addMul(&a.Epsilon, &a.Epsilon, &b.Epsilon, r)
}
func (o *ratCalc) epsNeg(a *solverValue) {
	o.neg(&a.Main)
	o.neg(&a.Epsilon)
}
func (o *ratCalc) epsMul(a *solverValue, r *numeral.Rational) {
	o.mul(&a.Main, &a.Main, r)
	o.mul(&a.Epsilon, &a.Epsilon, r)
}
func (o *ratCalc) epsDiv(a *solverValue, r *numeral.Rational) {
	o.div(&a.Main, &a.Main, r)
	o.div(&a.Epsilon, &a.Epsilon, r)
}
func (o *ratCalc) epsSign(a *solverValue) int {
	if s := o.sign(&a.Main); s != 0 {
		return s
	}
	return o.sign(&a.Epsilon)
}
func (o *ratCalc) epsCmp(a, b *solverValue) int {
	if c := o.cmp(&a.Main, &b.Main); c != 0 {
		return c
	}
	return o.cmp(&a.Epsilon, &b.Epsilon)
}
func (o *ratCalc) epsIsZero(a *solverValue) bool { return o.epsSign(a) == 0 }
func (o *ratCalc) epsAbs(a *solverValue) {
	if o.epsSign(a) < 0 {
		o.epsNeg(a)
	}
}
func (o *ratCalc) epsText(a *solverValue) string {
	text := o.text(&a.Main)
	if o.sign(&a.Epsilon) != 0 {
		text += "+" + o.text(&a.Epsilon) + "*epsilon"
	}
	return text
}

type variableInfo struct {
	row                          uint32
	base, lowerValid, upperValid bool
	value, lower, upper          solverValue
	baseCoeff                    numeral.Rational
}

func newVariableInfo() variableInfo {
	return variableInfo{value: newSolverValue(), lower: newSolverValue(), upper: newSolverValue(), baseCoeff: numeral.NewRational()}
}

func (o *ratCalc) delVariableInfo(x *variableInfo) {
	o.del(&x.baseCoeff)
	o.epsDel(&x.upper)
	o.epsDel(&x.lower)
	o.epsDel(&x.value)
}

type Stats struct{ NumPivots, NumInfeasible, NumChecks uint32 }

// Solver is the concrete exact tableau used by both retained mpz and mpq
// consumers. Integer rows are rational rows with integral coefficients.
type Solver struct {
	c               ratCalc                  // the private manager (m, em)
	ext             *numeral.RationalManager // the callers' manager
	limit           *control.Reslimit
	matrix          Matrix
	vars            []variableInfo
	rowBase         []uint32
	patch           simplexVariableHeap
	maxIterations   uint32
	arithmeticBland bool
	random          z3rand.Generator // m_random, seed 0
	infeasible      uint32
	Stats           Stats
}

// NewSolver returns a solver whose numerals live on a private rational
// manager of numerals; API values are rationals of numerals.Manager.
func NewSolver(limit *control.Reslimit, numerals *numeral.RationalService) (*Solver, error) {
	if numerals == nil || numerals.Manager == nil {
		return nil, fmt.Errorf("simplex: no numeral service")
	}
	q, err := numerals.NewPrivateRationalManager()
	if err != nil {
		return nil, err
	}
	s := &Solver{ext: numerals.Manager, limit: limit, maxIterations: ^uint32(0), infeasible: NullVariable}
	s.c.q = q
	s.matrix.c = &s.c
	return s, nil
}

// Close is ~simplex: it frees the solver's numerals and closes its manager.
func (s *Solver) Close() error {
	if s == nil || s.c.q == nil {
		return nil
	}
	for i := range s.vars {
		s.c.delVariableInfo(&s.vars[i])
	}
	s.vars = nil
	err := errors.Join(s.c.take(), s.matrix.Close())
	if err != nil {
		return err
	}
	if err = s.c.q.CloseChecked(); err != nil {
		return err
	}
	s.c.q = nil
	return nil
}

// Numerals is the solver's own rational manager: RowEntries' and
// BaseCoefficient's borrowed coefficients are its rationals.
func (s *Solver) Numerals() *numeral.RationalManager { return s.c.q }

// result returns err joined with the first failed numeral operation.
func (s *Solver) result(err error) error { return s.c.joined(err) }

// in returns a fresh solver value copied from the callers' b (b's Infinity
// is not part of an eps_numeral).
func (s *Solver) in(b *Infinitesimal) solverValue {
	r := newSolverValue()
	if s.c.ok() {
		s.c.fail(s.c.q.CopyFrom(&r.Main, s.ext, &b.Main))
	}
	if s.c.ok() {
		s.c.fail(s.c.q.CopyFrom(&r.Epsilon, s.ext, &b.Epsilon))
	}
	return r
}

// out sets the callers' out to a.
func (s *Solver) out(out *Infinitesimal, a *solverValue) {
	if s.c.ok() {
		s.c.fail(s.ext.SetInt64(&out.Infinity, 0))
	}
	if s.c.ok() {
		s.c.fail(s.ext.CopyFrom(&out.Main, s.c.q, &a.Main))
	}
	if s.c.ok() {
		s.c.fail(s.ext.CopyFrom(&out.Epsilon, s.c.q, &a.Epsilon))
	}
}

// outZero sets the callers' out to 0.
func (s *Solver) outZero(out *Infinitesimal) {
	zero := ZeroInfinitesimal()
	if s.c.ok() {
		s.c.fail(out.Set(s.ext, &zero))
	}
}

func (s *Solver) EnsureVariable(v uint32) {
	for uint32(len(s.vars)) <= v {
		s.matrix.EnsureVariable(uint32(len(s.vars)))
		s.vars = append(s.vars, newVariableInfo())
	}
	s.patch.ensure(v)
}
func (s *Solver) Matrix() *Matrix           { return &s.matrix }
func (s *Solver) SetMaxIterations(n uint32) { s.maxIterations = n }

func (s *Solver) isBase(v uint32) bool  { return int(v) < len(s.vars) && s.vars[v].base }
func (s *Solver) outside(v uint32) bool { return s.belowLower(v) || s.aboveUpper(v) }
func (s *Solver) belowLower(v uint32) bool {
	x := &s.vars[v]
	return x.lowerValid && s.c.epsCmp(&x.value, &x.lower) < 0
}
func (s *Solver) aboveUpper(v uint32) bool {
	x := &s.vars[v]
	return x.upperValid && s.c.epsCmp(&x.value, &x.upper) > 0
}
func (s *Solver) aboveLower(v uint32) bool {
	x := &s.vars[v]
	return !x.lowerValid || s.c.epsCmp(&x.value, &x.lower) > 0
}
func (s *Solver) belowUpper(v uint32) bool {
	x := &s.vars[v]
	return !x.upperValid || s.c.epsCmp(&x.value, &x.upper) < 0
}
func (s *Solver) addPatch(v uint32) {
	if s.isBase(v) && s.outside(v) {
		s.patch.insert(v)
	}
}

// AddRationalRow is add_row; the coefficients are rationals of the
// callers' manager.
func (s *Solver) AddRationalRow(base uint32, variables []uint32, coefficients []numeral.Rational) (Row, error) {
	if len(variables) != len(coefficients) {
		return Row{}, fmt.Errorf("simplex: row length mismatch")
	}
	local := make([]numeral.Rational, len(coefficients))
	for i := range local {
		local[i] = numeral.NewRational()
		if s.c.ok() {
			s.c.fail(s.c.q.CopyFrom(&local[i], s.ext, &coefficients[i]))
		}
	}
	r, err := s.addRow(base, variables, local)
	s.c.delAll(local)
	return r, s.result(err)
}

// AddInt64Row is add_row with small integral coefficients.
func (s *Solver) AddInt64Row(base uint32, variables []uint32, coefficients []int64) (Row, error) {
	if len(variables) != len(coefficients) {
		return Row{}, fmt.Errorf("simplex: row length mismatch")
	}
	local := make([]numeral.Rational, len(coefficients))
	for i := range local {
		local[i] = s.c.int64(coefficients[i])
	}
	r, err := s.addRow(base, variables, local)
	s.c.delAll(local)
	return r, s.result(err)
}

func (s *Solver) addRow(base uint32, variables []uint32, coefficients []numeral.Rational) (Row, error) {
	if !s.c.ok() {
		return Row{}, nil
	}
	s.EnsureVariable(base)
	if s.isBase(base) {
		return Row{}, fmt.Errorf("simplex: base variable %d is already basic", base)
	}
	r := s.matrix.NewRow()
	// Source add_row records the input positions of basic variables
	// (m_base_vars) while adding the entries, then eliminates them in that
	// order, one row addition each.
	var baseVars []uint32
	for i, v := range variables {
		s.EnsureVariable(v)
		if s.c.sign(&coefficients[i]) != 0 {
			if s.isBase(v) {
				baseVars = append(baseVars, v)
			}
			if err := s.matrix.AddRational(r, &coefficients[i], v); err != nil {
				_ = s.matrix.Delete(r)
				return Row{}, err
			}
		}
	}
	// A rational multiple is algebraically equivalent to the source LCM
	// scaling; scaling a row never changes its entry layout.
	factor := numeral.NewRational()
	defer s.c.del(&factor)
	for _, v := range baseVars {
		b := &s.vars[v]
		a, _ := s.matrix.coefficient(r, v)
		s.c.div(&factor, a, &b.baseCoeff)
		s.c.neg(&factor)
		if err := s.matrix.Add(r, &factor, Row{b.row}); err != nil {
			_ = s.matrix.Delete(r)
			return Row{}, err
		}
	}
	baseCoeff, ok := s.matrix.coefficient(r, base)
	if !ok || s.c.sign(baseCoeff) == 0 {
		_ = s.matrix.Delete(r)
		return Row{}, fmt.Errorf("simplex: base variable has zero coefficient")
	}
	value := newSolverValue()
	defer s.c.epsDel(&value)
	entries, _ := s.matrix.Entries(r)
	for _, e := range entries {
		if e.Variable != base {
			s.c.epsAddMul(&value, &s.vars[e.Variable].value, e.Coefficient)
		}
	}
	s.c.epsNeg(&value)
	s.c.epsDiv(&value, baseCoeff)
	for len(s.rowBase) <= int(r.id) {
		s.rowBase = append(s.rowBase, NullVariable)
	}
	s.rowBase[r.id] = base
	v := &s.vars[base]
	v.row = r.id
	v.base = true
	s.c.set(&v.baseCoeff, baseCoeff)
	s.c.epsSet(&v.value, &value)
	s.addPatch(base)
	return r, nil
}

func (s *Solver) updateValueCore(v uint32, delta *solverValue) {
	s.c.epsAdd(&s.vars[v].value, delta)
	s.addPatch(v)
}
func (s *Solver) updateValue(v uint32, delta *solverValue) {
	if s.c.epsIsZero(delta) {
		return
	}
	s.updateValueCore(v, delta)
	d := newSolverValue()
	defer s.c.epsDel(&d)
	for _, r := range s.matrix.ColumnRows(v) {
		base := s.rowBase[r.id]
		// Source update_value also visits a basic variable's own row.
		// Its opposite contribution cancels the initial delta, preserving
		// the canonical equation when a caller seeds a basic variable.
		coeff, _ := s.matrix.coefficient(r, v)
		s.c.epsSet(&d, delta)
		s.c.epsMul(&d, coeff)
		s.c.epsDiv(&d, &s.vars[base].baseCoeff)
		s.c.epsNeg(&d)
		s.updateValueCore(base, &d)
	}
}

// SetLower is set_lower(v, bound); bound is on the callers' manager.
func (s *Solver) SetLower(v uint32, bound *Infinitesimal) error {
	s.EnsureVariable(v)
	b := s.in(bound)
	defer s.c.epsDel(&b)
	x := &s.vars[v]
	if x.upperValid && s.c.epsCmp(&b, &x.upper) > 0 {
		return s.result(fmt.Errorf("simplex: lower bound exceeds upper bound"))
	}
	s.c.epsSet(&x.lower, &b)
	x.lowerValid = true
	if !x.base && s.c.epsCmp(&x.value, &b) < 0 {
		d := s.c.epsClone(&b)
		s.c.epsSub(&d, &x.value)
		s.updateValue(v, &d)
		s.c.epsDel(&d)
	} else {
		s.addPatch(v)
	}
	return s.result(nil)
}

// SetUpper is set_upper(v, bound); bound is on the callers' manager.
func (s *Solver) SetUpper(v uint32, bound *Infinitesimal) error {
	s.EnsureVariable(v)
	b := s.in(bound)
	defer s.c.epsDel(&b)
	x := &s.vars[v]
	if x.lowerValid && s.c.epsCmp(&x.lower, &b) > 0 {
		return s.result(fmt.Errorf("simplex: upper bound is below lower bound"))
	}
	s.c.epsSet(&x.upper, &b)
	x.upperValid = true
	if !x.base && s.c.epsCmp(&x.value, &b) > 0 {
		d := s.c.epsClone(&b)
		s.c.epsSub(&d, &x.value)
		s.updateValue(v, &d)
		s.c.epsDel(&d)
	} else {
		s.addPatch(v)
	}
	return s.result(nil)
}
func (s *Solver) UnsetLower(v uint32) {
	if int(v) < len(s.vars) {
		s.vars[v].lowerValid = false
	}
}
func (s *Solver) UnsetUpper(v uint32) {
	if int(v) < len(s.vars) {
		s.vars[v].upperValid = false
	}
}

// Lower sets the callers' out to v's lower bound (0 when v has none) and
// reports whether it is valid.
func (s *Solver) Lower(v uint32, out *Infinitesimal) (bool, error) {
	if int(v) >= len(s.vars) {
		s.outZero(out)
		return false, s.result(nil)
	}
	x := &s.vars[v]
	s.out(out, &x.lower)
	return x.lowerValid, s.result(nil)
}

// Upper sets the callers' out to v's upper bound (0 when v has none) and
// reports whether it is valid.
func (s *Solver) Upper(v uint32, out *Infinitesimal) (bool, error) {
	if int(v) >= len(s.vars) {
		s.outZero(out)
		return false, s.result(nil)
	}
	x := &s.vars[v]
	s.out(out, &x.upper)
	return x.upperValid, s.result(nil)
}

// Value sets the callers' out to get_value(v).
func (s *Solver) Value(v uint32, out *Infinitesimal) error {
	if int(v) >= len(s.vars) {
		s.outZero(out)
		return s.result(nil)
	}
	s.out(out, &s.vars[v].value)
	return s.result(nil)
}

// SetValue is set_value(v, value); value is on the callers' manager.
func (s *Solver) SetValue(v uint32, value *Infinitesimal) error {
	s.EnsureVariable(v)
	d := s.in(value)
	s.c.epsSub(&d, &s.vars[v].value)
	s.updateValue(v, &d)
	s.c.epsDel(&d)
	return s.result(nil)
}

// selectSmallestVar is simplex::select_smallest_var: a bare erase_min, so
// variables that are back in bounds still count as iterations.
func (s *Solver) selectSmallestVar() uint32 {
	v, ok := s.patch.popMin()
	if !ok {
		return NullVariable
	}
	return v
}

// blandsRuleThreshold is simplex::m_blands_rule_threshold.
const blandsRuleThreshold = 1000

func (s *Solver) selectPatch() uint32 {
	for {
		v, ok := s.patch.popMin()
		if !ok {
			return NullVariable
		}
		if s.outside(v) {
			return v
		}
	}
}
func (s *Solver) nonFreeDependencies(v uint32) int {
	n := 0
	x := &s.vars[v]
	if x.lowerValid || x.upperValid {
		n++
	}
	for _, r := range s.matrix.ColumnRows(v) {
		b := s.rowBase[r.id]
		if b == v {
			continue
		}
		x = &s.vars[b]
		if x.lowerValid || x.upperValid {
			n++
		}
	}
	return n
}

// selectPivot is select_pivot: select_pivot_blands under Bland's rule,
// otherwise select_pivot_core, which breaks (dependencies, column size) ties
// uniformly at random with m_random. The returned coefficient is a fresh
// copy the caller frees.
func (s *Solver) selectPivot(base uint32, below bool) (uint32, numeral.Rational) {
	entries, _ := s.matrix.Entries(Row{s.vars[base].row})
	result := NullVariable
	bestDep := int(^uint(0) >> 1)
	bestCol := ^uint32(0)
	n := uint32(0)
	for _, e := range entries {
		v := e.Variable
		if v == base {
			continue
		}
		sign := s.c.sign(e.Coefficient)
		negative := sign < 0
		wrongDirection := negative
		if !below {
			wrongDirection = sign > 0
		}
		can := (!wrongDirection && s.aboveLower(v)) || (wrongDirection && s.belowUpper(v))
		if !can {
			continue
		}
		if s.arithmeticBland {
			if v < result {
				result = v
			}
			continue
		}
		dep, col := s.nonFreeDependencies(v), s.matrix.ColumnSize(v)
		if dep < bestDep || (dep == bestDep && col < bestCol) {
			result = v
			bestDep = dep
			bestCol = col
			n = 1
		} else if dep == bestDep && col == bestCol {
			n++
			if s.random.Next()%n == 0 {
				result = v
			}
		}
	}
	if result == NullVariable {
		return result, numeral.NewRational()
	}
	c, _ := s.matrix.coefficient(Row{s.vars[base].row}, result)
	return result, s.c.clone(c)
}

func (s *Solver) pivot(oldBase, newBase uint32, pivotCoeff *numeral.Rational) error {
	r := Row{s.vars[oldBase].row}
	rows := s.matrix.ColumnRows(newBase)
	s.rowBase[r.id] = newBase
	n := &s.vars[newBase]
	n.row = r.id
	n.base = true
	s.c.set(&n.baseCoeff, pivotCoeff)
	s.vars[oldBase].base = false
	// The source walks newBase's column with a live col_iterator, whose
	// m_refs keeps the column from compressing while M.add frees its entries.
	s.matrix.retainColumn(newBase)
	defer s.matrix.releaseColumn(newBase)
	factor := numeral.NewRational()
	defer s.c.del(&factor)
	for _, other := range rows {
		if other == r {
			continue
		}
		akj, ok := s.matrix.coefficient(other, newBase)
		if !ok {
			continue
		}
		s.c.div(&factor, akj, pivotCoeff)
		s.c.neg(&factor)
		if err := s.matrix.Add(other, &factor, r); err != nil {
			return err
		}
		b := s.rowBase[other.id]
		coeff, ok := s.matrix.coefficient(other, b)
		if !ok {
			return fmt.Errorf("simplex: pivot lost base coefficient")
		}
		s.c.set(&s.vars[b].baseCoeff, coeff)
	}
	s.Stats.NumPivots++
	s.addPatch(newBase)
	return nil
}
func (s *Solver) updateAndPivot(oldBase, newBase uint32, pivotCoeff *numeral.Rational, newValue *solverValue) error {
	theta := s.c.epsClone(&s.vars[oldBase].value)
	s.c.epsSub(&theta, newValue)
	s.c.epsMul(&theta, &s.vars[oldBase].baseCoeff)
	s.c.epsDiv(&theta, pivotCoeff)
	s.updateValue(newBase, &theta)
	s.c.epsDel(&theta)
	return s.pivot(oldBase, newBase, pivotCoeff)
}

func (s *Solver) makeVariableFeasible(v uint32) (bool, error) {
	x := &s.vars[v]
	var target solverValue
	below := false
	if s.belowLower(v) {
		target = s.c.epsClone(&x.lower)
		below = s.c.sign(&x.baseCoeff) > 0
	} else if s.aboveUpper(v) {
		target = s.c.epsClone(&x.upper)
		below = s.c.sign(&x.baseCoeff) < 0
	} else {
		return true, nil
	}
	defer s.c.epsDel(&target)
	pivot, coeff := s.selectPivot(v, below)
	defer s.c.del(&coeff)
	if pivot == NullVariable {
		return false, nil
	}
	return true, s.updateAndPivot(v, pivot, &coeff, &target)
}

func (s *Solver) MakeFeasible() (lbool.LBool, error) {
	s.Stats.NumChecks++
	s.infeasible = NullVariable
	s.arithmeticBland = false
	left := make(map[uint32]bool)
	threshold := uint32(blandsRuleThreshold)
	var repeated uint32
	for iteration := uint32(0); ; iteration++ {
		if !s.c.ok() {
			return lbool.Undef, s.result(nil)
		}
		v := s.selectSmallestVar()
		if v == NullVariable {
			return lbool.True, s.result(nil)
		}
		if (s.limit != nil && !s.limit.Inc()) || iteration > s.maxIterations {
			return lbool.Undef, s.result(nil)
		}
		s.checkBlandsRule(v, left, &repeated, threshold)
		ok, err := s.makeVariableFeasible(v)
		if err != nil {
			return lbool.Undef, s.result(err)
		}
		if !ok {
			s.patch.insert(v)
			s.infeasible = v
			s.Stats.NumInfeasible++
			return lbool.False, s.result(nil)
		}
	}
}

func (s *Solver) InfeasibleRow() (Row, error) {
	if s.infeasible == NullVariable || !s.isBase(s.infeasible) {
		return Row{}, fmt.Errorf("simplex: no infeasible row")
	}
	return Row{s.vars[s.infeasible].row}, nil
}
func (s *Solver) BaseVariable(r Row) (uint32, error) {
	if !s.matrix.valid(r) || int(r.id) >= len(s.rowBase) || s.rowBase[r.id] == NullVariable {
		return 0, fmt.Errorf("simplex: row has no base")
	}
	return s.rowBase[r.id], nil
}

// BaseCoefficient is get_base_coeff: a borrowed rational of the solver's
// manager (Numerals), valid until the solver next changes.
func (s *Solver) BaseCoefficient(r Row) (*numeral.Rational, error) {
	v, e := s.BaseVariable(r)
	if e != nil {
		return nil, e
	}
	return &s.vars[v].baseCoeff, nil
}

// RowEntries iterates r; the coefficients are borrowed rationals of the
// solver's manager (Numerals), valid until the solver next changes.
func (s *Solver) RowEntries(r Row) ([]RowEntry, error) { return s.matrix.Entries(r) }

// BasicRows returns live rows in source row-vector order, including holes
// reused by later row creation.
func (s *Solver) BasicRows() []Row {
	var rows []Row
	for index, base := range s.rowBase {
		if base != NullVariable {
			rows = append(rows, Row{uint32(index)})
		}
	}
	return rows
}

// DeleteRow removes the row containing variable. A nonbasic variable is first
// pivoted into its first row, preserving the source operation order.
func (s *Solver) DeleteRow(variable uint32) error {
	if int(variable) >= len(s.vars) {
		return nil
	}
	var r Row
	if s.isBase(variable) {
		r = Row{s.vars[variable].row}
	} else {
		rows := s.matrix.ColumnRows(variable)
		if len(rows) == 0 {
			return nil
		}
		r = rows[0]
		old := s.rowBase[r.id]
		var target solverValue
		if s.belowLower(old) {
			target = s.c.epsClone(&s.vars[old].lower)
		} else if s.aboveUpper(old) {
			target = s.c.epsClone(&s.vars[old].upper)
		} else {
			target = s.c.epsClone(&s.vars[old].value)
		}
		c, _ := s.matrix.coefficient(r, variable)
		coeff := s.c.clone(c)
		err := s.updateAndPivot(old, variable, &coeff, &target)
		s.c.del(&coeff)
		s.c.epsDel(&target)
		if err != nil {
			return s.result(err)
		}
	}
	// Source del_row leaves the variable in m_to_patch; with its bounds
	// cleared, make_var_feasible accepts it when it is popped.
	base := s.rowBase[r.id]
	x := &s.vars[base]
	x.base, x.lowerValid, x.upperValid = false, false, false
	s.rowBase[r.id] = NullVariable
	return s.result(s.matrix.Delete(r))
}

func (s *Solver) atLower(v uint32) bool {
	x := &s.vars[v]
	return x.lowerValid && s.c.epsCmp(&x.value, &x.lower) == 0
}
func (s *Solver) atUpper(v uint32) bool {
	x := &s.vars[v]
	return x.upperValid && s.c.epsCmp(&x.value, &x.upper) == 0
}

// pickLeaving returns the leaving variable with its gain and a fresh copy
// of its x coefficient, which the caller frees (both are 0 without one).
func (s *Solver) pickLeaving(x uint32, increase bool) (uint32, solverValue, numeral.Rational, bool) {
	selected := NullVariable
	gain := newSolverValue()
	selectedCoeff := numeral.NewRational()
	selectedIncrease := false
	current := newSolverValue()
	defer s.c.epsDel(&current)
	for _, r := range s.matrix.ColumnRows(x) {
		base := s.rowBase[r.id]
		if base == x {
			continue
		}
		v := &s.vars[base]
		coeff, _ := s.matrix.coefficient(r, x)
		incBase := (s.c.sign(&v.baseCoeff) == s.c.sign(coeff)) != increase
		if (incBase && !v.upperValid) || (!incBase && !v.lowerValid) {
			continue
		}
		s.c.epsSet(&current, &v.value)
		if incBase {
			s.c.epsSub(&current, &v.upper)
		} else {
			s.c.epsSub(&current, &v.lower)
		}
		s.c.epsMul(&current, &v.baseCoeff)
		s.c.epsDiv(&current, coeff)
		s.c.epsAbs(&current)
		if selected == NullVariable || s.c.epsCmp(&current, &gain) < 0 || (s.c.epsIsZero(&current) && s.c.epsIsZero(&gain) && base < selected) {
			selected, selectedIncrease = base, incBase
			s.c.epsSet(&gain, &current)
			s.c.set(&selectedCoeff, coeff)
		}
	}
	return selected, gain, selectedCoeff, selectedIncrease
}

// selectPrimal returns the leaving and entering variables and a fresh copy
// of the pivot coefficient, which the caller frees.
func (s *Solver) selectPrimal(objective uint32) (leave, enter uint32, coeff numeral.Rational, incLeave, incEnter bool) {
	leave, enter = NullVariable, NullVariable
	coeff = numeral.NewRational()
	entries, _ := s.matrix.Entries(Row{s.vars[objective].row})
	best := newSolverValue()
	defer s.c.epsDel(&best)
	objectiveSign := s.c.sign(&s.vars[objective].baseCoeff)
	for _, e := range entries {
		x := e.Variable
		if x == objective {
			continue
		}
		increase := s.c.sign(e.Coefficient) == objectiveSign
		if (increase && s.atUpper(x)) || (!increase && s.atLower(x)) {
			continue
		}
		candidate, gain, a, inc := s.pickLeaving(x, increase)
		if candidate == NullVariable {
			s.c.epsDel(&gain)
			s.c.del(&coeff)
			return candidate, x, a, inc, increase
		}
		if leave == NullVariable || s.c.epsCmp(&gain, &best) > 0 || (s.c.epsIsZero(&gain) && s.c.epsIsZero(&best) && candidate < leave) {
			leave, enter, incLeave, incEnter = candidate, x, inc, increase
			s.c.set(&coeff, &a)
			s.c.epsSet(&best, &gain)
		}
		s.c.epsDel(&gain)
		s.c.del(&a)
	}
	return
}

func (s *Solver) moveToBound(v uint32, toLower bool) {
	x := &s.vars[v]
	var delta solverValue
	if toLower {
		delta = s.c.epsClone(&x.value)
		s.c.epsSub(&delta, &x.lower)
	} else {
		delta = s.c.epsClone(&x.upper)
		s.c.epsSub(&delta, &x.value)
	}
	defer s.c.epsDel(&delta)
	d := newSolverValue()
	defer s.c.epsDel(&d)
	for _, r := range s.matrix.ColumnRows(v) {
		base := s.rowBase[r.id]
		if base == v {
			continue
		}
		b := &s.vars[base]
		coeff, _ := s.matrix.coefficient(r, v)
		baseToLower := ((s.c.sign(coeff) > 0) != (s.c.sign(&b.baseCoeff) > 0)) == toLower
		var bound *solverValue
		if !baseToLower && b.upperValid {
			bound = &b.upper
		} else if baseToLower && b.lowerValid {
			bound = &b.lower
		}
		if bound != nil {
			s.c.epsSet(&d, bound)
			s.c.epsSub(&d, &b.value)
			s.c.epsMul(&d, &b.baseCoeff)
			s.c.epsDiv(&d, coeff)
			s.c.epsAbs(&d)
			if s.c.epsCmp(&d, &delta) < 0 {
				s.c.epsSet(&delta, &d)
			}
		}
	}
	if toLower {
		s.c.epsNeg(&delta)
	}
	s.updateValue(v, &delta)
}

// Minimize returns True for an optimum, False for an unbounded objective and
// Undef for a resource stop.
func (s *Solver) Minimize(objective uint32) (lbool.LBool, error) {
	if !s.isBase(objective) {
		return lbool.Undef, fmt.Errorf("simplex: objective is not basic")
	}
	for {
		if !s.c.ok() {
			return lbool.Undef, s.result(nil)
		}
		if s.limit != nil && !s.limit.Inc() {
			return lbool.Undef, s.result(nil)
		}
		leave, enter, coeff, incLeave, incEnter := s.selectPrimal(objective)
		if enter == NullVariable {
			s.c.del(&coeff)
			return lbool.True, s.result(nil)
		}
		if leave == NullVariable {
			s.c.del(&coeff)
			x := &s.vars[enter]
			var d solverValue
			if incEnter && x.upperValid {
				d = s.c.epsClone(&x.upper)
				s.c.epsSub(&d, &x.value)
			} else if !incEnter && x.lowerValid {
				d = s.c.epsClone(&x.lower)
				s.c.epsSub(&d, &x.value)
			} else {
				return lbool.False, s.result(nil)
			}
			s.updateValue(enter, &d)
			s.c.epsDel(&d)
			continue
		}
		err := s.pivot(leave, enter, &coeff)
		s.c.del(&coeff)
		if err != nil {
			return lbool.Undef, s.result(err)
		}
		s.moveToBound(leave, !incLeave)
	}
}

// Reset is simplex::reset; the numerals are freed.
func (s *Solver) Reset() {
	for i := range s.vars {
		s.c.delVariableInfo(&s.vars[i])
	}
	s.matrix.Reset()
	s.vars = nil
	s.rowBase = nil
	s.patch.clear()
	s.infeasible = NullVariable
}
func (s *Solver) Display(w io.Writer) error {
	if err := s.matrix.Display(w); err != nil {
		return err
	}
	for i := range s.vars {
		x := &s.vars[i]
		lo, hi := "-oo", "oo"
		if x.lowerValid {
			lo = s.c.epsText(&x.lower)
		}
		if x.upperValid {
			hi = s.c.epsText(&x.upper)
		}
		if _, err := fmt.Fprintf(w, "v%d %s [%s:%s]", i, s.c.epsText(&x.value), lo, hi); err != nil {
			return err
		}
		if x.base {
			if _, err := fmt.Fprintf(w, " b:%d", x.row); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return s.result(nil)
}

// checkBlandsRule is simplex::check_blands_rule; arithmeticBland is m_bland.
func (s *Solver) checkBlandsRule(v uint32, left map[uint32]bool, repeated *uint32, threshold uint32) {
	if s.arithmeticBland {
		return
	}
	if left[v] {
		(*repeated)++
		if *repeated > threshold {
			s.arithmeticBland = true
		}
	} else {
		left[v] = true
	}
}
