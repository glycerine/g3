package smtcore

// Source: src/smt/theory_utvpi_def.h add_ineq, enable_edge,
// check_z_consistency, is_parity_ok and enforce_parity (rdl_ext).

import "github.com/glycerine/g3/pkg/util/numeral"

// A source UTVPI variable has two graph vertices: positive 2*v and negative
// 2*v+1. Its value is half their potential difference. Edge coefficients are
// kept separately from literals so unary constraints retain their factor two
// in conflict explanations.
type smtUTVPIGraph struct {
	graph        *SMTInfRationalDiffGraph
	integer      []bool
	coefficients []uint32
	scopes       []int
	// parityVariable is the variable of the last parityConflict, which
	// check_z_consistency's verbose line prints.
	parityVariable int
}

type smtUTVPIGraphTerm struct {
	variable int32
	negative bool
}

func newSMTUTVPIGraph() *smtUTVPIGraph {
	return &smtUTVPIGraph{graph: NewSMTInfRationalDiffGraph(nil)}
}

func (g *smtUTVPIGraph) addVariable(integer bool) (int32, error) {
	v := int32(len(g.integer))
	if err := g.graph.InitVariable(2 * v); err != nil {
		return -1, err
	}
	if err := g.graph.InitVariable(2*v + 1); err != nil {
		return -1, err
	}
	g.integer = append(g.integer, integer)
	return v, nil
}

// addInequality encodes sum(terms)+weight <= 0 as two adjacent edges.
// Terms have already been combined and checked by the UTVPI linearizer.
func (g *smtUTVPIGraph) addInequality(terms []smtUTVPIGraphTerm, weight *smtInfIntRational, literal SMTLiteral) (int32, error) {
	if weight == nil || len(terms) < 1 || len(terms) > 2 {
		return -1, errSMTDiffGraphDomain
	}
	for _, term := range terms {
		if term.variable < 0 || int(term.variable) >= len(g.integer) {
			return -1, errSMTDiffGraphDomain
		}
	}
	if len(terms) == 2 && terms[0].variable == terms[1].variable {
		return -1, errSMTDiffGraphDomain
	}
	first := 2 * terms[0].variable
	if terms[0].negative {
		first++
	}
	q := g.graph.q
	edgeWeight := newSMTInfIntRational()
	defer edgeWeight.del(q)
	if err := smtInfIntSet(q, &edgeWeight, weight); err != nil {
		return -1, err
	}
	if err := smtInfIntNeg(q, &edgeWeight); err != nil {
		return -1, err
	}
	source, target := first^1, first
	otherSource, otherTarget := source, target
	coefficient := uint32(2)
	if len(terms) == 1 {
		if err := smtInfIntAdd(q, &edgeWeight, &edgeWeight, &edgeWeight); err != nil {
			return -1, err
		}
	} else {
		second := 2 * terms[1].variable
		if terms[1].negative {
			second++
		}
		source, target = second^1, first
		otherSource, otherTarget = first^1, second
		if terms[0].negative && terms[1].negative {
			source, target, otherSource, otherTarget = otherSource, otherTarget, source, target
		}
		coefficient = 1
	}
	// Popped graph edges can reuse their IDs; discard matching stale weights.
	if len(g.coefficients) > len(g.graph.edges) {
		g.coefficients = g.coefficients[:len(g.graph.edges)]
	}
	for len(g.coefficients) < len(g.graph.edges) {
		// Model normalization can add unconditional, zero-weight edges.
		g.coefficients = append(g.coefficients, 0)
	}
	id, err := g.graph.AddEdge(source, target, &edgeWeight, literal)
	if err != nil {
		return -1, err
	}
	g.coefficients = append(g.coefficients, coefficient)
	if _, err := g.graph.AddEdge(otherSource, otherTarget, &edgeWeight, literal); err != nil {
		return -1, err
	}
	g.coefficients = append(g.coefficients, coefficient)
	return id, nil
}

func (g *smtUTVPIGraph) enablePair(id int32) (bool, error) {
	if !g.graph.validEdge(id) || !g.graph.validEdge(id+1) {
		return false, errSMTDiffGraphDomain
	}
	ok, err := g.graph.EnableEdge(id)
	if err != nil || !ok {
		return ok, err
	}
	return g.graph.EnableEdge(id + 1)
}

func (g *smtUTVPIGraph) parityOK(variable int32) (bool, error) {
	if variable < 0 || int(variable) >= len(g.integer) {
		return false, errSMTDiffGraphDomain
	}
	q := g.graph.q
	// rational::is_even is false for non-integral rationals. Real vertices
	// can be visited while repairing a neighboring integer's parity.
	firstEven, err := q.IsEven(&g.graph.assignment[2*variable].rational)
	if err != nil {
		return false, err
	}
	secondEven, err := q.IsEven(&g.graph.assignment[2*variable+1].rational)
	return firstEven == secondEven, err
}

// parityConflict returns the two shortest tight paths witnessing an odd
// signed pair in one SCC. A feasible rational graph can still be integer UNSAT.
func (g *smtUTVPIGraph) parityConflict() ([]int32, error) {
	components, err := g.graph.ComputeZeroEdgeSCC()
	if err != nil {
		return nil, err
	}
	for variable, integer := range g.integer {
		if !integer {
			continue
		}
		// C++ only SASSERTs r1.is_int()/r2.is_int(); release builds
		// compare is_even, which parityOK reproduces for fractions.
		ok, err := g.parityOK(int32(variable))
		if err != nil {
			return nil, err
		}
		first := int32(2 * variable)
		second := first + 1
		if ok || components[first] < 0 || components[first] != components[second] {
			continue
		}
		forward, found, err := g.graph.FindShortestZeroEdgePath(first, second)
		if err != nil || !found {
			return nil, errSMTDiffGraphDomain
		}
		backward, found, err := g.graph.FindShortestZeroEdgePath(second, first)
		if err != nil || !found {
			return nil, errSMTDiffGraphDomain
		}
		g.parityVariable = variable
		return append(forward, backward...), nil
	}
	return nil, nil
}

// enforceParity requires successful parityConflict first. Decrementing the
// entire tight successor set preserves every enabled graph inequality.
func (g *smtUTVPIGraph) enforceParity() error {
	todo := []int32{}
	for variable, integer := range g.integer {
		if !integer {
			continue
		}
		ok, err := g.parityOK(int32(variable))
		if err != nil {
			return err
		}
		if !ok {
			todo = append(todo, int32(variable))
		}
	}
	decrement := smtInfIntRational{rational: smtQInt(-1)}
	for len(todo) > 0 {
		variable := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		ok, err := g.parityOK(variable)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		first, second := 2*variable, 2*variable+1
		successors, err := g.graph.ComputeZeroSuccessors(first)
		if err != nil {
			return err
		}
		for _, vertex := range successors {
			if vertex == second {
				successors, err = g.graph.ComputeZeroSuccessors(second)
				if err != nil {
					return err
				}
				break
			}
		}
		for _, vertex := range successors {
			if err := g.graph.IncrementAssignment(vertex, &decrement); err != nil {
				return err
			}
			ok, err := g.parityOK(vertex / 2)
			if err != nil {
				return err
			}
			if !ok {
				todo = append(todo, vertex/2)
			}
		}
	}
	// C++ ends with SASSERT(m_graph.is_feasible()), debug-only.
	return nil
}

func (g *smtUTVPIGraph) push() error {
	if err := g.graph.Push(); err != nil {
		return err
	}
	g.scopes = append(g.scopes, len(g.integer))
	return nil
}

func (g *smtUTVPIGraph) pop(count uint32) error {
	if count == 0 || int(count) > len(g.scopes) {
		return errSMTDiffGraphDomain
	}
	level := len(g.scopes) - int(count)
	if err := g.graph.Pop(count); err != nil {
		return err
	}
	g.integer = g.integer[:g.scopes[level]]
	g.scopes = g.scopes[:level]
	if len(g.coefficients) > len(g.graph.edges) {
		g.coefficients = g.coefficients[:len(g.graph.edges)]
	}
	return nil
}

// modelValues implements the graph part of init_model, compute_delta and
// mk_value. The distinguished zero variable is created by the theory owner.
// Values own their rationals. Source only debug-asserts integer-valued Int
// assignments here; its arithmetic factory performs the release rejection.
func (g *smtUTVPIGraph) modelValues(zeroVariable int32) ([]numeral.Rational, error) {
	if zeroVariable < 0 || int(zeroVariable) >= len(g.integer) {
		return nil, errSMTDiffGraphDomain
	}
	conflict, err := g.parityConflict()
	if err != nil {
		return nil, err
	}
	if len(conflict) != 0 {
		return nil, errSMTDiffGraphDomain
	}
	if err := g.enforceParity(); err != nil {
		return nil, err
	}
	if err := g.graph.SetPairToZero(2*zeroVariable, 2*zeroVariable+1); err != nil {
		return nil, err
	}
	// compute_delta and mk_value. The caller frees the returned values.
	q := g.graph.q
	delta := smtQInt(1)
	bound, limit, scaled := newSMTInfIntRational(), smtQZero(), smtQZero()
	defer func() {
		smtQDel(q, &delta)
		bound.del(q)
		smtQDel(q, &limit)
		smtQDel(q, &scaled)
	}()
	for i := range g.graph.edges {
		edge := &g.graph.edges[i]
		if !edge.enabled {
			continue
		}
		// b = tgt - src - w
		if err := smtInfIntSub(q, &g.graph.assignment[edge.target], &g.graph.assignment[edge.source], &bound); err != nil {
			return nil, err
		}
		if err := smtInfIntSub(q, &bound, &edge.weight, &bound); err != nil {
			return nil, err
		}
		if bound.sign() > 0 {
			return nil, errSMTDiffGraphDomain
		}
		if bound.infinitesimal > 0 {
			// new_delta = -b.num / eps_r
			if err := q.Copy(&limit, &bound.rational); err != nil {
				return nil, err
			}
			if err := q.Neg(&limit); err != nil {
				return nil, err
			}
			eps := smtQInt(0)
			if err := q.SetInt64(&eps, int64(bound.infinitesimal)); err != nil {
				return nil, err
			}
			err := q.Div(&limit, &eps, &limit)
			if e := smtQDel(q, &eps); err == nil {
				err = e
			}
			if err != nil {
				return nil, err
			}
			c, err := q.Compare(&limit, &delta)
			if err != nil {
				return nil, err
			}
			if c < 0 {
				if err := q.Copy(&delta, &limit); err != nil {
					return nil, err
				}
			}
		}
	}
	values := make([]numeral.Rational, len(g.integer))
	for i := range values {
		values[i] = smtQZero()
	}
	difference := newSMTInfIntRational()
	defer difference.del(q)
	for variable := range g.integer {
		// num = (val.rational + delta*val.infinitesimal) / 2, val = val1 - val2
		if err := smtInfIntSub(q, &g.graph.assignment[2*variable], &g.graph.assignment[2*variable+1], &difference); err != nil {
			return values, err
		}
		if err := q.SetInt64(&scaled, int64(difference.infinitesimal)); err != nil {
			return values, err
		}
		if err := q.Mul(&delta, &scaled, &scaled); err != nil {
			return values, err
		}
		if err := q.Add(&difference.rational, &scaled, &values[variable]); err != nil {
			return values, err
		}
		if err := q.Div(&values[variable], smtQIntPtr(2), &values[variable]); err != nil {
			return values, err
		}
	}
	return values, nil
}
