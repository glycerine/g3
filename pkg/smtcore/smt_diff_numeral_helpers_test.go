package smtcore

import (
	"math/big"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// Test-only math/big views of the difference-logic numerals, on testQ().

// testInfInit makes a Go zero value a valid zero.
func testInfInit(v *smtInfIntRational) {
	if v.rational.Num.Cell == 0 && v.rational.Den.Cell == 0 && v.rational.Den.Val == 0 {
		v.rational = smtQZero()
	}
}

func (v *smtInfIntRational) Set(o *smtInfIntRational) *smtInfIntRational {
	testInfInit(v)
	if err := smtInfIntSet(testQ(), v, o); err != nil {
		panic(err)
	}
	return v
}

func (v *smtInfIntRational) Add(a, b *smtInfIntRational) *smtInfIntRational {
	testInfInit(v)
	if err := smtInfIntAdd(testQ(), a, b, v); err != nil {
		panic(err)
	}
	return v
}

func (v *smtInfIntRational) Sub(a, b *smtInfIntRational) *smtInfIntRational {
	testInfInit(v)
	if err := smtInfIntSub(testQ(), a, b, v); err != nil {
		panic(err)
	}
	return v
}

func (v *smtInfIntRational) Cmp(o *smtInfIntRational) int {
	testInfInit(v)
	c, err := smtInfIntCmp(testQ(), v, o)
	if err != nil {
		panic(err)
	}
	return c
}

func (v *smtInfIntRational) Sign() int { testInfInit(v); return v.sign() }

// diffInf is rational + infinitesimal*epsilon.
func diffInf(rational, infinitesimal int64) *smtInfIntRational {
	return &smtInfIntRational{rational: *testQR(big.NewRat(rational, 1)), infinitesimal: int(infinitesimal)}
}

// testInfText is inf_int_rational::to_string.
func testInfText(v *smtInfIntRational) string {
	s, err := smtInfIntText(testQ(), v)
	if err != nil {
		panic(err)
	}
	return s
}

func (g *SMTRationalDiffGraph) testAddEdge(source, target int32, weight *big.Rat, explanation SMTLiteral) (int32, error) {
	if weight == nil {
		return g.AddEdge(source, target, nil, explanation)
	}
	return g.AddEdge(source, target, testQR(weight), explanation)
}

func (g *SMTRationalDiffGraph) testAssignment(variable int32) (*big.Rat, error) {
	r := smtQZero()
	if err := g.Assignment(variable, &r); err != nil {
		return nil, err
	}
	return testBig(&r), nil
}

func (g *SMTRationalDiffGraph) testEdgeWeight(id int32) (*big.Rat, error) {
	r := smtQZero()
	if err := g.EdgeWeight(id, &r); err != nil {
		return nil, err
	}
	return testBig(&r), nil
}

func (g *SMTRationalDiffGraph) testMinimumEnabledEdge(source, target int32) (*big.Rat, SMTLiteral, bool, error) {
	r := smtQZero()
	explanation, found, err := g.MinimumEnabledEdge(source, target, &r)
	if err != nil || !found {
		return nil, explanation, found, err
	}
	return testBig(&r), explanation, true, nil
}

func (g *SMTRationalDiffGraph) testSetAssignment(variable int32, value *big.Rat) error {
	if value == nil {
		return g.SetAssignment(variable, nil)
	}
	return g.SetAssignment(variable, testQR(value))
}

func (g *SMTRationalDiffGraph) testIncrementAssignment(variable int32, value *big.Rat) error {
	if value == nil {
		return g.IncrementAssignment(variable, nil)
	}
	return g.IncrementAssignment(variable, testQR(value))
}

func (g *SMTInfRationalDiffGraph) testAssignment(variable int32) (*smtInfIntRational, error) {
	v := newSMTInfIntRational()
	if err := g.Assignment(variable, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// testQCmp compares a rational with a math/big value.
func testQCmp(r *numeral.Rational, b *big.Rat) int { return testBig(r).Cmp(b) }

// testInfRat is r + e*epsilon for a math/big r.
func testInfRat(r *big.Rat, e int64) *smtInfIntRational {
	return &smtInfIntRational{rational: *testQR(r), infinitesimal: int(e)}
}

// testQBigs is a math/big copy of a vector of rationals.
func testQBigs(values []numeral.Rational) []big.Rat {
	out := make([]big.Rat, len(values))
	for i := range values {
		out[i].Set(testBig(&values[i]))
	}
	return out
}

// testUTVPIGraph is newSMTUTVPIGraph on testQ().
func testUTVPIGraph() *smtUTVPIGraph {
	g := newSMTUTVPIGraph()
	g.graph.q = testQ()
	return g
}
