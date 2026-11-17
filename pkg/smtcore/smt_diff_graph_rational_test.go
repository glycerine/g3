package smtcore

import (
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func diffRat(value int64) *big.Rat { return new(big.Rat).SetInt64(value) }

func TestSMTRationalDiffGraphFeasibleActivationAndAssignment(t *testing.T) {
	g := NewSMTRationalDiffGraph(testQ())
	for variable := int32(0); variable < 4; variable++ {
		if err := g.InitVariable(variable); err != nil {
			t.Fatal(err)
		}
	}
	edges := [][3]int64{{0, 1, 2}, {1, 2, -5}, {2, 3, 1}}
	for index, item := range edges {
		id, err := g.testAddEdge(int32(item[0]), int32(item[1]), diffRat(item[2]), NewSMTLiteral(int32(index+1), false))
		if err != nil {
			t.Fatal(err)
		}
		feasible, err := g.EnableEdge(id)
		if err != nil || !feasible {
			t.Fatal("feasible activation", index, feasible, err)
		}
	}
	if !g.IsFeasible() || g.NumNodes() != 4 || g.NumEdges() != 3 {
		t.Fatal("graph dimensions or feasibility")
	}
	want := []*big.Rat{diffRat(0), diffRat(0), diffRat(-5), diffRat(-4)}
	for variable := int32(0); variable < 4; variable++ {
		got, err := g.testAssignment(variable)
		if err != nil || got.Cmp(want[variable]) != 0 {
			t.Fatal("assignment", variable, got, err)
		}
	}
}

func TestSMTRationalDiffGraphNegativeCycleExplanationAndRollback(t *testing.T) {
	g := NewSMTRationalDiffGraph(testQ())
	for variable := int32(0); variable < 3; variable++ {
		if err := g.InitVariable(variable); err != nil {
			t.Fatal(err)
		}
	}
	literals := []SMTLiteral{NewSMTLiteral(7, false), NewSMTLiteral(8, true), NewSMTLiteral(9, false)}
	input := [][3]int64{{0, 1, 1}, {1, 2, 1}, {2, 0, -3}}
	for index, item := range input {
		id, err := g.testAddEdge(int32(item[0]), int32(item[1]), diffRat(item[2]), literals[index])
		if err != nil {
			t.Fatal(err)
		}
		feasible, err := g.EnableEdge(id)
		if err != nil {
			t.Fatal(err)
		}
		if feasible != (index != 2) {
			t.Fatal("negative cycle frontier", index, feasible)
		}
	}
	if g.IsFeasible() {
		t.Fatal("negative cycle reported feasible")
	}
	got, err := g.NegativeCycleExplanations(false)
	if err != nil {
		t.Fatal(err)
	}
	want := []SMTLiteral{literals[2], literals[1], literals[0]}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("cycle order", got, want)
	}
	for variable := int32(0); variable < 3; variable++ {
		value, err := g.testAssignment(variable)
		if err != nil || value.Sign() != 0 {
			t.Fatal("failed activation did not restore assignment", variable, value, err)
		}
	}
}

func TestSMTRationalDiffGraphScopesAndDomainGuards(t *testing.T) {
	g := NewSMTRationalDiffGraph(testQ())
	if err := g.InitVariable(1); err != nil {
		t.Fatal(err)
	}
	base, err := g.testAddEdge(0, 1, diffRat(4), SMTTrueLiteral)
	if err != nil {
		t.Fatal(err)
	}
	if feasible, err := g.EnableEdge(base); err != nil || !feasible {
		t.Fatal(feasible, err)
	}
	if err = g.Push(); err != nil {
		t.Fatal(err)
	}
	temporary, err := g.testAddEdge(1, 0, diffRat(-4), SMTFalseLiteral)
	if err != nil {
		t.Fatal(err)
	}
	if feasible, err := g.EnableEdge(temporary); err != nil || !feasible {
		t.Fatal(feasible, err)
	}
	if err = g.Pop(1); err != nil || g.NumEdges() != 1 || !g.IsFeasible() {
		t.Fatal("scope rollback", g.NumEdges(), err)
	}
	if _, err = g.EnableEdge(temporary); !errors.Is(err, errSMTDiffGraphDomain) {
		t.Fatal("retired edge guard", err)
	}
	if err = g.Pop(0); !errors.Is(err, errSMTDiffGraphDomain) {
		t.Fatal("zero scope guard", err)
	}
	if err = g.InitVariable(0); !errors.Is(err, errSMTDiffGraphDomain) {
		t.Fatal("incident variable reset guard", err)
	}
}

func TestSMTRationalDiffGraphZeroNormalizationAndEdgeAccessors(t *testing.T) {
	g := NewSMTRationalDiffGraph(testQ())
	for variable := int32(0); variable < 4; variable++ {
		if err := g.InitVariable(variable); err != nil {
			t.Fatal(err)
		}
	}
	first, err := g.testAddEdge(0, 1, diffRat(-2), NewSMTLiteral(11, false))
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.testAddEdge(2, 3, diffRat(-5), NewSMTLiteral(12, true))
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range []int32{first, second} {
		if feasible, enableErr := g.EnableEdge(edge); enableErr != nil || !feasible {
			t.Fatal(feasible, enableErr)
		}
	}
	if err = g.SetPairToZero(1, 3); err != nil {
		t.Fatal(err)
	}
	firstValue, err := g.testAssignment(1)
	if err != nil {
		t.Fatal(err)
	}
	secondValue, err := g.testAssignment(3)
	if err != nil || firstValue.Cmp(secondValue) != 0 {
		t.Fatal("pair normalization", firstValue, secondValue, err)
	}
	if g.NumEdges() != 4 || !g.IsFeasible() {
		t.Fatal("joining equality edges", g.NumEdges())
	}
	source, err := g.EdgeSource(first)
	if err != nil || source != 0 {
		t.Fatal(source, err)
	}
	target, err := g.EdgeTarget(first)
	if err != nil || target != 1 {
		t.Fatal(target, err)
	}
	weight, err := g.testEdgeWeight(first)
	if err != nil || weight.Cmp(diffRat(-2)) != 0 {
		t.Fatal(weight, err)
	}
	weight.SetInt64(99)
	weight, _ = g.testEdgeWeight(first)
	if weight.Cmp(diffRat(-2)) != 0 {
		t.Fatal("edge weight escaped graph ownership", weight)
	}
	literal, err := g.EdgeExplanation(second)
	if err != nil || literal != NewSMTLiteral(12, true) {
		t.Fatal(literal, err)
	}
}

func TestSMTRationalDiffGraphDirectAndTwoHopSubsumption(t *testing.T) {
	g := NewSMTRationalDiffGraph(testQ())
	for variable := int32(0); variable < 4; variable++ {
		if err := g.InitVariable(variable); err != nil {
			t.Fatal(err)
		}
	}
	add := func(source, target int32, weight int64, enabled bool) int32 {
		t.Helper()
		id, err := g.testAddEdge(source, target, diffRat(weight), SMTNullLiteral)
		if err != nil {
			t.Fatal(err)
		}
		if enabled {
			feasible, enableErr := g.EnableEdge(id)
			if enableErr != nil || !feasible {
				t.Fatal(feasible, enableErr)
			}
		}
		return id
	}
	bridge := add(0, 1, 2, true)
	parallel := add(0, 1, 3, false)
	_ = add(0, 1, 1, false)
	_ = add(2, 0, 1, true)
	prefix := add(2, 1, 4, false)
	_ = add(1, 3, 1, true)
	suffix := add(0, 3, 4, false)

	direct, err := g.FindDirectlySubsumed(bridge)
	if err != nil || !reflect.DeepEqual(direct, []int32{parallel}) {
		t.Fatal("direct subsumption", direct, err)
	}
	twoHop, err := g.FindTwoHopSubsumed(bridge)
	if err != nil || !reflect.DeepEqual(twoHop, []int32{parallel, prefix, suffix}) {
		t.Fatal("two-hop subsumption", twoHop, err)
	}
}

func TestSMTRationalDiffGraphLazySubsumptionExplanationTimestamp(t *testing.T) {
	g := NewSMTRationalDiffGraph(testQ())
	for variable := int32(0); variable < 3; variable++ {
		if err := g.InitVariable(variable); err != nil {
			t.Fatal(err)
		}
	}
	add := func(source, target int32, weight int64, literal SMTLiteral, enabled bool) int32 {
		t.Helper()
		id, err := g.testAddEdge(source, target, diffRat(weight), literal)
		if err != nil {
			t.Fatal(err)
		}
		if enabled {
			feasible, enableErr := g.EnableEdge(id)
			if enableErr != nil || !feasible {
				t.Fatal(feasible, enableErr)
			}
		}
		return id
	}
	firstLiteral := NewSMTLiteral(21, false)
	secondLiteral := NewSMTLiteral(22, true)
	_ = add(0, 1, 1, firstLiteral, true)
	bridge := add(1, 2, 1, secondLiteral, true)
	subsumed := add(0, 2, 3, SMTNullLiteral, false)
	_ = add(0, 2, 0, NewSMTLiteral(23, false), true) // newer than bridge

	explanation, err := g.ExplainSubsumedLazy(bridge, subsumed)
	if err != nil || !reflect.DeepEqual(explanation, []SMTLiteral{secondLiteral, firstLiteral}) {
		t.Fatal("timestamp-bounded explanation", explanation, err)
	}
	if _, err = g.ExplainSubsumedLazy(subsumed, subsumed); !errors.Is(err, errSMTDiffGraphDomain) {
		t.Fatal("disabled bridge guard", err)
	}
}

func TestSMTRationalDiffGraphZeroSCCReachabilityAndTraversal(t *testing.T) {
	g := NewSMTRationalDiffGraph(testQ())
	for variable := int32(0); variable < 4; variable++ {
		if err := g.InitVariable(variable); err != nil {
			t.Fatal(err)
		}
	}
	add := func(source, target int32, weight int64, literal SMTLiteral, enabled bool) int32 {
		t.Helper()
		id, err := g.testAddEdge(source, target, diffRat(weight), literal)
		if err != nil {
			t.Fatal(err)
		}
		if enabled {
			feasible, enableErr := g.EnableEdge(id)
			if enableErr != nil || !feasible {
				t.Fatal(feasible, enableErr)
			}
		}
		return id
	}
	firstLiteral := NewSMTLiteral(31, false)
	_ = add(0, 1, 0, firstLiteral, true)
	_ = add(1, 0, 0, NewSMTLiteral(32, false), true)
	_ = add(1, 2, 1, NewSMTLiteral(33, false), true)
	disabled := add(2, 3, 0, NewSMTLiteral(34, false), false)

	components, err := g.ComputeZeroEdgeSCC()
	if err != nil || components[0] < 0 || components[0] != components[1] || components[2] != -1 || components[3] != -1 {
		t.Fatal("zero SCC", components, err)
	}
	successors, err := g.ComputeZeroSuccessors(0)
	if err != nil || !reflect.DeepEqual(successors, []int32{0, 1}) {
		t.Fatal("zero successors", successors, err)
	}
	weight, explanation, found, err := g.testMinimumEnabledEdge(0, 1)
	if err != nil || !found || weight.Sign() != 0 || explanation != firstLiteral {
		t.Fatal("minimum edge", weight, explanation, found, err)
	}
	id, found, err := g.FindEdge(2, 3)
	if err != nil || !found || id != disabled {
		t.Fatal("disabled edge lookup", id, found, err)
	}
	neighbors, err := g.NeighborsUndirected(2)
	if err != nil || !reflect.DeepEqual(neighbors, []int32{3, 1}) {
		t.Fatal("undirected neighbor order", neighbors, err)
	}
	thread, err := g.DFSUndirectedThread(0)
	if err != nil || thread[0] != 1 || thread[1] != 2 || thread[2] != 3 || thread[3] != 0 {
		t.Fatal("DFS discovery thread", thread, err)
	}
	parents, depths, err := g.BFSUndirected(0)
	if err != nil || !reflect.DeepEqual(parents, []int32{-1, 0, 1, 2}) || !reflect.DeepEqual(depths, []int32{0, 1, 2, 3}) {
		t.Fatal("BFS", parents, depths, err)
	}
}

func TestSMTRationalDiffGraphAssignmentMutationTimestampAndReset(t *testing.T) {
	g := NewSMTRationalDiffGraph(testQ())
	if err := g.InitVariable(0); err != nil {
		t.Fatal(err)
	}
	if err := g.testSetAssignment(0, diffRat(4)); err != nil {
		t.Fatal(err)
	}
	if err := g.testIncrementAssignment(0, diffRat(-3)); err != nil {
		t.Fatal(err)
	}
	value, err := g.testAssignment(0)
	if err != nil || value.Cmp(diffRat(1)) != 0 {
		t.Fatal(value, err)
	}
	id, err := g.testAddEdge(0, 0, diffRat(0), SMTNullLiteral)
	if err != nil {
		t.Fatal(err)
	}
	if feasible, enableErr := g.EnableEdge(id); enableErr != nil || !feasible || g.Timestamp() != 1 {
		t.Fatal(feasible, g.Timestamp(), enableErr)
	}
	g.Reset()
	if g.NumNodes() != 0 || g.NumEdges() != 0 || g.Timestamp() != 0 || !g.IsFeasible() {
		t.Fatal("reset state")
	}
}

func TestSMTRationalDiffGraphShortestZeroPathStrictTimestamp(t *testing.T) {
	g := NewSMTRationalDiffGraph(testQ())
	for variable := int32(0); variable < 3; variable++ {
		if err := g.InitVariable(variable); err != nil {
			t.Fatal(err)
		}
	}
	firstLiteral := NewSMTLiteral(41, false)
	secondLiteral := NewSMTLiteral(42, true)
	for index, edge := range []struct {
		source, target int32
		literal        SMTLiteral
	}{{0, 1, firstLiteral}, {1, 2, secondLiteral}} {
		id, err := g.testAddEdge(edge.source, edge.target, diffRat(0), edge.literal)
		if err != nil {
			t.Fatal(err)
		}
		feasible, err := g.EnableEdge(id)
		if err != nil || !feasible || g.Timestamp() != uint32(index+1) {
			t.Fatal(feasible, err, g.Timestamp())
		}
	}
	explanation, found, err := g.FindShortestZeroEdgePath(0, 2, 2)
	if err != nil || !found || !reflect.DeepEqual(explanation, []SMTLiteral{secondLiteral, firstLiteral}) {
		t.Fatal(explanation, found, err)
	}
	explanation, found, err = g.FindShortestZeroEdgePath(0, 2, 1)
	if err != nil || found || explanation != nil {
		t.Fatal("strict timestamp", explanation, found, err)
	}
}
