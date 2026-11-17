package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h instantiated with i_ext
// (numeral rational) and mi_ext (numeral inf_rational), theory_arith.h.
// The rationals are on the AST manager's mpq manager (rational's
// g_mpq_manager); each owns its cell and is freed where the C++ vector or
// local holding it is destroyed.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtDenseExtI is theory_dense_i's numerals: rational, with
// m_int_epsilon = m_real_epsilon = rational(1).
type smtDenseExtI struct {
	q          *numeral.RationalManager
	dist       [][]numeral.Rational
	offsets    []numeral.Rational // parallel to the matrix edges
	trail      []numeral.Rational // cell_trail::m_old_distance
	atoms      []numeral.Rational
	assignment []numeral.Rational
	// targets is m_f_targets: its entries keep their values between calls.
	targets []smtDenseTargetI
	k       numeral.Rational
	newDist numeral.Rational // update_cells' new_dist
	negDist numeral.Rational // the negated distances of add_edge and propagate_using_cell
}

type smtDenseTargetI struct {
	target   int
	distance numeral.Rational
}

func (x *smtDenseExtI) addVariable(v int) {
	if len(x.offsets) == 0 {
		x.offsets = append(x.offsets, smtQZero()) // the null edge
		x.newDist, x.negDist = smtQZero(), smtQZero()
	}
	for i := range x.dist {
		x.dist[i] = append(x.dist[i], smtQZero())
	}
	row := make([]numeral.Rational, v+1)
	for i := range row {
		row[i] = smtQZero()
	}
	x.dist = append(x.dist, row)
	x.targets = append(x.targets, smtDenseTargetI{distance: smtQZero()})
}

func (x *smtDenseExtI) shrink(g *smtDenseDiffMatrix, variables, edges, trail int) error {
	q := x.q
	for i := len(x.trail) - 1; i >= trail; i-- {
		t := &g.trail[i]
		if err := q.Swap(&x.dist[t.source][t.target], &x.trail[i]); err != nil {
			return err
		}
	}
	var err error
	if x.trail, err = smtQDelTail(q, x.trail, trail); err != nil {
		return err
	}
	if x.offsets, err = smtQDelTail(q, x.offsets, edges); err != nil {
		return err
	}
	for i := range x.dist[:variables] {
		if x.dist[i], err = smtQDelTail(q, x.dist[i], variables); err != nil {
			return err
		}
	}
	for i := variables; i < len(x.dist); i++ {
		if _, err = smtQDelAll(q, x.dist[i]); err != nil {
			return err
		}
	}
	clear(x.dist[variables:])
	x.dist = x.dist[:variables]
	// m_f_targets.shrink(old_num_vars)
	for i := variables; i < len(x.targets); i++ {
		if err = smtQDel(q, &x.targets[i].distance); err != nil {
			return err
		}
	}
	if variables < len(x.targets) {
		clear(x.targets[variables:])
		x.targets = x.targets[:variables]
	}
	return nil
}

// smtQDelTail frees rs[n:] and returns rs[:n].
func smtQDelTail(q *numeral.RationalManager, rs []numeral.Rational, n int) ([]numeral.Rational, error) {
	if n >= len(rs) {
		return rs, nil
	}
	if _, err := smtQDelAll(q, rs[n:]); err != nil {
		return rs, err
	}
	return rs[:n], nil
}

func (x *smtDenseExtI) setK(r *numeral.Rational) error { return x.q.Copy(&x.k, r) }
func (x *smtDenseExtI) setKAtom(atom int) error        { return x.q.Copy(&x.k, &x.atoms[atom]) }
func (x *smtDenseExtI) negK() error                    { return x.q.Neg(&x.k) }
func (x *smtDenseExtI) subKEpsilon(bool) error {
	one := smtQInt(1)
	return x.q.Sub(&x.k, &one, &x.k)
}
func (x *smtDenseExtI) kText() string { return smtQText(x.q, &x.k) }
func (x *smtDenseExtI) pushAtom() error {
	a, err := smtQNew(x.q, &x.k)
	if err != nil {
		return err
	}
	x.atoms = append(x.atoms, a)
	return nil
}
func (x *smtDenseExtI) popAtoms(atoms int) error {
	var err error
	x.atoms, err = smtQDelTail(x.q, x.atoms, atoms)
	return err
}
func (x *smtDenseExtI) atomText(atom, width int) string {
	return fmt.Sprintf("%-*s", width, smtQText(x.q, &x.atoms[atom]))
}

func (x *smtDenseExtI) kConflicts(target, source int) (bool, error) {
	q := x.q
	if err := q.Copy(&x.negDist, &x.dist[target][source]); err != nil {
		return false, err
	}
	if err := q.Neg(&x.negDist); err != nil {
		return false, err
	}
	c, err := q.Compare(&x.negDist, &x.k)
	return c > 0, err
}
func (x *smtDenseExtI) kImproves(source, target int) (bool, error) {
	c, err := x.q.Compare(&x.k, &x.dist[source][target])
	return c < 0, err
}
func (x *smtDenseExtI) pushEdge() error {
	e, err := smtQNew(x.q, &x.k)
	if err != nil {
		return err
	}
	x.offsets = append(x.offsets, e)
	return nil
}

func (x *smtDenseExtI) updateCells(g *smtDenseDiffMatrix) error {
	q := x.q
	id := len(g.edges) - 1
	e := &g.edges[id]
	s, t := e.source, e.target
	k := &x.offsets[id]
	traced := g.owner != nil && smtTraceEnabled("ddl")
	newDist := &x.newDist
	count := 0
	tRow, tDist := g.rows[t], x.dist[t]
	sRow, sDist := g.rows[s], x.dist[s]
	for v := range tRow {
		if tRow[v] >= 0 && v != s {
			if err := q.Add(k, &tDist[v], newDist); err != nil {
				return err
			}
			if traced {
				g.traceUpdate(s, v, smtQText(q, newDist), sRow[v], func() string { return smtQText(q, &sDist[v]) })
			}
			better := sRow[v] < 0
			if !better {
				c, err := q.Compare(newDist, &sDist[v])
				if err != nil {
					return err
				}
				better = c < 0
			}
			if better {
				target := &x.targets[count]
				target.target = v
				if err := q.Copy(&target.distance, newDist); err != nil {
					return err
				}
				count++
			}
		}
	}
	targets := x.targets[:count]
	for y := range g.rows {
		if y == t {
			continue
		}
		row := g.rows[y]
		if row[s] < 0 {
			continue
		}
		dist := x.dist[y]
		dys := &dist[s]
		for i := range targets {
			v := targets[i].target
			if v == y {
				continue
			}
			if err := q.Add(dys, &targets[i].distance, newDist); err != nil {
				return err
			}
			better := row[v] < 0
			if !better {
				c, err := q.Compare(newDist, &dist[v])
				if err != nil {
					return err
				}
				better = c < 0
			}
			if better {
				old, err := smtQNew(q, &dist[v])
				if err != nil {
					return err
				}
				g.trail = append(g.trail, smtDenseDiffTrail{y, v, row[v]})
				x.trail = append(x.trail, old)
				row[v] = id
				if err = q.Copy(&dist[v], newDist); err != nil {
					return err
				}
				if err = g.propagate(y, v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (x *smtDenseExtI) distText(source, target, width int) string {
	return fmt.Sprintf("%-*s", width, smtQText(x.q, &x.dist[source][target]))
}

func (x *smtDenseExtI) implies(source, target, atom int, forward bool) (bool, error) {
	q := x.q
	if forward {
		c, err := q.Compare(&x.dist[source][target], &x.atoms[atom])
		return c <= 0, err
	}
	if err := q.Copy(&x.negDist, &x.dist[source][target]); err != nil {
		return false, err
	}
	if err := q.Neg(&x.negDist); err != nil {
		return false, err
	}
	c, err := q.Compare(&x.negDist, &x.atoms[atom])
	return c > 0, err
}

func (x *smtDenseExtI) initModel(g *smtDenseDiffMatrix) error {
	q := x.q
	var err error
	if x.assignment, err = smtQDelAll(q, x.assignment); err != nil {
		return err
	}
	variables := len(g.rows)
	for i := 0; i < variables; i++ {
		x.assignment = append(x.assignment, smtQZero())
		d := &x.assignment[i]
		row, dist := g.rows[i], x.dist[i]
		for j := 0; j < variables; j++ {
			if i != j && row[j] >= 0 {
				c, err := q.Compare(&dist[j], d)
				if err != nil {
					return err
				}
				if c < 0 {
					if err = q.Copy(d, &dist[j]); err != nil {
						return err
					}
				}
			}
		}
	}
	for i := range x.assignment {
		if err = q.Neg(&x.assignment[i]); err != nil {
			return err
		}
	}
	return nil
}

func (x *smtDenseExtI) assignmentText(v int) string { return smtQText(x.q, &x.assignment[v]) }
func (x *smtDenseExtI) assignmentIsZero(v int) bool { return x.assignment[v].IsZero() }
func (x *smtDenseExtI) fixZero(v int, same []bool) error {
	q := x.q
	val, err := smtQNew(q, &x.assignment[v])
	if err != nil {
		return err
	}
	for v2 := range x.assignment {
		if same[v2] {
			if err = q.Sub(&x.assignment[v2], &val, &x.assignment[v2]); err != nil {
				break
			}
		}
	}
	if e := smtQDel(q, &val); err == nil {
		err = e
	}
	return err
}
func (x *smtDenseExtI) assignmentParts(v int, n, k *numeral.Rational) error {
	if err := x.q.Copy(n, &x.assignment[v]); err != nil {
		return err
	}
	return x.q.SetInt32(k, 0)
}
func (x *smtDenseExtI) edgeParts(edge int, n, k *numeral.Rational) error {
	if err := x.q.Copy(n, &x.offsets[edge]); err != nil {
		return err
	}
	return x.q.SetInt32(k, 0)
}
func (x *smtDenseExtI) setAssignment(v int, r *numeral.Rational) error {
	return x.q.Copy(&x.assignment[v], r)
}

func (x *smtDenseExtI) close() error {
	q := x.q
	var errs []error
	keep := func(_ []numeral.Rational, err error) { errs = append(errs, err) }
	for i := range x.dist {
		keep(smtQDelAll(q, x.dist[i]))
	}
	keep(smtQDelAll(q, x.offsets))
	keep(smtQDelAll(q, x.trail))
	keep(smtQDelAll(q, x.atoms))
	keep(smtQDelAll(q, x.assignment))
	for i := range x.targets {
		errs = append(errs, smtQDel(q, &x.targets[i].distance))
	}
	for _, r := range []*numeral.Rational{&x.k, &x.newDist, &x.negDist} {
		errs = append(errs, smtQDel(q, r))
	}
	*x = smtDenseExtI{q: q, k: smtQZero()}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// smtDenseExtMI is theory_dense_mi's numerals: inf_rational, with
// m_int_epsilon = inf_rational(1) and m_real_epsilon = (0, 1).
type smtDenseExtMI struct {
	q          *numeral.RationalManager
	dist       [][]smtInfRational
	offsets    []smtInfRational
	trail      []smtInfRational
	atoms      []smtInfRational
	assignment []smtInfRational
	targets    []smtDenseTargetMI
	k          smtInfRational
	newDist    smtInfRational
	negDist    smtInfRational
}

type smtDenseTargetMI struct {
	target   int
	distance smtInfRational
}

func (x *smtDenseExtMI) addVariable(v int) {
	if len(x.offsets) == 0 {
		x.offsets = append(x.offsets, newSMTInfRational())
		x.newDist, x.negDist = newSMTInfRational(), newSMTInfRational()
	}
	for i := range x.dist {
		x.dist[i] = append(x.dist[i], newSMTInfRational())
	}
	row := make([]smtInfRational, v+1)
	for i := range row {
		row[i] = newSMTInfRational()
	}
	x.dist = append(x.dist, row)
	x.targets = append(x.targets, smtDenseTargetMI{distance: newSMTInfRational()})
}

// smtInfRationalDelTail frees vs[n:] and returns vs[:n].
func smtInfRationalDelTail(q *numeral.RationalManager, vs []smtInfRational, n int) ([]smtInfRational, error) {
	if n >= len(vs) {
		return vs, nil
	}
	var err error
	for i := n; i < len(vs); i++ {
		if e := vs[i].del(q); err == nil {
			err = e
		}
	}
	clear(vs[n:])
	return vs[:n], err
}

func (x *smtDenseExtMI) shrink(g *smtDenseDiffMatrix, variables, edges, trail int) error {
	q := x.q
	for i := len(x.trail) - 1; i >= trail; i-- {
		t := &g.trail[i]
		cell := &x.dist[t.source][t.target]
		*cell, x.trail[i] = x.trail[i], *cell
	}
	var err error
	if x.trail, err = smtInfRationalDelTail(q, x.trail, trail); err != nil {
		return err
	}
	if x.offsets, err = smtInfRationalDelTail(q, x.offsets, edges); err != nil {
		return err
	}
	for i := range x.dist[:variables] {
		if x.dist[i], err = smtInfRationalDelTail(q, x.dist[i], variables); err != nil {
			return err
		}
	}
	for i := variables; i < len(x.dist); i++ {
		if _, err = smtInfRationalDelTail(q, x.dist[i], 0); err != nil {
			return err
		}
	}
	clear(x.dist[variables:])
	x.dist = x.dist[:variables]
	for i := variables; i < len(x.targets); i++ {
		if err = x.targets[i].distance.del(q); err != nil {
			return err
		}
	}
	if variables < len(x.targets) {
		clear(x.targets[variables:])
		x.targets = x.targets[:variables]
	}
	return nil
}

func (x *smtDenseExtMI) setK(r *numeral.Rational) error { return x.k.setRational(x.q, r) }
func (x *smtDenseExtMI) setKAtom(atom int) error        { return x.k.set(x.q, &x.atoms[atom]) }
func (x *smtDenseExtMI) negK() error                    { return x.k.neg(x.q) }
func (x *smtDenseExtMI) subKEpsilon(real bool) error {
	epsilon := smtInfRational{first: smtQInt(1), second: smtQInt(0)}
	if real {
		epsilon = smtInfRational{first: smtQInt(0), second: smtQInt(1)}
	}
	return smtInfRationalSub(x.q, &x.k, &epsilon, &x.k)
}
func (x *smtDenseExtMI) kText() string { return smtInfRationalText(x.q, &x.k) }
func (x *smtDenseExtMI) pushAtom() error {
	a := newSMTInfRational()
	if err := a.set(x.q, &x.k); err != nil {
		return err
	}
	x.atoms = append(x.atoms, a)
	return nil
}
func (x *smtDenseExtMI) popAtoms(atoms int) error {
	var err error
	x.atoms, err = smtInfRationalDelTail(x.q, x.atoms, atoms)
	return err
}
func (x *smtDenseExtMI) atomText(atom, width int) string {
	return fmt.Sprintf("%-*s", width, smtInfRationalText(x.q, &x.atoms[atom]))
}

func (x *smtDenseExtMI) kConflicts(target, source int) (bool, error) {
	q := x.q
	if err := x.negDist.set(q, &x.dist[target][source]); err != nil {
		return false, err
	}
	if err := x.negDist.neg(q); err != nil {
		return false, err
	}
	c, err := smtInfRationalCmp(q, &x.negDist, &x.k)
	return c > 0, err
}
func (x *smtDenseExtMI) kImproves(source, target int) (bool, error) {
	c, err := smtInfRationalCmp(x.q, &x.k, &x.dist[source][target])
	return c < 0, err
}
func (x *smtDenseExtMI) pushEdge() error {
	e := newSMTInfRational()
	if err := e.set(x.q, &x.k); err != nil {
		return err
	}
	x.offsets = append(x.offsets, e)
	return nil
}

func (x *smtDenseExtMI) updateCells(g *smtDenseDiffMatrix) error {
	q := x.q
	id := len(g.edges) - 1
	e := &g.edges[id]
	s, t := e.source, e.target
	k := &x.offsets[id]
	traced := g.owner != nil && smtTraceEnabled("ddl")
	newDist := &x.newDist
	count := 0
	tRow, tDist := g.rows[t], x.dist[t]
	sRow, sDist := g.rows[s], x.dist[s]
	for v := range tRow {
		if tRow[v] >= 0 && v != s {
			if err := smtInfRationalAdd(q, k, &tDist[v], newDist); err != nil {
				return err
			}
			if traced {
				g.traceUpdate(s, v, smtInfRationalText(q, newDist), sRow[v], func() string { return smtInfRationalText(q, &sDist[v]) })
			}
			better := sRow[v] < 0
			if !better {
				c, err := smtInfRationalCmp(q, newDist, &sDist[v])
				if err != nil {
					return err
				}
				better = c < 0
			}
			if better {
				target := &x.targets[count]
				target.target = v
				if err := target.distance.set(q, newDist); err != nil {
					return err
				}
				count++
			}
		}
	}
	targets := x.targets[:count]
	for y := range g.rows {
		if y == t {
			continue
		}
		row := g.rows[y]
		if row[s] < 0 {
			continue
		}
		dist := x.dist[y]
		dys := &dist[s]
		for i := range targets {
			v := targets[i].target
			if v == y {
				continue
			}
			if err := smtInfRationalAdd(q, dys, &targets[i].distance, newDist); err != nil {
				return err
			}
			better := row[v] < 0
			if !better {
				c, err := smtInfRationalCmp(q, newDist, &dist[v])
				if err != nil {
					return err
				}
				better = c < 0
			}
			if better {
				old := newSMTInfRational()
				if err := old.set(q, &dist[v]); err != nil {
					return err
				}
				g.trail = append(g.trail, smtDenseDiffTrail{y, v, row[v]})
				x.trail = append(x.trail, old)
				row[v] = id
				if err := dist[v].set(q, newDist); err != nil {
					return err
				}
				if err := g.propagate(y, v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (x *smtDenseExtMI) distText(source, target, width int) string {
	return fmt.Sprintf("%-*s", width, smtInfRationalText(x.q, &x.dist[source][target]))
}

func (x *smtDenseExtMI) implies(source, target, atom int, forward bool) (bool, error) {
	q := x.q
	if forward {
		c, err := smtInfRationalCmp(q, &x.dist[source][target], &x.atoms[atom])
		return c <= 0, err
	}
	if err := x.negDist.set(q, &x.dist[source][target]); err != nil {
		return false, err
	}
	if err := x.negDist.neg(q); err != nil {
		return false, err
	}
	c, err := smtInfRationalCmp(q, &x.negDist, &x.atoms[atom])
	return c > 0, err
}

func (x *smtDenseExtMI) initModel(g *smtDenseDiffMatrix) error {
	q := x.q
	var err error
	if x.assignment, err = smtInfRationalDelTail(q, x.assignment, 0); err != nil {
		return err
	}
	variables := len(g.rows)
	for i := 0; i < variables; i++ {
		x.assignment = append(x.assignment, newSMTInfRational())
		d := &x.assignment[i]
		row, dist := g.rows[i], x.dist[i]
		for j := 0; j < variables; j++ {
			if i != j && row[j] >= 0 {
				c, err := smtInfRationalCmp(q, &dist[j], d)
				if err != nil {
					return err
				}
				if c < 0 {
					if err = d.set(q, &dist[j]); err != nil {
						return err
					}
				}
			}
		}
	}
	for i := range x.assignment {
		if err = x.assignment[i].neg(q); err != nil {
			return err
		}
	}
	return nil
}

func (x *smtDenseExtMI) assignmentText(v int) string {
	return smtInfRationalText(x.q, &x.assignment[v])
}
func (x *smtDenseExtMI) assignmentIsZero(v int) bool { return x.assignment[v].isZero() }
func (x *smtDenseExtMI) fixZero(v int, same []bool) error {
	q := x.q
	val := newSMTInfRational()
	err := val.set(q, &x.assignment[v])
	for v2 := range x.assignment {
		if err == nil && same[v2] {
			err = smtInfRationalSub(q, &x.assignment[v2], &val, &x.assignment[v2])
		}
	}
	if e := val.del(q); err == nil {
		err = e
	}
	return err
}
func (x *smtDenseExtMI) assignmentParts(v int, n, k *numeral.Rational) error {
	if err := x.q.Copy(n, &x.assignment[v].first); err != nil {
		return err
	}
	return x.q.Copy(k, &x.assignment[v].second)
}
func (x *smtDenseExtMI) edgeParts(edge int, n, k *numeral.Rational) error {
	if err := x.q.Copy(n, &x.offsets[edge].first); err != nil {
		return err
	}
	return x.q.Copy(k, &x.offsets[edge].second)
}
func (x *smtDenseExtMI) setAssignment(v int, r *numeral.Rational) error {
	return x.assignment[v].setRational(x.q, r)
}

func (x *smtDenseExtMI) close() error {
	q := x.q
	var errs []error
	keep := func(_ []smtInfRational, err error) { errs = append(errs, err) }
	for i := range x.dist {
		keep(smtInfRationalDelTail(q, x.dist[i], 0))
	}
	keep(smtInfRationalDelTail(q, x.offsets, 0))
	keep(smtInfRationalDelTail(q, x.trail, 0))
	keep(smtInfRationalDelTail(q, x.atoms, 0))
	keep(smtInfRationalDelTail(q, x.assignment, 0))
	for i := range x.targets {
		errs = append(errs, x.targets[i].distance.del(q))
	}
	for _, v := range []*smtInfRational{&x.k, &x.newDist, &x.negDist} {
		errs = append(errs, v.del(q))
	}
	*x = smtDenseExtMI{q: q, k: newSMTInfRational()}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// smtInfRationalText is inf_rational::to_string for a trace operand.
func smtInfRationalText(q *numeral.RationalManager, v *smtInfRational) string {
	text, err := v.text(q)
	if err != nil {
		return fmt.Sprint(err)
	}
	return text
}
