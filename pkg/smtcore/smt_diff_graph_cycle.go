package smtcore

// Source: src/smt/diff_logic.h traverse_neg_cycle2 and check_explanation.

// negativeCycleEdgesWithShortcuts preserves strict inconsistency while using
// enabled weaker edges to shorten the parent cycle. The native try_relax
// argument is unused. Theory UTVPI's new_edge is empty; difference logic's
// callback is disabled by constructor-fixed m_theory_resolve=false with no
// retained writer. Neither concrete consumer needs a generic cut callback.
func (graph *SMTInfRationalDiffGraph) negativeCycleEdgesWithShortcuts() ([]int32, error) {
	if !graph.validEdge(graph.lastEnabledEdge) || graph.edgeFeasible(&graph.edges[graph.lastEnabledEdge]) {
		return nil, graph.takeErr(errSMTDiffGraphDomain)
	}
	last := graph.lastEnabledEdge
	gamma, potential := newSMTInfIntRational(), newSMTInfIntRational()
	delta, relaxed, weight := newSMTInfIntRational(), newSMTInfIntRational(), newSMTInfIntRational()
	potentials := []smtInfIntRational{}
	defer func() {
		for _, v := range []*smtInfIntRational{&gamma, &potential, &delta, &relaxed, &weight} {
			graph.del(v)
		}
		for i := range potentials {
			graph.del(&potentials[i])
		}
	}()
	graph.set(&gamma, &graph.gamma[graph.edges[last].source])
	if gamma.sign() >= 0 {
		return nil, graph.takeErr(errSMTDiffGraphDomain)
	}
	nodes := []int32{}
	edges := []int32{}
	id := last
	for steps := 0; ; steps++ {
		if steps > len(graph.edges) || !graph.validEdge(id) {
			return nil, graph.takeErr(errSMTDiffGraphDomain)
		}
		edges = append(edges, id)
		edge := &graph.edges[id]
		source := edge.source
		graph.add(&potential, &edge.weight, &potential)
		for _, candidateID := range graph.outEdges[source] {
			candidate := &graph.edges[candidateID]
			if candidateID == id || !candidate.enabled {
				continue
			}
			for j, target := range nodes {
				if target != candidate.target {
					continue
				}
				graph.sub(&candidate.weight, &potential, &delta)
				graph.add(&delta, &potentials[j], &delta)
				graph.add(&gamma, &delta, &relaxed)
				if delta.sign() >= 0 && relaxed.sign() < 0 {
					graph.set(&gamma, &relaxed)
					nodes = nodes[:j+1]
					for k := j + 1; k < len(potentials); k++ {
						graph.del(&potentials[k])
					}
					potentials = potentials[:j+1]
					edges = append(edges[:j+1], candidateID)
					graph.add(&potentials[j], &candidate.weight, &potential)
					break
				}
			}
		}
		copied := newSMTInfIntRational()
		graph.set(&copied, &potential)
		potentials = append(potentials, copied)
		nodes = append(nodes, source)
		id = graph.parent[source]
		if id == last {
			break
		}
	}
	for i, id := range edges {
		previous := edges[(i+len(edges)-1)%len(edges)]
		edge := &graph.edges[id]
		if edge.target != graph.edges[previous].source {
			return nil, graph.takeErr(errSMTDiffGraphDomain)
		}
		graph.add(&weight, &edge.weight, &weight)
	}
	if weight.sign() >= 0 {
		return nil, graph.takeErr(errSMTDiffGraphDomain)
	}
	return edges, graph.takeErr(nil)
}
