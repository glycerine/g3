package smtcore

// Source: src/smt/diff_logic.h sparse graph construction, incremental edge
// activation, feasible assignments, negative-cycle explanations and scopes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

var errSMTDiffGraphDomain = errors.New("smt: invalid difference-logic graph operation")

type smtDiffSearchMark uint8

const (
	smtDiffUnmarked smtDiffSearchMark = iota
	smtDiffFound
	smtDiffProcessed
)

type smtRationalDiffEdge struct {
	source, target int32
	weight         numeral.Rational
	timestamp      uint32
	explanation    SMTLiteral
	enabled        bool
}

type smtRationalDiffAssignmentTrail struct {
	variable int32
	oldValue numeral.Rational
}

type smtRationalDiffScope struct {
	edges, enabled int
	timestamp      uint32
}

// SMTRationalDiffGraph is the concrete rational dl_graph instantiation used
// by integer difference logic (idl_ext::numeral is rational). Rational
// values are graph-owned numeral.Rationals on q, the AST manager's mpq
// manager (rational's g_mpq_manager), freed where dl_graph's vectors drop
// them; explanations are borrowed SMT literals. A manager failure inside an
// operation that returns no error is kept in qErr and returned by the next
// entry point that does.
type SMTRationalDiffGraph struct {
	q                 *numeral.RationalManager
	qErr              error
	assignment        []numeral.Rational
	assignmentTrail   []smtRationalDiffAssignmentTrail
	edges             []smtRationalDiffEdge
	outEdges, inEdges [][]int32
	scopes            []smtRationalDiffScope
	gamma             []numeral.Rational
	mark              []smtDiffSearchMark
	parent            []int32
	visited           []int32
	heap              smtRationalDiffHeap
	timestamp         uint32
	lastEnabledEdge   int32
	enabledEdges      []int32
	activity          []uint32
	propagationCost   uint32
	numRelax          uint32
}

func NewSMTRationalDiffGraph(q *numeral.RationalManager) *SMTRationalDiffGraph {
	g := &SMTRationalDiffGraph{q: q, lastEnabledEdge: -1}
	g.heap.graph = g
	return g
}

// keep records the first manager failure; takeErr returns and clears it.
func (g *SMTRationalDiffGraph) keep(err error) {
	if err != nil && g.qErr == nil {
		g.qErr = err
	}
}

func (g *SMTRationalDiffGraph) takeErr(err error) error {
	if g.qErr != nil {
		err = errors.Join(err, g.qErr)
		g.qErr = nil
	}
	return err
}

// cmp is a three-way rational comparison; sign is is_neg/is_pos.
func (g *SMTRationalDiffGraph) cmp(a, b *numeral.Rational) int {
	c, err := g.q.Compare(a, b)
	g.keep(err)
	return c
}

func (g *SMTRationalDiffGraph) set(dst, src *numeral.Rational) { g.keep(g.q.Copy(dst, src)) }
func (g *SMTRationalDiffGraph) add(a, b, out *numeral.Rational) {
	g.keep(g.q.Add(a, b, out))
}
func (g *SMTRationalDiffGraph) sub(a, b, out *numeral.Rational) {
	g.keep(g.q.Sub(a, b, out))
}
func (g *SMTRationalDiffGraph) del(a *numeral.Rational) { g.keep(smtQDel(g.q, a)) }

// release frees every value the graph owns (dl_graph's destructor).
func (g *SMTRationalDiffGraph) release() {
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

func (g *SMTRationalDiffGraph) validVariable(v int32) bool {
	return g != nil && v >= 0 && int(v) < len(g.assignment)
}

func (g *SMTRationalDiffGraph) validEdge(id int32) bool {
	return g != nil && id >= 0 && int(id) < len(g.edges)
}

// InitVariable creates all missing source IDs through v and resets v. As in
// the source, an already initialized variable may be reset only while it has
// no incident edges.
func (g *SMTRationalDiffGraph) InitVariable(v int32) error {
	if g == nil || v < 0 {
		return errSMTDiffGraphDomain
	}
	for int(v) >= len(g.assignment) {
		g.assignment = append(g.assignment, smtQZero())
		g.outEdges = append(g.outEdges, nil)
		g.inEdges = append(g.inEdges, nil)
		g.gamma = append(g.gamma, smtQZero())
		g.mark = append(g.mark, smtDiffUnmarked)
		g.parent = append(g.parent, -1)
	}
	if len(g.outEdges[v]) != 0 || len(g.inEdges[v]) != 0 || g.mark[v] != smtDiffUnmarked {
		return errSMTDiffGraphDomain
	}
	g.keep(g.q.Reset(&g.assignment[v]))
	return g.takeErr(nil)
}

func (g *SMTRationalDiffGraph) AddEdge(source, target int32, weight *numeral.Rational, explanation SMTLiteral) (int32, error) {
	if !g.validVariable(source) || !g.validVariable(target) || weight == nil {
		return -1, errSMTDiffGraphDomain
	}
	id := int32(len(g.edges))
	edge := smtRationalDiffEdge{source: source, target: target, weight: smtQZero(), timestamp: g.timestamp, explanation: explanation}
	if err := g.q.Copy(&edge.weight, weight); err != nil {
		return -1, err
	}
	g.edges = append(g.edges, edge)
	g.activity = append(g.activity, 0)
	g.outEdges[source] = append(g.outEdges[source], id)
	g.inEdges[target] = append(g.inEdges[target], id)
	return id, nil
}

func (g *SMTRationalDiffGraph) edgeFeasible(edge *smtRationalDiffEdge) bool {
	if !edge.enabled {
		return true
	}
	difference := smtQZero()
	g.sub(&g.assignment[edge.target], &g.assignment[edge.source], &difference)
	feasible := g.cmp(&difference, &edge.weight) <= 0
	g.del(&difference)
	return feasible
}

func (g *SMTRationalDiffGraph) IsFeasible() bool {
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

func (g *SMTRationalDiffGraph) setGamma(edge *smtRationalDiffEdge, result *numeral.Rational) {
	g.sub(&g.assignment[edge.source], &g.assignment[edge.target], result)
	g.add(result, &edge.weight, result)
}

func (g *SMTRationalDiffGraph) accumulateAssignment(variable int32, increment *numeral.Rational) {
	trail := smtRationalDiffAssignmentTrail{variable: variable, oldValue: smtQZero()}
	g.set(&trail.oldValue, &g.assignment[variable])
	g.assignmentTrail = append(g.assignmentTrail, trail)
	g.add(&g.assignment[variable], increment, &g.assignment[variable])
}

// clearAssignmentTrail is m_assignment_stack.reset().
func (g *SMTRationalDiffGraph) clearAssignmentTrail() {
	for index := range g.assignmentTrail {
		g.del(&g.assignmentTrail[index].oldValue)
	}
	g.assignmentTrail = g.assignmentTrail[:0]
}

func (g *SMTRationalDiffGraph) undoAssignments() {
	for index := len(g.assignmentTrail); index > 0; index-- {
		trail := &g.assignmentTrail[index-1]
		g.set(&g.assignment[trail.variable], &trail.oldValue)
	}
	g.clearAssignmentTrail()
}

func (g *SMTRationalDiffGraph) resetMarks() {
	for _, variable := range g.visited {
		g.mark[variable] = smtDiffUnmarked
	}
	g.visited = g.visited[:0]
}

func (g *SMTRationalDiffGraph) makeFeasible(id int32) bool {
	edge := &g.edges[id]
	root, target := edge.source, edge.target
	g.keep(g.q.Reset(&g.gamma[root]))
	g.setGamma(edge, &g.gamma[target])
	g.mark[target] = smtDiffProcessed
	g.parent[target] = id
	g.visited = append(g.visited, target)
	g.accumulateAssignment(target, &g.gamma[target])

	gamma := smtQZero()
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
			if !gamma.IsNeg() {
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

// EnableEdge returns false precisely when this activation closes a negative
// cycle. The edge remains enabled so NegativeCycleExplanations can inspect it.
func (g *SMTRationalDiffGraph) EnableEdge(id int32) (bool, error) {
	if !g.validEdge(id) {
		return false, errSMTDiffGraphDomain
	}
	edge := &g.edges[id]
	if edge.enabled {
		return true, nil
	}
	edge.enabled = true
	edge.timestamp = g.timestamp
	g.lastEnabledEdge = id
	g.timestamp++
	feasible := true
	if !g.edgeFeasible(edge) {
		feasible = g.makeFeasible(id)
	}
	g.enabledEdges = append(g.enabledEdges, id)
	return feasible, g.takeErr(nil)
}

func (g *SMTRationalDiffGraph) findRelaxedEdge(id int32, gamma *numeral.Rational) int32 {
	edge := &g.edges[id]
	weight, delta, adjusted := smtQZero(), smtQZero(), smtQZero()
	defer g.del(&weight)
	defer g.del(&delta)
	defer g.del(&adjusted)
	g.set(&weight, &edge.weight)
	selected := id
	for _, candidateID := range g.outEdges[edge.source] {
		candidate := &g.edges[candidateID]
		if candidate.target != edge.target || !candidate.enabled || g.cmp(&candidate.weight, &weight) <= 0 {
			continue
		}
		g.sub(&candidate.weight, &weight, &delta)
		g.add(gamma, &delta, &adjusted)
		if adjusted.IsNeg() {
			selected = candidateID
			g.set(gamma, &adjusted)
			g.set(&weight, &candidate.weight)
			g.numRelax++
		}
	}
	return selected
}

func (g *SMTRationalDiffGraph) NegativeCycleExplanations(tryRelax bool) ([]SMTLiteral, error) {
	if !g.validEdge(g.lastEnabledEdge) || g.edgeFeasible(&g.edges[g.lastEnabledEdge]) {
		return nil, errSMTDiffGraphDomain
	}
	last := g.lastEnabledEdge
	edge := &g.edges[last]
	gamma := smtQZero()
	defer g.del(&gamma)
	g.set(&gamma, &g.gamma[edge.source])
	result := make([]SMTLiteral, 0, 4)
	id := last
	for steps := 0; ; steps++ {
		if steps > len(g.edges) || !g.validEdge(id) {
			return nil, errSMTDiffGraphDomain
		}
		selected := id
		if tryRelax {
			selected = g.findRelaxedEdge(id, &gamma)
		}
		g.activity[id]++
		result = append(result, g.edges[selected].explanation)
		id = g.parent[g.edges[id].source]
		if id == last {
			break
		}
	}
	return result, g.takeErr(nil)
}

func (g *SMTRationalDiffGraph) Push() error {
	if g == nil {
		return errSMTDiffGraphDomain
	}
	g.scopes = append(g.scopes, smtRationalDiffScope{len(g.edges), len(g.enabledEdges), g.timestamp})
	return nil
}

func (g *SMTRationalDiffGraph) Pop(count uint32) error {
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
		g.inEdges[edge.target] = g.inEdges[edge.target][:len(g.inEdges[edge.target])-1]
		g.del(&edge.weight)
		g.edges = g.edges[:id]
		g.activity = g.activity[:id]
	}
	g.scopes = g.scopes[:level]
	return g.takeErr(nil)
}

// Assignment copies the value of variable into out.
func (g *SMTRationalDiffGraph) Assignment(variable int32, out *numeral.Rational) error {
	if !g.validVariable(variable) || out == nil {
		return errSMTDiffGraphDomain
	}
	return g.q.Copy(out, &g.assignment[variable])
}

func (g *SMTRationalDiffGraph) EdgeSource(id int32) (int32, error) {
	if !g.validEdge(id) {
		return -1, errSMTDiffGraphDomain
	}
	return g.edges[id].source, nil
}

func (g *SMTRationalDiffGraph) EdgeTarget(id int32) (int32, error) {
	if !g.validEdge(id) {
		return -1, errSMTDiffGraphDomain
	}
	return g.edges[id].target, nil
}

// EdgeWeight copies the weight of edge id into out.
func (g *SMTRationalDiffGraph) EdgeWeight(id int32, out *numeral.Rational) error {
	if !g.validEdge(id) || out == nil {
		return errSMTDiffGraphDomain
	}
	return g.q.Copy(out, &g.edges[id].weight)
}

func (g *SMTRationalDiffGraph) EdgeExplanation(id int32) (SMTLiteral, error) {
	if !g.validEdge(id) {
		return SMTNullLiteral, errSMTDiffGraphDomain
	}
	return g.edges[id].explanation, nil
}

func (g *SMTRationalDiffGraph) EdgeEnabled(id int32) (bool, error) {
	if !g.validEdge(id) {
		return false, errSMTDiffGraphDomain
	}
	return g.edges[id].enabled, nil
}

// MinimumEnabledEdge copies the least weight of an enabled source->target
// edge into weight and returns its explanation.
func (g *SMTRationalDiffGraph) MinimumEnabledEdge(source, target int32, weight *numeral.Rational) (SMTLiteral, bool, error) {
	if !g.validVariable(source) || !g.validVariable(target) || weight == nil {
		return SMTNullLiteral, false, errSMTDiffGraphDomain
	}
	explanation := SMTNullLiteral
	found := false
	for _, edgeID := range g.outEdges[source] {
		edge := &g.edges[edgeID]
		if edge.enabled && edge.target == target && (!found || g.cmp(&edge.weight, weight) < 0) {
			g.set(weight, &edge.weight)
			explanation = edge.explanation
			found = true
		}
	}
	return explanation, found, g.takeErr(nil)
}

func (g *SMTRationalDiffGraph) FindEdge(source, target int32) (int32, bool, error) {
	if !g.validVariable(source) || !g.validVariable(target) {
		return -1, false, errSMTDiffGraphDomain
	}
	for _, edgeID := range g.outEdges[source] {
		if g.edges[edgeID].target == target {
			return edgeID, true, nil
		}
	}
	return -1, false, nil
}

func (g *SMTRationalDiffGraph) NeighborsUndirected(variable int32) ([]int32, error) {
	if !g.validVariable(variable) {
		return nil, errSMTDiffGraphDomain
	}
	result := make([]int32, 0, len(g.outEdges[variable])+len(g.inEdges[variable]))
	for _, edgeID := range g.outEdges[variable] {
		result = append(result, g.edges[edgeID].target)
	}
	for _, edgeID := range g.inEdges[variable] {
		result = append(result, g.edges[edgeID].source)
	}
	return result, nil
}

// DFSUndirectedThread returns the source traversal's cyclic discovery thread.
func (g *SMTRationalDiffGraph) DFSUndirectedThread(start int32) ([]int32, error) {
	if !g.validVariable(start) {
		return nil, errSMTDiffGraphDomain
	}
	thread := make([]int32, len(g.assignment))
	discovered := make([]bool, len(g.assignment))
	explored := make([]bool, len(g.assignment))
	stack := []int32{start}
	discovered[start] = true
	previous := start
	for len(stack) != 0 {
		current := stack[len(stack)-1]
		neighbors, err := g.NeighborsUndirected(current)
		if err != nil || len(neighbors) == 0 {
			return nil, errSMTDiffGraphDomain
		}
		found := false
		for _, next := range neighbors {
			if !discovered[next] && !explored[next] {
				thread[previous] = next
				previous = next
				discovered[next] = true
				stack = append(stack, next)
				found = true
				break
			}
		}
		if !found {
			explored[current] = true
			stack = stack[:len(stack)-1]
		}
	}
	thread[previous] = start
	return thread, nil
}

func (g *SMTRationalDiffGraph) BFSUndirected(start int32) (parents, depths []int32, err error) {
	if !g.validVariable(start) {
		return nil, nil, errSMTDiffGraphDomain
	}
	parents = make([]int32, len(g.assignment))
	depths = make([]int32, len(g.assignment))
	parents[start] = -1
	visited := make([]bool, len(g.assignment))
	visited[start] = true
	queue := []int32{start}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		neighbors, neighborErr := g.NeighborsUndirected(current)
		if neighborErr != nil || len(neighbors) == 0 {
			return nil, nil, errSMTDiffGraphDomain
		}
		for _, next := range neighbors {
			if !visited[next] {
				parents[next] = current
				depths[next] = depths[current] + 1
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return parents, depths, nil
}

// ComputeZeroEdgeSCC runs the source Gabow-style DFS over enabled edges whose
// normalized weight is zero. Singleton components retain the source -1 ID.
func (g *SMTRationalDiffGraph) ComputeZeroEdgeSCC() ([]int32, error) {
	if g == nil || !g.IsFeasible() {
		return nil, errSMTDiffGraphDomain
	}
	count := len(g.assignment)
	dfsTime := make([]int32, count)
	component := make([]int32, count)
	unfinishedSet := make([]bool, count)
	for index := range dfsTime {
		dfsTime[index], component[index] = -1, -1
	}
	roots := make([]int32, 0, count)
	unfinished := make([]int32, 0, count)
	nextTime, nextComponent := int32(0), int32(0)
	gamma := smtQZero()
	defer g.del(&gamma)
	var visit func(int32)
	visit = func(variable int32) {
		dfsTime[variable] = nextTime
		nextTime++
		unfinishedSet[variable] = true
		unfinished = append(unfinished, variable)
		roots = append(roots, variable)
		for _, edgeID := range g.outEdges[variable] {
			edge := &g.edges[edgeID]
			if !edge.enabled {
				continue
			}
			g.setGamma(edge, &gamma)
			if !gamma.IsZero() {
				continue
			}
			target := edge.target
			if dfsTime[target] == -1 {
				visit(target)
			} else if unfinishedSet[target] {
				for dfsTime[roots[len(roots)-1]] > dfsTime[target] {
					roots = roots[:len(roots)-1]
				}
			}
		}
		if variable == roots[len(roots)-1] {
			size := 0
			last := int32(-1)
			for last != variable {
				last = unfinished[len(unfinished)-1]
				unfinished = unfinished[:len(unfinished)-1]
				unfinishedSet[last] = false
				component[last] = nextComponent
				size++
			}
			if size == 1 {
				component[last] = -1
			} else {
				nextComponent++
			}
			roots = roots[:len(roots)-1]
		}
	}
	for variable := int32(0); int(variable) < count; variable++ {
		if dfsTime[variable] == -1 {
			visit(variable)
		}
	}
	return component, g.takeErr(nil)
}

func (g *SMTRationalDiffGraph) ComputeZeroSuccessors(variable int32) ([]int32, error) {
	if !g.validVariable(variable) {
		return nil, errSMTDiffGraphDomain
	}
	seen := make([]bool, len(g.assignment))
	seen[variable] = true
	result := []int32{variable}
	gamma := smtQZero()
	defer g.del(&gamma)
	for index := 0; index < len(result); index++ {
		current := result[index]
		for _, edgeID := range g.outEdges[current] {
			edge := &g.edges[edgeID]
			if !edge.enabled {
				continue
			}
			g.setGamma(edge, &gamma)
			if gamma.IsZero() && !seen[edge.target] {
				seen[edge.target] = true
				result = append(result, edge.target)
			}
		}
	}
	return result, g.takeErr(nil)
}

type smtRationalDiffBFSElement struct {
	variable    int32
	parentIndex int
	edgeID      int32
}

// FindShortestZeroEdgePath performs the source breadth-first search over
// enabled normalized-zero edges older than timestamp. Explanations follow the
// source callback order: final edge first, then predecessors toward source.
func (g *SMTRationalDiffGraph) FindShortestZeroEdgePath(source, target int32, timestamp uint32) ([]SMTLiteral, bool, error) {
	if !g.validVariable(source) || !g.validVariable(target) {
		return nil, false, errSMTDiffGraphDomain
	}
	queue := []smtRationalDiffBFSElement{{variable: source, parentIndex: -1, edgeID: -1}}
	marked := make([]bool, len(g.assignment))
	marked[source] = true
	gamma := smtQZero()
	defer g.del(&gamma)
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		for _, edgeID := range g.outEdges[current.variable] {
			edge := &g.edges[edgeID]
			if !edge.enabled || edge.timestamp >= timestamp {
				continue
			}
			g.setGamma(edge, &gamma)
			if !gamma.IsZero() {
				continue
			}
			if edge.target == target {
				result := []SMTLiteral{edge.explanation}
				parent := head
				for parent >= 0 && queue[parent].edgeID != -1 {
					result = append(result, g.edges[queue[parent].edgeID].explanation)
					parent = queue[parent].parentIndex
				}
				return result, true, g.takeErr(nil)
			}
			if !marked[edge.target] {
				marked[edge.target] = true
				queue = append(queue, smtRationalDiffBFSElement{variable: edge.target, parentIndex: head, edgeID: edgeID})
			}
		}
	}
	return nil, false, g.takeErr(nil)
}

func (g *SMTRationalDiffGraph) IncrementAssignment(variable int32, increment *numeral.Rational) error {
	if !g.validVariable(variable) || increment == nil {
		return errSMTDiffGraphDomain
	}
	return g.q.Add(&g.assignment[variable], increment, &g.assignment[variable])
}

func (g *SMTRationalDiffGraph) SetAssignment(variable int32, value *numeral.Rational) error {
	if !g.validVariable(variable) || value == nil {
		return errSMTDiffGraphDomain
	}
	return g.q.Copy(&g.assignment[variable], value)
}

func (g *SMTRationalDiffGraph) Timestamp() uint32 {
	if g == nil {
		return 0
	}
	return g.timestamp
}

func (g *SMTRationalDiffGraph) Reset() {
	if g == nil {
		return
	}
	g.release()
	*g = SMTRationalDiffGraph{q: g.q, qErr: g.qErr, lastEnabledEdge: -1}
	g.heap.graph = g
}

// SetToZero translates every assignment by the selected value. Difference
// constraints are translation invariant, so feasibility is preserved.
func (g *SMTRationalDiffGraph) SetToZero(variable int32) error {
	if !g.validVariable(variable) || !g.IsFeasible() {
		return errSMTDiffGraphDomain
	}
	if g.assignment[variable].IsZero() {
		return nil
	}
	offset := smtQZero()
	g.set(&offset, &g.assignment[variable])
	for index := range g.assignment {
		g.sub(&g.assignment[index], &offset, &g.assignment[index])
	}
	g.del(&offset)
	return g.takeErr(nil)
}

// SetPairToZero implements the source helper for disconnected variables. If
// translation does not zero both values, zero-weight edges join the components.
func (g *SMTRationalDiffGraph) SetPairToZero(first, second int32) error {
	if !g.validVariable(first) || !g.validVariable(second) || !g.IsFeasible() {
		return errSMTDiffGraphDomain
	}
	selected := second
	if !g.assignment[first].IsZero() {
		selected = first
	}
	if err := g.SetToZero(selected); err != nil {
		return err
	}
	if g.assignment[first].IsZero() && g.assignment[second].IsZero() {
		return nil
	}
	zero := smtQIntPtr(0)
	forward, err := g.AddEdge(first, second, zero, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, err := g.EnableEdge(forward); err != nil || !feasible {
		if err != nil {
			return err
		}
		return errSMTDiffGraphDomain
	}
	backward, err := g.AddEdge(second, first, zero, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, err := g.EnableEdge(backward); err != nil || !feasible {
		if err != nil {
			return err
		}
		return errSMTDiffGraphDomain
	}
	return nil
}

// FindDirectlySubsumed returns disabled parallel edges whose bounds are no
// stronger than the selected edge. It preserves source adjacency order.
func (g *SMTRationalDiffGraph) FindDirectlySubsumed(id int32) ([]int32, error) {
	if !g.validEdge(id) {
		return nil, errSMTDiffGraphDomain
	}
	edge := &g.edges[id]
	result := make([]int32, 0)
	candidates := g.outEdges[edge.source]
	useOutgoing := len(candidates) < len(g.inEdges[edge.target])
	if !useOutgoing {
		candidates = g.inEdges[edge.target]
	}
	for _, candidateID := range candidates {
		g.propagationCost++
		candidate := &g.edges[candidateID]
		matches := candidate.target == edge.target
		if !useOutgoing {
			matches = candidate.source == edge.source
		}
		if candidateID != id && !candidate.enabled && matches && g.cmp(&candidate.weight, &edge.weight) >= 0 {
			result = append(result, candidateID)
		}
	}
	return result, g.takeErr(nil)
}

// FindTwoHopSubsumed extends direct subsumption through one enabled incoming
// edge at the source and one enabled outgoing edge at the destination.
func (g *SMTRationalDiffGraph) FindTwoHopSubsumed(id int32) ([]int32, error) {
	if !g.validEdge(id) {
		return nil, errSMTDiffGraphDomain
	}
	result, err := g.FindDirectlySubsumed(id)
	if err != nil {
		return nil, err
	}
	edge := &g.edges[id]
	combined := smtQZero()
	defer g.del(&combined)
	for _, prefixID := range g.inEdges[edge.source] {
		prefix := &g.edges[prefixID]
		if !prefix.enabled || prefix.source == edge.target {
			continue
		}
		g.add(&prefix.weight, &edge.weight, &combined)
		for _, candidateID := range g.outEdges[prefix.source] {
			g.propagationCost++
			candidate := &g.edges[candidateID]
			if !candidate.enabled && candidate.target == edge.target && g.cmp(&candidate.weight, &combined) >= 0 {
				result = append(result, candidateID)
			}
		}
	}
	for _, suffixID := range g.outEdges[edge.target] {
		suffix := &g.edges[suffixID]
		if !suffix.enabled || suffix.target == edge.source {
			continue
		}
		g.add(&suffix.weight, &edge.weight, &combined)
		for _, candidateID := range g.inEdges[suffix.target] {
			g.propagationCost++
			candidate := &g.edges[candidateID]
			if !candidate.enabled && candidate.source == edge.source && g.cmp(&candidate.weight, &combined) >= 0 {
				result = append(result, candidateID)
			}
		}
	}
	return result, g.takeErr(nil)
}

// ExplainSubsumedLazy finds an enabled path, no newer than bridge, whose
// weight implies the disabled subsumed edge. Explanations are returned from
// destination to source, matching the source parent walk.
func (g *SMTRationalDiffGraph) ExplainSubsumedLazy(bridgeID, subsumedID int32) ([]SMTLiteral, error) {
	if !g.validEdge(bridgeID) || !g.validEdge(subsumedID) || !g.edges[bridgeID].enabled || !g.heap.empty() || len(g.visited) != 0 {
		return nil, errSMTDiffGraphDomain
	}
	bridge := &g.edges[bridgeID]
	subsumed := &g.edges[subsumedID]
	source, destination := subsumed.source, subsumed.target
	g.keep(g.q.Reset(&g.gamma[source]))
	g.set(&g.gamma[destination], &subsumed.weight)
	g.heap.insert(source)
	g.visited = append(g.visited, source)
	distance := smtQZero()
	defer g.del(&distance)

	for !g.heap.empty() {
		variable := g.heap.eraseMin()
		g.mark[variable] = smtDiffProcessed
		for _, edgeID := range g.outEdges[variable] {
			edge := &g.edges[edgeID]
			if !edge.enabled || edge.timestamp > bridge.timestamp {
				continue
			}
			next := edge.target
			g.add(&g.gamma[variable], &edge.weight, &distance)
			if g.mark[next] != smtDiffUnmarked && g.cmp(&g.gamma[next], &distance) <= 0 {
				continue
			}
			g.set(&g.gamma[next], &distance)
			g.parent[next] = edgeID
			if next == destination && g.cmp(&distance, &subsumed.weight) <= 0 {
				g.resetMarks()
				g.heap.reset()
				result := make([]SMTLiteral, 0, 4)
				current := destination
				for steps := 0; current != source; steps++ {
					if steps > len(g.edges) || !g.validEdge(g.parent[current]) {
						return nil, errSMTDiffGraphDomain
					}
					parentID := g.parent[current]
					g.activity[parentID]++
					result = append(result, g.edges[parentID].explanation)
					current = g.edges[parentID].source
				}
				return result, g.takeErr(nil)
			}
			switch g.mark[next] {
			case smtDiffUnmarked:
				g.visited = append(g.visited, next)
				g.mark[next] = smtDiffFound
				g.heap.insert(next)
			case smtDiffProcessed:
				g.mark[next] = smtDiffFound
				g.heap.insert(next)
			case smtDiffFound:
				g.heap.decreased(next)
			}
		}
	}
	g.resetMarks()
	return nil, g.takeErr(errSMTDiffGraphDomain)
}

func (g *SMTRationalDiffGraph) NumNodes() int {
	if g == nil {
		return 0
	}
	return len(g.assignment)
}
func (g *SMTRationalDiffGraph) NumEdges() int {
	if g == nil {
		return 0
	}
	return len(g.edges)
}

// Concrete indexed minimum heap ordered by the graph gamma values.
type smtRationalDiffHeap struct {
	graph     *SMTRationalDiffGraph
	variables []int32
	positions []int
}

func (h *smtRationalDiffHeap) empty() bool { return len(h.variables) == 0 }
func (h *smtRationalDiffHeap) ensure(v int32) {
	for int(v) >= len(h.positions) {
		h.positions = append(h.positions, -1)
	}
}
func (h *smtRationalDiffHeap) less(i, j int) bool {
	return h.graph.cmp(&h.graph.gamma[h.variables[i]], &h.graph.gamma[h.variables[j]]) < 0
}
func (h *smtRationalDiffHeap) swap(i, j int) {
	h.variables[i], h.variables[j] = h.variables[j], h.variables[i]
	h.positions[h.variables[i]], h.positions[h.variables[j]] = i, j
}
func (h *smtRationalDiffHeap) up(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if !h.less(index, parent) {
			break
		}
		h.swap(index, parent)
		index = parent
	}
}
func (h *smtRationalDiffHeap) down(index int) {
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
func (h *smtRationalDiffHeap) insert(v int32) {
	h.ensure(v)
	if h.positions[v] >= 0 {
		return
	}
	h.positions[v] = len(h.variables)
	h.variables = append(h.variables, v)
	h.up(len(h.variables) - 1)
}
func (h *smtRationalDiffHeap) decreased(v int32) {
	h.ensure(v)
	if h.positions[v] >= 0 {
		h.up(h.positions[v])
	}
}
func (h *smtRationalDiffHeap) eraseMin() int32 {
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
func (h *smtRationalDiffHeap) reset() {
	for _, v := range h.variables {
		h.positions[v] = -1
	}
	h.variables = h.variables[:0]
}
