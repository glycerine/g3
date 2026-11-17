package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h mk_var, add_edge,
// update_cells, get_antecedents, restore_cells and init_model.
//
// theory_dense_diff_logic<Ext> is instantiated four times (theory_dense.h):
// si_ext (s_integer), smi_ext (inf_s_integer), i_ext (rational) and mi_ext
// (inf_rational). The matrix keeps the numeral-independent parts (cell edge
// ids, edge endpoints and justifications, the cell trail, scopes and atom
// occurrences); an smtDenseExt keeps every numeral of one instantiation in
// its own concrete type: the cell distances, edge and atom offsets, the
// trailed distances, the model assignment and the k register of the
// operation in progress.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtDenseExtKind names the four instantiations.
type smtDenseExtKind uint8

const (
	smtDenseMI  smtDenseExtKind = iota // theory_dense_mi: inf_rational
	smtDenseI                          // theory_dense_i: rational
	smtDenseSMI                        // theory_dense_smi: inf_s_integer
	smtDenseSI                         // theory_dense_si: s_integer
)

// smtDenseExt is the numeral half of one instantiation. k is the numeral
// local of internalize_term_core, internalize_atom and assign_eh, passed to
// add_edge as its offset.
type smtDenseExt interface {
	// addVariable is mk_var's matrix growth: a cell() for v in every row, a
	// new row of v+1 cells, and m_matrix[v][v].m_distance.reset().
	addVariable(v int)
	// shrink is restore_cells(trail) (newest first, reading g.trail's
	// cells), m_edges.shrink(edges) and del_vars(variables).
	shrink(g *smtDenseDiffMatrix, variables, edges, trail int) error
	setK(r *numeral.Rational) error // numeral k(_k)
	setKAtom(atom int) error        // k = a->get_offset()
	negK() error                    // k.neg()
	subKEpsilon(real bool) error    // k -= get_epsilon(s)
	kText() string
	pushAtom() error // the new atom's m_offset = k
	popAtoms(atoms int) error
	atomText(atom int, width int) string
	// kConflicts is add_edge's -c_inv.m_distance > offset, c_inv = m[target][source].
	kConflicts(target, source int) (bool, error)
	// kImproves is add_edge's offset < c.m_distance, c = m[source][target].
	kImproves(source, target int) (bool, error)
	pushEdge() error // m_edges.push_back(edge(source, target, k, l))
	updateCells(g *smtDenseDiffMatrix) error
	distText(source, target int, width int) string
	// implies is propagate_using_cell's test for atom over cell
	// (source, target): c.m_distance <= a->get_offset() when the atom has the
	// cell's direction, -c.m_distance > a->get_offset() otherwise.
	implies(source, target, atom int, forward bool) (bool, error)
	// initModel is init_model(): m_assignment[i] = -min(0, distances of row i).
	initModel(g *smtDenseDiffMatrix) error
	assignmentText(v int) string
	assignmentIsZero(v int) bool
	// fixZero is fix_zero's numeral val = m_assignment[v] and
	// m_assignment[v2] -= val for every v2 with same[v2].
	fixZero(v int, same []bool) error
	// parts are get_rational().to_rational() and
	// get_infinitesimal().to_rational().
	assignmentParts(v int, n, k *numeral.Rational) error
	edgeParts(edge int, n, k *numeral.Rational) error
	// setAssignment is maximize's numeral a(q); m_assignment[i] = a.
	setAssignment(v int, r *numeral.Rational) error
	close() error
}

func newSMTDenseExt(kind smtDenseExtKind, q *numeral.RationalManager) smtDenseExt {
	switch kind {
	case smtDenseI:
		return &smtDenseExtI{q: q, k: smtQZero()}
	case smtDenseSMI:
		return &smtDenseExtSMI{q: q}
	case smtDenseSI:
		return &smtDenseExtSI{q: q}
	}
	return &smtDenseExtMI{q: q, k: newSMTInfRational()}
}

// smtDenseDiffEdge is edge without its offset (kept by the Ext).
type smtDenseDiffEdge struct {
	source, target int
	literal        SMTLiteral
}

// smtDenseDiffTrail is cell_trail without m_old_distance (kept by the Ext).
type smtDenseDiffTrail struct {
	source, target int
	edge           int
}

type smtDenseDiffScope struct{ variables, edges, trail int }

type smtDenseDiffMatrix struct {
	owner *SMTDenseDiffTheory
	ext   smtDenseExt
	// rows[source][target] is cell::m_edge_id: zero is the self edge,
	// negative one means disconnected.
	rows   [][]int
	edges  []smtDenseDiffEdge
	trail  []smtDenseDiffTrail
	scopes []smtDenseDiffScope
	// occs[source][target] is cell::m_occs, the atoms over (source, target)
	// or (target, source); kept beside rows so the closure loops stay compact.
	occs [][][]int
}

func newSMTDenseDiffMatrix(ext smtDenseExt) *smtDenseDiffMatrix {
	return &smtDenseDiffMatrix{ext: ext, edges: []smtDenseDiffEdge{{source: -1, target: -1, literal: SMTNullLiteral}}}
}

func (g *smtDenseDiffMatrix) addVariable() int {
	v := len(g.rows)
	for i := range g.rows {
		g.rows[i] = append(g.rows[i], -1)
	}
	row := make([]int, v+1)
	for i := range row {
		row[i] = -1
	}
	row[v] = 0
	g.rows = append(g.rows, row)
	for i := range g.occs {
		g.occs[i] = append(g.occs[i], nil)
	}
	g.occs = append(g.occs, make([][]int, v+1))
	g.ext.addVariable(v)
	return v
}

func (g *smtDenseDiffMatrix) valid(v int) bool {
	return g != nil && v >= 0 && v < len(g.rows)
}

func (g *smtDenseDiffMatrix) connected(source, target int) bool {
	return g.valid(source) && g.valid(target) && g.rows[source][target] >= 0
}

// traceOwner emits a ddl TRACE record of the owning dense theory.
func (g *smtDenseDiffMatrix) traceOwner(function string, line int, body func(id func(int) uint32) string) {
	if g.owner == nil {
		return
	}
	g.owner.traceDDL("ddl", function, line, func() string {
		return body(func(v int) uint32 {
			id, _ := g.owner.variables[v].OwnerID()
			return id
		})
	})
}

func (g *smtDenseDiffMatrix) antecedents(source, target int) ([]SMTLiteral, error) {
	g.traceOwner("get_antecedents", 442, func(id func(int) uint32) string {
		return fmt.Sprintf("get_antecedents, source: #%d, target: #%d\n", id(source), id(target))
	})
	if !g.connected(source, target) {
		return nil, errSMTDiffGraphDomain
	}
	type pair struct{ source, target int }
	var todo []pair
	if source != target {
		todo = append(todo, pair{source, target})
	}
	var result []SMTLiteral
	for len(todo) > 0 {
		i := len(todo) - 1
		p := todo[i]
		todo = todo[:i]
		id := g.rows[p.source][p.target]
		if id <= 0 || id >= len(g.edges) {
			return nil, errSMTDiffGraphDomain
		}
		e := &g.edges[id]
		if e.literal != SMTNullLiteral {
			result = append(result, e.literal)
		}
		if p.source != e.source {
			todo = append(todo, pair{p.source, e.source})
		}
		if e.target != p.target {
			todo = append(todo, pair{e.target, p.target})
		}
	}
	return result, nil
}

// addEdge is add_edge(source, target, k, literal) with the Ext's k. A
// rejected edge reports the source's reverse-path antecedents followed by
// the new literal, without changing the matrix or edge list.
func (g *smtDenseDiffMatrix) addEdge(source, target int, literal SMTLiteral) (bool, []SMTLiteral, error) {
	if !g.valid(source) || !g.valid(target) {
		return false, nil, errSMTDiffGraphDomain
	}
	g.traceOwner("add_edge", 589, func(id func(int) uint32) string {
		return fmt.Sprintf("trying adding edge: #%d -- %s --> #%d\n", id(source), g.ext.kText(), id(target))
	})
	inverse := g.rows[target][source]
	if inverse >= 0 {
		conflict, err := g.ext.kConflicts(target, source)
		if err != nil {
			return false, nil, err
		}
		if conflict {
			g.traceOwner("add_edge", 593, func(id func(int) uint32) string {
				return fmt.Sprintf("conflict detected: #%d #%d offset: %s, c_inv.m_edge_id: %d, c_inv.m_distance: %s\n",
					id(source), id(target), g.ext.kText(), inverse, g.ext.distText(target, source, 0))
			})
			antecedents, err := g.antecedents(target, source)
			if err != nil {
				return false, nil, err
			}
			if literal != SMTNullLiteral {
				antecedents = append(antecedents, literal)
			}
			return false, antecedents, nil
		}
	}
	improves := g.rows[source][target] < 0
	if !improves {
		var err error
		if improves, err = g.ext.kImproves(source, target); err != nil {
			return false, nil, err
		}
	}
	if improves {
		g.traceOwner("add_edge", 613, func(id func(int) uint32) string {
			return fmt.Sprintf("adding edge: #%d -- %s --> #%d\n", id(source), g.ext.kText(), id(target))
		})
		g.edges = append(g.edges, smtDenseDiffEdge{source: source, target: target, literal: literal})
		if err := g.ext.pushEdge(); err != nil {
			return false, nil, err
		}
		if err := g.ext.updateCells(g); err != nil {
			return false, nil, err
		}
	}
	return true, nil, nil
}

// traceUpdate is update_cells' TRACE("ddl") for candidate x.
func (g *smtDenseDiffMatrix) traceUpdate(s, x int, newDist string, oldEdge int, oldDist func() string) {
	g.traceOwner("update_cells", 495, func(id func(int) uint32) string {
		return fmt.Sprintf("s: #%d x: #%d new_dist: %s\nalready has edge: %d  old dist: %s\n",
			id(s), id(x), newDist, oldEdge, oldDist())
	})
}

// propagate runs propagate_using_cell(y, x) when the cell has atoms.
func (g *smtDenseDiffMatrix) propagate(y, x int) error {
	if g.owner == nil || len(g.occs[y][x]) == 0 {
		return nil
	}
	return g.owner.propagateCell(y, x)
}

func (g *smtDenseDiffMatrix) push() {
	g.scopes = append(g.scopes, smtDenseDiffScope{len(g.rows), len(g.edges), len(g.trail)})
}

func (g *smtDenseDiffMatrix) pop(count uint32) error {
	if g == nil || uint64(count) > uint64(len(g.scopes)) {
		return errSMTDiffGraphDomain
	}
	if count == 0 {
		return nil
	}
	level := len(g.scopes) - int(count)
	scope := g.scopes[level]
	// restore_cells: the Ext restores the distances first, newest first.
	if err := g.ext.shrink(g, scope.variables, scope.edges, scope.trail); err != nil {
		return err
	}
	for i := len(g.trail) - 1; i >= scope.trail; i-- {
		t := &g.trail[i]
		g.rows[t.source][t.target] = t.edge
	}
	clear(g.trail[scope.trail:])
	g.trail = g.trail[:scope.trail]
	clear(g.edges[scope.edges:])
	g.edges = g.edges[:scope.edges]
	for i := range g.rows {
		g.rows[i] = g.rows[i][:scope.variables]
		clear(g.occs[i][scope.variables:])
		g.occs[i] = g.occs[i][:scope.variables]
	}
	clear(g.rows[scope.variables:])
	g.rows = g.rows[:scope.variables]
	clear(g.occs[scope.variables:])
	g.occs = g.occs[:scope.variables]
	g.scopes = g.scopes[:level]
	return nil
}
