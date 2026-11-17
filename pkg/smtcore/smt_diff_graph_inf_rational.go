package smtcore

// Source: src/smt/diff_logic.h concrete inf_int_rational graph fragment used
// by real difference logic (rdl_ext, srdl_ext) and UTVPI (rdl_ext).

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtInfIntRational is inf_int_rational: rational + infinitesimal*epsilon,
// ordered lexicographically with a positive infinitesimal epsilon. The
// rational is a numeral.Rational on the graph's mpq manager (owned: free
// it with del); the infinitesimal is inf_int_rational::m_second, a C++
// int. z3's += and -= on m_second overflow (undefined behaviour) only after
// 2^31 accumulated epsilon steps, which no difference-logic input reaches
// (each strict bound contributes one step); Go's int does not wrap there.
type smtInfIntRational struct {
	rational      numeral.Rational
	infinitesimal int
}

func newSMTInfIntRational() smtInfIntRational { return smtInfIntRational{rational: smtQZero()} }

func (v *smtInfIntRational) del(q *numeral.RationalManager) error { return smtQDel(q, &v.rational) }

func (v *smtInfIntRational) isZero() bool { return v.rational.IsZero() && v.infinitesimal == 0 }

// sign is the sign of the lexicographically leading nonzero component.
func (v *smtInfIntRational) sign() int {
	if s := v.rational.Sign(); s != 0 {
		return s
	}
	switch {
	case v.infinitesimal < 0:
		return -1
	case v.infinitesimal > 0:
		return 1
	}
	return 0
}

func smtInfIntSet(q *numeral.RationalManager, dst, src *smtInfIntRational) error {
	dst.infinitesimal = src.infinitesimal
	return q.Copy(&dst.rational, &src.rational)
}

func smtInfIntAdd(q *numeral.RationalManager, a, b, out *smtInfIntRational) error {
	out.infinitesimal = a.infinitesimal + b.infinitesimal
	return q.Add(&a.rational, &b.rational, &out.rational)
}

func smtInfIntSub(q *numeral.RationalManager, a, b, out *smtInfIntRational) error {
	out.infinitesimal = a.infinitesimal - b.infinitesimal
	return q.Sub(&a.rational, &b.rational, &out.rational)
}

func smtInfIntNeg(q *numeral.RationalManager, v *smtInfIntRational) error {
	v.infinitesimal = -v.infinitesimal
	return q.Neg(&v.rational)
}

func smtInfIntCmp(q *numeral.RationalManager, a, b *smtInfIntRational) (int, error) {
	c, err := q.Compare(&a.rational, &b.rational)
	if err != nil || c != 0 {
		return c, err
	}
	switch {
	case a.infinitesimal < b.infinitesimal:
		return -1, nil
	case a.infinitesimal > b.infinitesimal:
		return 1, nil
	}
	return 0, nil
}

// smtInfIntText is inf_int_rational::to_string.
func smtInfIntText(q *numeral.RationalManager, v *smtInfIntRational) (string, error) {
	main, err := q.String(&v.rational)
	if err != nil || v.infinitesimal == 0 {
		return main, err
	}
	if v.infinitesimal < 0 {
		return "(" + main + " -e*" + strconv.Itoa(-v.infinitesimal) + ")", nil
	}
	return "(" + main + " +e*" + strconv.Itoa(v.infinitesimal) + ")", nil
}

type smtInfRationalDiffEdge struct {
	source, target int32
	weight         smtInfIntRational
	explanation    SMTLiteral
	timestamp      uint32
	enabled        bool
}

type smtInfRationalDiffAssignmentTrail struct {
	variable int32
	oldValue smtInfIntRational
}

type smtInfRationalDiffScope struct {
	edges, enabled int
	timestamp      uint32
}

// SMTInfRationalDiffGraph is the retained inf_int_rational dl_graph
// instantiation. It contains only operations used by real difference logic
// and UTVPI. Values are graph-owned and freed where dl_graph's vectors drop
// them; a manager failure in an operation that returns no error is kept in
// qErr and returned by the next entry point that does.
type SMTInfRationalDiffGraph struct {
	q               *numeral.RationalManager
	qErr            error
	assignment      []smtInfIntRational
	assignmentTrail []smtInfRationalDiffAssignmentTrail
	edges           []smtInfRationalDiffEdge
	outEdges        [][]int32
	scopes          []smtInfRationalDiffScope
	gamma           []smtInfIntRational
	mark            []smtDiffSearchMark
	parent          []int32
	visited         []int32
	heap            smtInfRationalDiffHeap
	lastEnabledEdge int32
	enabledEdges    []int32
	propagationCost uint32
	timestamp       uint32
}

func NewSMTInfRationalDiffGraph(q *numeral.RationalManager) *SMTInfRationalDiffGraph {
	g := &SMTInfRationalDiffGraph{q: q, lastEnabledEdge: -1}
	g.heap.graph = g
	return g
}

func (g *SMTInfRationalDiffGraph) keep(err error) {
	if err != nil && g.qErr == nil {
		g.qErr = err
	}
}

func (g *SMTInfRationalDiffGraph) takeErr(err error) error {
	if g.qErr != nil {
		err = errors.Join(err, g.qErr)
		g.qErr = nil
	}
	return err
}

func (g *SMTInfRationalDiffGraph) cmp(a, b *smtInfIntRational) int {
	c, err := smtInfIntCmp(g.q, a, b)
	g.keep(err)
	return c
}
func (g *SMTInfRationalDiffGraph) set(dst, src *smtInfIntRational) {
	g.keep(smtInfIntSet(g.q, dst, src))
}
func (g *SMTInfRationalDiffGraph) add(a, b, out *smtInfIntRational) {
	g.keep(smtInfIntAdd(g.q, a, b, out))
}
func (g *SMTInfRationalDiffGraph) sub(a, b, out *smtInfIntRational) {
	g.keep(smtInfIntSub(g.q, a, b, out))
}
func (g *SMTInfRationalDiffGraph) del(v *smtInfIntRational) { g.keep(v.del(g.q)) }

// reset makes v zero.
func (g *SMTInfRationalDiffGraph) reset(v *smtInfIntRational) {
	v.infinitesimal = 0
	g.keep(g.q.Reset(&v.rational))
}

// release frees every value the graph owns (dl_graph's destructor).
func (g *SMTInfRationalDiffGraph) release() {
	for i := range g.assignment {
		g.del(&g.assignment[i])
		g.del(&g.gamma[i])
	}
	for i := range g.assignmentTrail {
		g.del(&g.assignmentTrail[i].oldValue)
	}
	for i := range g.edges {
		g.del(&g.edges[i].weight)
	}
}

func (g *SMTInfRationalDiffGraph) validVariable(v int32) bool {
	return g != nil && v >= 0 && int(v) < len(g.assignment)
}

func (g *SMTInfRationalDiffGraph) validEdge(id int32) bool {
	return g != nil && id >= 0 && int(id) < len(g.edges)
}

func (g *SMTInfRationalDiffGraph) InitVariable(v int32) error {
	if g == nil || v < 0 {
		return errSMTDiffGraphDomain
	}
	for int(v) >= len(g.assignment) {
		g.assignment = append(g.assignment, newSMTInfIntRational())
		g.outEdges = append(g.outEdges, nil)
		g.gamma = append(g.gamma, newSMTInfIntRational())
		g.mark = append(g.mark, smtDiffUnmarked)
		g.parent = append(g.parent, -1)
	}
	if len(g.outEdges[v]) != 0 || g.mark[v] != smtDiffUnmarked {
		return errSMTDiffGraphDomain
	}
	g.reset(&g.assignment[v])
	return g.takeErr(nil)
}

func (g *SMTInfRationalDiffGraph) AddEdge(source, target int32, weight *smtInfIntRational, explanation SMTLiteral) (int32, error) {
	if !g.validVariable(source) || !g.validVariable(target) || weight == nil {
		return -1, errSMTDiffGraphDomain
	}
	id := int32(len(g.edges))
	edge := smtInfRationalDiffEdge{source: source, target: target, weight: newSMTInfIntRational(), explanation: explanation, timestamp: g.timestamp}
	if err := smtInfIntSet(g.q, &edge.weight, weight); err != nil {
		return -1, err
	}
	g.edges = append(g.edges, edge)
	g.outEdges[source] = append(g.outEdges[source], id)
	return id, nil
}

func (g *SMTInfRationalDiffGraph) edgeFeasible(edge *smtInfRationalDiffEdge) bool {
	if !edge.enabled {
		return true
	}
	difference := newSMTInfIntRational()
	g.sub(&g.assignment[edge.target], &g.assignment[edge.source], &difference)
	feasible := g.cmp(&difference, &edge.weight) <= 0
	g.del(&difference)
	return feasible
}

func (g *SMTInfRationalDiffGraph) IsFeasible() bool {
	if g == nil {
		return false
	}
	for index := range g.edges {
		if !g.edgeFeasible(&g.edges[index]) {
			return false
		}
	}
	return true
}

func (g *SMTInfRationalDiffGraph) setGamma(edge *smtInfRationalDiffEdge, result *smtInfIntRational) {
	g.sub(&g.assignment[edge.source], &g.assignment[edge.target], result)
	g.add(result, &edge.weight, result)
}

func (g *SMTInfRationalDiffGraph) accumulateAssignment(variable int32, increment *smtInfIntRational) {
	trail := smtInfRationalDiffAssignmentTrail{variable: variable, oldValue: newSMTInfIntRational()}
	g.set(&trail.oldValue, &g.assignment[variable])
	g.assignmentTrail = append(g.assignmentTrail, trail)
	g.add(&g.assignment[variable], increment, &g.assignment[variable])
}

// clearAssignmentTrail is m_assignment_stack.reset().
func (g *SMTInfRationalDiffGraph) clearAssignmentTrail() {
	for index := range g.assignmentTrail {
		g.del(&g.assignmentTrail[index].oldValue)
	}
	g.assignmentTrail = g.assignmentTrail[:0]
}

func (g *SMTInfRationalDiffGraph) undoAssignments() {
	for index := len(g.assignmentTrail); index > 0; index-- {
		trail := &g.assignmentTrail[index-1]
		g.set(&g.assignment[trail.variable], &trail.oldValue)
	}
	g.clearAssignmentTrail()
}

func (g *SMTInfRationalDiffGraph) resetMarks() {
	for _, variable := range g.visited {
		g.mark[variable] = smtDiffUnmarked
	}
	g.visited = g.visited[:0]
}

func (g *SMTInfRationalDiffGraph) makeFeasible(id int32) bool {
	edge := &g.edges[id]
	root, target := edge.source, edge.target
	g.reset(&g.gamma[root])
	g.setGamma(edge, &g.gamma[target])
	g.mark[target] = smtDiffProcessed
	g.parent[target] = id
	g.visited = append(g.visited, target)
	g.accumulateAssignment(target, &g.gamma[target])
	if smtTraceEnabled("arith") {
		emitSMTTrace("arith", "make_feasible", "src/smt/diff_logic.h", 378, strconv.Itoa(int(id))+"\n")
	}

	gamma := newSMTInfIntRational()
	defer g.del(&gamma)
	source := target
	for {
		g.propagationCost++
		if g.mark[root] != smtDiffUnmarked {
			g.heap.reset()
			g.resetMarks()
			g.undoAssignments()
			return false
		}
		for _, edgeID := range g.outEdges[source] {
			candidate := &g.edges[edgeID]
			if !candidate.enabled {
				continue
			}
			g.setGamma(candidate, &gamma)
			if gamma.sign() >= 0 {
				continue
			}
			next := candidate.target
			switch g.mark[next] {
			case smtDiffUnmarked:
				g.set(&g.gamma[next], &gamma)
				g.mark[next] = smtDiffFound
				g.parent[next] = edgeID
				g.visited = append(g.visited, next)
				g.heap.insert(next)
			case smtDiffFound:
				if g.cmp(&gamma, &g.gamma[next]) < 0 {
					g.set(&g.gamma[next], &gamma)
					g.parent[next] = edgeID
					g.heap.decreased(next)
				}
			}
		}
		if g.heap.empty() {
			g.resetMarks()
			g.clearAssignmentTrail()
			return true
		}
		source = g.heap.eraseMin()
		g.mark[source] = smtDiffProcessed
		g.accumulateAssignment(source, &g.gamma[source])
	}
}

func (g *SMTInfRationalDiffGraph) EnableEdge(id int32) (bool, error) {
	if !g.validEdge(id) {
		return false, errSMTDiffGraphDomain
	}
	edge := &g.edges[id]
	if edge.enabled {
		return true, nil
	}
	edge.enabled = true
	edge.timestamp = g.timestamp
	g.timestamp++
	g.lastEnabledEdge = id
	feasible := true
	if !g.edgeFeasible(edge) {
		feasible = g.makeFeasible(id)
	}
	g.enabledEdges = append(g.enabledEdges, id)
	return feasible, g.takeErr(nil)
}

func (g *SMTInfRationalDiffGraph) NegativeCycleEdges() ([]int32, error) {
	if !g.validEdge(g.lastEnabledEdge) || g.edgeFeasible(&g.edges[g.lastEnabledEdge]) {
		return nil, errSMTDiffGraphDomain
	}
	last := g.lastEnabledEdge
	result := make([]int32, 0, 4)
	id := last
	for steps := 0; ; steps++ {
		if steps > len(g.edges) || !g.validEdge(id) {
			return nil, errSMTDiffGraphDomain
		}
		result = append(result, id)
		id = g.parent[g.edges[id].source]
		if id == last {
			break
		}
	}
	return result, nil
}

func (g *SMTInfRationalDiffGraph) NegativeCycleExplanations() ([]SMTLiteral, error) {
	edges, err := g.negativeCycleEdgesWithShortcuts()
	if err != nil {
		return nil, err
	}
	result := make([]SMTLiteral, len(edges))
	for i, id := range edges {
		result[i] = g.edges[id].explanation
	}
	return result, nil
}

func (g *SMTInfRationalDiffGraph) Push() error {
	if g == nil {
		return errSMTDiffGraphDomain
	}
	g.scopes = append(g.scopes, smtInfRationalDiffScope{len(g.edges), len(g.enabledEdges), g.timestamp})
	return nil
}

func (g *SMTInfRationalDiffGraph) Pop(count uint32) error {
	if g == nil || count == 0 || int(count) > len(g.scopes) {
		return errSMTDiffGraphDomain
	}
	level := len(g.scopes) - int(count)
	scope := g.scopes[level]
	for index := len(g.enabledEdges); index > scope.enabled; index-- {
		g.edges[g.enabledEdges[index-1]].enabled = false
	}
	g.enabledEdges = g.enabledEdges[:scope.enabled]
	g.timestamp = scope.timestamp
	for len(g.edges) > scope.edges {
		id := int32(len(g.edges) - 1)
		edge := &g.edges[id]
		g.outEdges[edge.source] = g.outEdges[edge.source][:len(g.outEdges[edge.source])-1]
		g.del(&edge.weight)
		g.edges = g.edges[:id]
	}
	g.scopes = g.scopes[:level]
	return g.takeErr(nil)
}

// Assignment copies the value of variable into out.
func (g *SMTInfRationalDiffGraph) Assignment(variable int32, out *smtInfIntRational) error {
	if !g.validVariable(variable) || out == nil {
		return errSMTDiffGraphDomain
	}
	return smtInfIntSet(g.q, out, &g.assignment[variable])
}

func (g *SMTInfRationalDiffGraph) SetToZero(variable int32) error {
	if !g.validVariable(variable) || !g.IsFeasible() {
		return errSMTDiffGraphDomain
	}
	if g.assignment[variable].isZero() {
		return nil
	}
	offset := newSMTInfIntRational()
	g.set(&offset, &g.assignment[variable])
	for index := range g.assignment {
		g.sub(&g.assignment[index], &offset, &g.assignment[index])
	}
	g.del(&offset)
	return g.takeErr(nil)
}

func (g *SMTInfRationalDiffGraph) NumNodes() int {
	if g == nil {
		return 0
	}
	return len(g.assignment)
}

func (g *SMTInfRationalDiffGraph) NumEdges() int {
	if g == nil {
		return 0
	}
	return len(g.edges)
}

type smtInfRationalDiffHeap struct {
	graph     *SMTInfRationalDiffGraph
	variables []int32
	positions []int
}

func (h *smtInfRationalDiffHeap) empty() bool { return len(h.variables) == 0 }
func (h *smtInfRationalDiffHeap) ensure(v int32) {
	for int(v) >= len(h.positions) {
		h.positions = append(h.positions, -1)
	}
}
func (h *smtInfRationalDiffHeap) less(i, j int) bool {
	return h.graph.cmp(&h.graph.gamma[h.variables[i]], &h.graph.gamma[h.variables[j]]) < 0
}
func (h *smtInfRationalDiffHeap) swap(i, j int) {
	h.variables[i], h.variables[j] = h.variables[j], h.variables[i]
	h.positions[h.variables[i]], h.positions[h.variables[j]] = i, j
}
func (h *smtInfRationalDiffHeap) up(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if !h.less(index, parent) {
			break
		}
		h.swap(index, parent)
		index = parent
	}
}
func (h *smtInfRationalDiffHeap) down(index int) {
	for {
		child := index*2 + 1
		if child >= len(h.variables) {
			return
		}
		if child+1 < len(h.variables) && h.less(child+1, child) {
			child++
		}
		if !h.less(child, index) {
			return
		}
		h.swap(index, child)
		index = child
	}
}
func (h *smtInfRationalDiffHeap) insert(v int32) {
	h.ensure(v)
	if h.positions[v] >= 0 {
		return
	}
	h.positions[v] = len(h.variables)
	h.variables = append(h.variables, v)
	h.up(len(h.variables) - 1)
}
func (h *smtInfRationalDiffHeap) decreased(v int32) {
	h.ensure(v)
	if h.positions[v] >= 0 {
		h.up(h.positions[v])
	}
}
func (h *smtInfRationalDiffHeap) eraseMin() int32 {
	result := h.variables[0]
	last := len(h.variables) - 1
	h.swap(0, last)
	h.variables = h.variables[:last]
	h.positions[result] = -1
	if len(h.variables) != 0 {
		h.down(0)
	}
	return result
}
func (h *smtInfRationalDiffHeap) reset() {
	for _, variable := range h.variables {
		h.positions[variable] = -1
	}
	h.variables = h.variables[:0]
}
