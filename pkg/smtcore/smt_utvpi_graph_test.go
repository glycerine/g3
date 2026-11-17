package smtcore

import (
	"math/big"
	"testing"
)

func TestSMTUTVPIGraphEncodingWeightsAndCyclePremises(t *testing.T) {
	for _, negativeFirst := range []bool{false, true} {
		for _, negativeSecond := range []bool{false, true} {
			g := testUTVPIGraph()
			_, err := g.addVariable(false)
			astTestMust(t, err)
			_, err = g.addVariable(false)
			astTestMust(t, err)
			terms := []smtUTVPIGraphTerm{{0, negativeFirst}, {1, negativeSecond}}
			id, err := g.addInequality(terms, diffInf(3, 1), NewSMTLiteral(2, false))
			astTestMust(t, err)
			// These are the four explicit source branches, in source edge order.
			endpoints := [2][2]int32{{3, 0}, {1, 2}}
			if !negativeFirst && negativeSecond {
				endpoints = [2][2]int32{{2, 0}, {1, 3}}
			} else if negativeFirst && !negativeSecond {
				endpoints = [2][2]int32{{3, 1}, {0, 2}}
			} else if negativeFirst && negativeSecond {
				endpoints = [2][2]int32{{0, 3}, {2, 1}}
			}
			for i, pair := range endpoints {
				edge := &g.graph.edges[int(id)+i]
				if edge.source != pair[0] || edge.target != pair[1] || edge.weight.Cmp(diffInf(-3, -1)) != 0 || g.coefficients[int(id)+i] != 1 {
					t.Fatal("binary signed encoding", terms, i, edge)
				}
			}
		}
	}
	g := testUTVPIGraph()
	_, err := g.addVariable(false)
	astTestMust(t, err)
	upper, err := g.addInequality([]smtUTVPIGraphTerm{{0, false}}, diffInf(-1, 0), NewSMTLiteral(3, false))
	astTestMust(t, err)
	if g.graph.edges[upper].weight.Cmp(diffInf(2, 0)) != 0 || g.coefficients[upper] != 2 {
		t.Fatal("unary bound lost its doubled weight or explanation coefficient")
	}
	ok, err := g.enablePair(upper)
	astTestMust(t, err)
	if !ok {
		t.Fatal("upper bound refused")
	}
	astTestMust(t, g.graph.Push())
	lower, err := g.addInequality([]smtUTVPIGraphTerm{{0, true}}, diffInf(1, 1), NewSMTLiteral(4, false))
	astTestMust(t, err)
	ok, err = g.enablePair(lower)
	astTestMust(t, err)
	if ok {
		t.Fatal("x<=1 and x>1 accepted")
	}
	cycle, err := g.graph.NegativeCycleEdges()
	astTestMust(t, err)
	seen := map[int32]bool{}
	for _, id := range cycle {
		if g.coefficients[id] != 2 {
			t.Fatal("negative-cycle unary explanation coefficient")
		}
		seen[int32(g.graph.edges[id].explanation)] = true
	}
	if len(seen) != 2 {
		t.Fatal("conflict omitted one bound", cycle)
	}
	astTestMust(t, g.graph.Pop(1))
	reused, err := g.addInequality([]smtUTVPIGraphTerm{{0, true}}, diffInf(0, 0), NewSMTLiteral(5, false))
	astTestMust(t, err)
	if reused != lower || len(g.coefficients) != len(g.graph.edges) {
		t.Fatal("popped edge coefficients did not track reused IDs")
	}
	ok, err = g.enablePair(reused)
	astTestMust(t, err)
	if !ok {
		t.Fatal("popped strict bound remained active")
	}
}

func TestSMTUTVPIGraphIntegerParityConflictAndRealRelaxation(t *testing.T) {
	for _, integer := range []bool{false, true} {
		g := testUTVPIGraph()
		for i := 0; i < 2; i++ {
			_, err := g.addVariable(integer)
			astTestMust(t, err)
		}
		// x+y=1 and x-y=0 have the unique rational solution x=y=1/2.
		for i, constraint := range []struct {
			terms  []smtUTVPIGraphTerm
			weight int64
		}{
			{[]smtUTVPIGraphTerm{{0, false}, {1, false}}, -1},
			{[]smtUTVPIGraphTerm{{0, true}, {1, true}}, 1},
			{[]smtUTVPIGraphTerm{{0, false}, {1, true}}, 0},
			{[]smtUTVPIGraphTerm{{0, true}, {1, false}}, 0},
		} {
			id, err := g.addInequality(constraint.terms, diffInf(constraint.weight, 0), NewSMTLiteral(int32(i+2), false))
			astTestMust(t, err)
			ok, err := g.enablePair(id)
			astTestMust(t, err)
			if !ok {
				t.Fatal("rational relaxation incorrectly infeasible")
			}
		}
		cycle, err := g.parityConflict()
		astTestMust(t, err)
		if integer != (len(cycle) > 0) {
			t.Fatal("parity conflict ignored variable sort", integer, cycle)
		}
		if integer {
			var weight smtInfIntRational
			seen := map[SMTLiteral]bool{}
			for _, id := range cycle {
				weight.Add(&weight, &g.graph.edges[id].weight)
				seen[g.graph.edges[id].explanation] = true
			}
			if weight.Sign() != 0 || len(seen) != 4 {
				t.Fatal("parity witness did not cover the tight odd cycle", cycle)
			}
		}
	}
}

func TestSMTUTVPIGraphBoundedIntegerEnumerationAndParityRepair(t *testing.T) {
	// Enumerate two octagonal constraints under -2<=x,y<=2. Independent
	// integer enumeration checks both the rational solver and parity algorithm;
	// every accepted graph must also yield an actual satisfying integer model.
	for a := 0; a < 28; a++ {
		for b := 0; b < 28; b++ {
			g := testUTVPIGraph()
			for i := 0; i < 2; i++ {
				_, err := g.addVariable(true)
				astTestMust(t, err)
			}
			type inequality struct{ x, y, weight int64 }
			constraints := []inequality{{1, 0, -2}, {-1, 0, -2}, {0, 1, -2}, {0, -1, -2}}
			for _, encoded := range []int{a, b} {
				x, y := int64(1), int64(1)
				if encoded/7&1 != 0 {
					x = -1
				}
				if encoded/7&2 != 0 {
					y = -1
				}
				constraints = append(constraints, inequality{x, y, int64(encoded%7) - 3})
			}
			feasible := true
			for i, c := range constraints {
				terms := []smtUTVPIGraphTerm{}
				if c.x != 0 {
					terms = append(terms, smtUTVPIGraphTerm{0, c.x < 0})
				}
				if c.y != 0 {
					terms = append(terms, smtUTVPIGraphTerm{1, c.y < 0})
				}
				id, err := g.addInequality(terms, diffInf(c.weight, 0), NewSMTLiteral(int32(i+2), false))
				astTestMust(t, err)
				ok, err := g.enablePair(id)
				astTestMust(t, err)
				if !ok {
					feasible = false
					break
				}
			}
			if feasible {
				conflict, err := g.parityConflict()
				astTestMust(t, err)
				feasible = len(conflict) == 0
			}
			enumerated := false
			for x := int64(-2); x <= 2; x++ {
				for y := int64(-2); y <= 2; y++ {
					ok := true
					for _, c := range constraints {
						ok = ok && c.x*x+c.y*y+c.weight <= 0
					}
					enumerated = enumerated || ok
				}
			}
			if feasible != enumerated {
				t.Fatalf("constraints %v: graph %v, enumeration %v", constraints, feasible, enumerated)
			}
			if !feasible {
				continue
			}
			astTestMust(t, g.enforceParity())
			values := [2]big.Rat{}
			for v := range values {
				values[v].Sub(testBig(&g.graph.assignment[2*v].rational), testBig(&g.graph.assignment[2*v+1].rational))
				values[v].Quo(&values[v], big.NewRat(2, 1))
				if !values[v].IsInt() {
					t.Fatal("parity repair produced fractional model", &values[v])
				}
			}
			for _, c := range constraints {
				value := new(big.Rat).Mul(&values[0], big.NewRat(c.x, 1))
				value.Add(value, new(big.Rat).Mul(&values[1], big.NewRat(c.y, 1)))
				value.Add(value, big.NewRat(c.weight, 1))
				if value.Sign() > 0 {
					t.Fatal("repaired integer model violates input", constraints, values)
				}
			}
		}
	}
}

func TestSMTUTVPIGraphStrictMixedModelsAndScopedVariableReuse(t *testing.T) {
	g := testUTVPIGraph()
	zero, err := g.addVariable(true)
	astTestMust(t, err)
	integer, err := g.addVariable(true)
	astTestMust(t, err)
	real, err := g.addVariable(false)
	astTestMust(t, err)
	bound := func(variable int32, negative bool, weight *smtInfIntRational) {
		id, err := g.addInequality([]smtUTVPIGraphTerm{{variable, negative}}, weight, NewSMTLiteral(int32(len(g.graph.edges)+2), false))
		astTestMust(t, err)
		ok, err := g.enablePair(id)
		astTestMust(t, err)
		if !ok {
			t.Fatal("consistent model constraint rejected")
		}
	}
	bound(integer, false, diffInf(-3, 0))
	bound(integer, true, diffInf(3, 0))
	// 0<r<1/3. Infinitesimal substitution must respect the rational slack;
	// substituting a fixed epsilon such as 1 would violate this interval.
	bound(real, true, diffInf(0, 1))
	bound(real, false, testInfRat(big.NewRat(-1, 3), 1))
	check := func(values []big.Rat, count int) {
		if len(values) != count || values[zero].Sign() != 0 || values[integer].Cmp(big.NewRat(3, 1)) != 0 || values[real].Sign() <= 0 || values[real].Cmp(big.NewRat(1, 3)) >= 0 {
			t.Fatal("typed mixed model violated a source constraint", values)
		}
	}
	values, err := g.modelValues(zero)
	astTestMust(t, err)
	check(testQBigs(values), 3)
	astTestMust(t, g.push())
	child, err := g.addVariable(true)
	astTestMust(t, err)
	bound(child, false, diffInf(-7, 0))
	bound(child, true, diffInf(7, 0))
	childValues, err := g.modelValues(zero)
	astTestMust(t, err)
	check(testQBigs(childValues), 4)
	if testQCmp(&childValues[child], big.NewRat(7, 1)) != 0 {
		t.Fatal("child integer value", &childValues[child])
	}
	astTestMust(t, g.pop(1))
	values, err = g.modelValues(zero)
	astTestMust(t, err)
	check(testQBigs(values), 3)
	astTestMust(t, g.push())
	reused, err := g.addVariable(true)
	astTestMust(t, err)
	if reused != child {
		t.Fatal("source variable numbering was not restored by pop")
	}
	bound(reused, false, diffInf(4, 0))
	bound(reused, true, diffInf(-4, 0))
	values, err = g.modelValues(zero)
	astTestMust(t, err)
	check(testQBigs(values), 4)
	if testQCmp(&values[reused], big.NewRat(-4, 1)) != 0 {
		t.Fatal("reused variable retained popped model state", &values[reused])
	}
	astTestMust(t, g.pop(1))
	if err := g.pop(1); err == nil {
		t.Fatal("invalid scope pop accepted")
	}
	if _, err := g.modelValues(-1); err == nil {
		t.Fatal("invalid zero variable accepted")
	}
}

func TestSMTUTVPIGraphRealParityPredicateAndFractionalIntegerCandidate(t *testing.T) {
	g := testUTVPIGraph()
	zero, err := g.addVariable(true)
	astTestMust(t, err)
	real, err := g.addVariable(false)
	astTestMust(t, err)
	first := testInfRat(big.NewRat(1, 2), 0)
	second := testInfRat(big.NewRat(3, 2), 0)
	astTestMust(t, g.graph.SetAssignment(2*real, first))
	astTestMust(t, g.graph.SetAssignment(2*real+1, second))
	ok, err := g.parityOK(real)
	astTestMust(t, err)
	if !ok {
		t.Fatal("source is_even must be false for both nonintegral rational parts")
	}
	first.rational = *testQR(big.NewRat(2, 1))
	astTestMust(t, g.graph.SetAssignment(2*real, first))
	ok, err = g.parityOK(real)
	astTestMust(t, err)
	if ok {
		t.Fatal("even integer and nonintegral rational incorrectly have the same parity predicate")
	}
	integer, err := g.addVariable(true)
	astTestMust(t, err)
	astTestMust(t, g.graph.SetAssignment(2*integer+1, diffInf(0, -1)))
	ok, err = g.parityOK(integer)
	astTestMust(t, err)
	if !ok {
		t.Fatal("rational-part parity predicate must ignore epsilon")
	}
	values, err := g.modelValues(zero)
	astTestMust(t, err)
	if testQCmp(&values[integer], big.NewRat(1, 2)) != 0 {
		t.Fatal("source fractional Int candidate was rounded before the arithmetic factory", &values[integer])
	}
}
