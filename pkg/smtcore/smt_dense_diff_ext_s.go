package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h instantiated with si_ext
// (numeral s_integer) and smi_ext (numeral inf_s_integer), theory_arith.h.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtDenseExtSI is theory_dense_si's numerals: s_integer, with
// m_int_epsilon = m_real_epsilon = s_integer(1).
type smtDenseExtSI struct {
	dist       [][]smtSInteger
	offsets    []smtSInteger // parallel to the matrix edges
	trail      []smtSInteger // cell_trail::m_old_distance
	atoms      []smtSInteger
	assignment []smtSInteger
	targets    []smtDenseTargetSI
	k          smtSInteger
	q          *numeral.RationalManager // rational conversions only
}

// smtDenseTargetSI is f_target.
type smtDenseTargetSI struct {
	target   int
	distance smtSInteger
}

func (x *smtDenseExtSI) addVariable(v int) {
	if len(x.offsets) == 0 {
		x.offsets = append(x.offsets, smtSInteger{}) // the null edge
	}
	for i := range x.dist {
		x.dist[i] = append(x.dist[i], smtSInteger{})
	}
	x.dist = append(x.dist, make([]smtSInteger, v+1))
}

func (x *smtDenseExtSI) shrink(g *smtDenseDiffMatrix, variables, edges, trail int) error {
	for i := len(x.trail) - 1; i >= trail; i-- {
		t := &g.trail[i]
		x.dist[t.source][t.target] = x.trail[i]
	}
	x.trail = x.trail[:trail]
	x.offsets = x.offsets[:edges]
	for i := range x.dist[:variables] {
		x.dist[i] = x.dist[i][:variables]
	}
	clear(x.dist[variables:])
	x.dist = x.dist[:variables]
	return nil
}

func (x *smtDenseExtSI) setK(r *numeral.Rational) error {
	x.k = smtSInteger{smtSIntegerOfRational(x.q, r)}
	return nil
}
func (x *smtDenseExtSI) setKAtom(atom int) error { x.k = x.atoms[atom]; return nil }
func (x *smtDenseExtSI) negK() error             { x.k = x.k.neg(); return nil }
func (x *smtDenseExtSI) subKEpsilon(bool) error  { x.k = x.k.sub(smtSInteger{1}); return nil }
func (x *smtDenseExtSI) kText() string           { return x.k.String() }
func (x *smtDenseExtSI) pushAtom() error         { x.atoms = append(x.atoms, x.k); return nil }
func (x *smtDenseExtSI) popAtoms(atoms int) error {
	x.atoms = x.atoms[:atoms]
	return nil
}
func (x *smtDenseExtSI) atomText(atom, width int) string {
	return fmt.Sprintf("%-*s", width, x.atoms[atom].String())
}

func (x *smtDenseExtSI) kConflicts(target, source int) (bool, error) {
	return x.dist[target][source].neg().val > x.k.val, nil
}
func (x *smtDenseExtSI) kImproves(source, target int) (bool, error) {
	return x.k.val < x.dist[source][target].val, nil
}
func (x *smtDenseExtSI) pushEdge() error { x.offsets = append(x.offsets, x.k); return nil }

func (x *smtDenseExtSI) updateCells(g *smtDenseDiffMatrix) error {
	id := len(g.edges) - 1
	e := &g.edges[id]
	s, t := e.source, e.target
	k := x.offsets[id]
	traced := g.owner != nil && smtTraceEnabled("ddl")
	targets := x.targets[:0]
	tRow, tDist := g.rows[t], x.dist[t]
	sRow, sDist := g.rows[s], x.dist[s]
	for v := range tRow {
		if tRow[v] >= 0 && v != s {
			newDist := k.add(tDist[v])
			if traced {
				old := sDist[v]
				g.traceUpdate(s, v, newDist.String(), sRow[v], old.String)
			}
			if sRow[v] < 0 || newDist.val < sDist[v].val {
				targets = append(targets, smtDenseTargetSI{v, newDist})
			}
		}
	}
	x.targets = targets
	for y := range g.rows {
		if y == t {
			continue
		}
		row := g.rows[y]
		if row[s] < 0 {
			continue
		}
		dist := x.dist[y]
		dys := dist[s]
		for i := range targets {
			v := targets[i].target
			if v == y {
				continue
			}
			newDist := dys.add(targets[i].distance)
			if row[v] < 0 || newDist.val < dist[v].val {
				g.trail = append(g.trail, smtDenseDiffTrail{y, v, row[v]})
				x.trail = append(x.trail, dist[v])
				row[v] = id
				dist[v] = newDist
				if err := g.propagate(y, v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (x *smtDenseExtSI) distText(source, target, width int) string {
	return fmt.Sprintf("%-*s", width, x.dist[source][target].String())
}

func (x *smtDenseExtSI) implies(source, target, atom int, forward bool) (bool, error) {
	d := x.dist[source][target]
	if forward {
		return d.val <= x.atoms[atom].val, nil
	}
	return d.neg().val > x.atoms[atom].val, nil
}

func (x *smtDenseExtSI) initModel(g *smtDenseDiffMatrix) error {
	variables := len(g.rows)
	x.assignment = x.assignment[:0]
	for i := 0; i < variables; i++ {
		var d smtSInteger
		row, dist := g.rows[i], x.dist[i]
		for j := 0; j < variables; j++ {
			if i != j && row[j] >= 0 && dist[j].val < d.val {
				d = dist[j]
			}
		}
		x.assignment = append(x.assignment, d)
	}
	for i := range x.assignment {
		x.assignment[i] = x.assignment[i].neg()
	}
	return nil
}

func (x *smtDenseExtSI) assignmentText(v int) string { return x.assignment[v].String() }
func (x *smtDenseExtSI) assignmentIsZero(v int) bool { return x.assignment[v].val == 0 }
func (x *smtDenseExtSI) fixZero(v int, same []bool) error {
	val := x.assignment[v]
	for v2 := range x.assignment {
		if same[v2] {
			x.assignment[v2] = x.assignment[v2].sub(val)
		}
	}
	return nil
}
func (x *smtDenseExtSI) assignmentParts(v int, n, k *numeral.Rational) error {
	return smtDenseSIntegerParts(x.q, n, k, x.assignment[v].val, 0)
}
func (x *smtDenseExtSI) edgeParts(edge int, n, k *numeral.Rational) error {
	return smtDenseSIntegerParts(x.q, n, k, x.offsets[edge].val, 0)
}
func (x *smtDenseExtSI) setAssignment(v int, r *numeral.Rational) error {
	x.assignment[v] = smtSInteger{smtSIntegerOfRational(x.q, r)}
	return nil
}
func (x *smtDenseExtSI) close() error { *x = smtDenseExtSI{q: x.q}; return nil }

// smtDenseExtSMI is theory_dense_smi's numerals: inf_s_integer, with
// m_int_epsilon = inf_s_integer(1) and m_real_epsilon = (0, 1).
type smtDenseExtSMI struct {
	dist       [][]smtInfSInteger
	offsets    []smtInfSInteger
	trail      []smtInfSInteger
	atoms      []smtInfSInteger
	assignment []smtInfSInteger
	targets    []smtDenseTargetSMI
	k          smtInfSInteger
	q          *numeral.RationalManager // rational conversions only
}

type smtDenseTargetSMI struct {
	target   int
	distance smtInfSInteger
}

func (x *smtDenseExtSMI) addVariable(v int) {
	if len(x.offsets) == 0 {
		x.offsets = append(x.offsets, smtInfSInteger{})
	}
	for i := range x.dist {
		x.dist[i] = append(x.dist[i], smtInfSInteger{})
	}
	x.dist = append(x.dist, make([]smtInfSInteger, v+1))
}

func (x *smtDenseExtSMI) shrink(g *smtDenseDiffMatrix, variables, edges, trail int) error {
	for i := len(x.trail) - 1; i >= trail; i-- {
		t := &g.trail[i]
		x.dist[t.source][t.target] = x.trail[i]
	}
	x.trail = x.trail[:trail]
	x.offsets = x.offsets[:edges]
	for i := range x.dist[:variables] {
		x.dist[i] = x.dist[i][:variables]
	}
	clear(x.dist[variables:])
	x.dist = x.dist[:variables]
	return nil
}

func (x *smtDenseExtSMI) setK(r *numeral.Rational) error {
	x.k = smtInfSInteger{first: smtSIntegerOfRational(x.q, r)}
	return nil
}
func (x *smtDenseExtSMI) setKAtom(atom int) error { x.k = x.atoms[atom]; return nil }
func (x *smtDenseExtSMI) negK() error             { x.k = x.k.neg(); return nil }
func (x *smtDenseExtSMI) subKEpsilon(real bool) error {
	if real {
		x.k = x.k.sub(smtInfSInteger{0, 1})
	} else {
		x.k = x.k.sub(smtInfSInteger{1, 0})
	}
	return nil
}
func (x *smtDenseExtSMI) kText() string   { return x.k.String() }
func (x *smtDenseExtSMI) pushAtom() error { x.atoms = append(x.atoms, x.k); return nil }
func (x *smtDenseExtSMI) popAtoms(atoms int) error {
	x.atoms = x.atoms[:atoms]
	return nil
}
func (x *smtDenseExtSMI) atomText(atom, width int) string { return x.atoms[atom].widthText(width) }

func (x *smtDenseExtSMI) kConflicts(target, source int) (bool, error) {
	return x.k.less(x.dist[target][source].neg()), nil
}
func (x *smtDenseExtSMI) kImproves(source, target int) (bool, error) {
	return x.k.less(x.dist[source][target]), nil
}
func (x *smtDenseExtSMI) pushEdge() error { x.offsets = append(x.offsets, x.k); return nil }

func (x *smtDenseExtSMI) updateCells(g *smtDenseDiffMatrix) error {
	id := len(g.edges) - 1
	e := &g.edges[id]
	s, t := e.source, e.target
	k := x.offsets[id]
	traced := g.owner != nil && smtTraceEnabled("ddl")
	targets := x.targets[:0]
	tRow, tDist := g.rows[t], x.dist[t]
	sRow, sDist := g.rows[s], x.dist[s]
	for v := range tRow {
		if tRow[v] >= 0 && v != s {
			newDist := k.add(tDist[v])
			if traced {
				old := sDist[v]
				g.traceUpdate(s, v, newDist.String(), sRow[v], old.String)
			}
			if sRow[v] < 0 || newDist.less(sDist[v]) {
				targets = append(targets, smtDenseTargetSMI{v, newDist})
			}
		}
	}
	x.targets = targets
	for y := range g.rows {
		if y == t {
			continue
		}
		row := g.rows[y]
		if row[s] < 0 {
			continue
		}
		dist := x.dist[y]
		dys := dist[s]
		for i := range targets {
			v := targets[i].target
			if v == y {
				continue
			}
			newDist := dys.add(targets[i].distance)
			if row[v] < 0 || newDist.less(dist[v]) {
				g.trail = append(g.trail, smtDenseDiffTrail{y, v, row[v]})
				x.trail = append(x.trail, dist[v])
				row[v] = id
				dist[v] = newDist
				if err := g.propagate(y, v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (x *smtDenseExtSMI) distText(source, target, width int) string {
	return x.dist[source][target].widthText(width)
}

func (x *smtDenseExtSMI) implies(source, target, atom int, forward bool) (bool, error) {
	d := x.dist[source][target]
	if forward {
		return !x.atoms[atom].less(d), nil // d <= offset
	}
	return x.atoms[atom].less(d.neg()), nil // -d > offset
}

func (x *smtDenseExtSMI) initModel(g *smtDenseDiffMatrix) error {
	variables := len(g.rows)
	x.assignment = x.assignment[:0]
	for i := 0; i < variables; i++ {
		var d smtInfSInteger
		row, dist := g.rows[i], x.dist[i]
		for j := 0; j < variables; j++ {
			if i != j && row[j] >= 0 && dist[j].less(d) {
				d = dist[j]
			}
		}
		x.assignment = append(x.assignment, d)
	}
	for i := range x.assignment {
		x.assignment[i] = x.assignment[i].neg()
	}
	return nil
}

func (x *smtDenseExtSMI) assignmentText(v int) string { return x.assignment[v].String() }
func (x *smtDenseExtSMI) assignmentIsZero(v int) bool { return x.assignment[v].isZero() }
func (x *smtDenseExtSMI) fixZero(v int, same []bool) error {
	val := x.assignment[v]
	for v2 := range x.assignment {
		if same[v2] {
			x.assignment[v2] = x.assignment[v2].sub(val)
		}
	}
	return nil
}
func (x *smtDenseExtSMI) assignmentParts(v int, n, k *numeral.Rational) error {
	a := x.assignment[v]
	return smtDenseSIntegerParts(x.q, n, k, a.first, a.second)
}
func (x *smtDenseExtSMI) edgeParts(edge int, n, k *numeral.Rational) error {
	a := x.offsets[edge]
	return smtDenseSIntegerParts(x.q, n, k, a.first, a.second)
}
func (x *smtDenseExtSMI) setAssignment(v int, r *numeral.Rational) error {
	x.assignment[v] = smtInfSInteger{first: smtSIntegerOfRational(x.q, r)}
	return nil
}
func (x *smtDenseExtSMI) close() error { *x = smtDenseExtSMI{q: x.q}; return nil }

// smtDenseSIntegerParts is get_rational().to_rational() and
// get_infinitesimal().to_rational() of a machine numeral.
func smtDenseSIntegerParts(q *numeral.RationalManager, n, k *numeral.Rational, first, second int32) error {
	if err := q.SetInt32(n, first); err != nil {
		return err
	}
	return q.SetInt32(k, second)
}
