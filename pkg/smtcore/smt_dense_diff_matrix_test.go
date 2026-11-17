package smtcore

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtDenseTestValue is a dense numeral as math/big parts (test reference).
type smtDenseTestValue struct{ main, epsilon big.Rat }

func (v *smtDenseTestValue) add(a, b *smtDenseTestValue) *smtDenseTestValue {
	v.main.Add(&a.main, &b.main)
	v.epsilon.Add(&a.epsilon, &b.epsilon)
	return v
}

func (v *smtDenseTestValue) cmp(o *smtDenseTestValue) int {
	if c := v.main.Cmp(&o.main); c != 0 {
		return c
	}
	return v.epsilon.Cmp(&o.epsilon)
}

func smtDenseTestParts(t *testing.T, q *numeral.RationalManager, parts func(n, k *numeral.Rational) error) smtDenseTestValue {
	t.Helper()
	n, k := smtQZero(), smtQZero()
	astTestMust(t, parts(&n, &k))
	var v smtDenseTestValue
	astTestMust(t, q.BigRatOf(&n, &v.main))
	astTestMust(t, q.BigRatOf(&k, &v.epsilon))
	astTestMust(t, smtQDel(q, &n))
	astTestMust(t, smtQDel(q, &k))
	return v
}

// smtDenseTestDistance reads a cell distance of any instantiation.
func smtDenseTestDistance(t *testing.T, ext smtDenseExt, source, target int) smtDenseTestValue {
	t.Helper()
	var v smtDenseTestValue
	switch x := ext.(type) {
	case *smtDenseExtSI:
		v.main.SetInt64(int64(x.dist[source][target].val))
	case *smtDenseExtSMI:
		d := x.dist[source][target]
		v.main.SetInt64(int64(d.first))
		v.epsilon.SetInt64(int64(d.second))
	case *smtDenseExtI:
		astTestMust(t, testQ().BigRatOf(&x.dist[source][target], &v.main))
	case *smtDenseExtMI:
		astTestMust(t, testQ().BigRatOf(&x.dist[source][target].first, &v.main))
		astTestMust(t, testQ().BigRatOf(&x.dist[source][target].second, &v.epsilon))
	default:
		t.Fatal("unknown dense instantiation")
	}
	return v
}

// smtDenseTestSetK sets the Ext's k to main + epsilon*e through the theory
// operations (setK, negK and subKEpsilon with the real epsilon).
func smtDenseTestSetK(t *testing.T, ext smtDenseExt, main *big.Rat, e int) {
	t.Helper()
	if e > 0 {
		astTestMust(t, ext.setK(testQR(new(big.Rat).Neg(main))))
		for i := 0; i < e; i++ {
			astTestMust(t, ext.subKEpsilon(true))
		}
		astTestMust(t, ext.negK())
		return
	}
	astTestMust(t, ext.setK(testQR(main)))
	for i := 0; i < -e; i++ {
		astTestMust(t, ext.subKEpsilon(true))
	}
}

// Bellman-Ford recomputed from asserted edges is independent of the source
// dense incremental closure and checks both finite and disconnected cells.
func smtCheckDenseClosure(t *testing.T, g *smtDenseDiffMatrix) {
	t.Helper()
	offsets := make([]smtDenseTestValue, len(g.edges))
	for i := 1; i < len(g.edges); i++ {
		offsets[i] = smtDenseTestParts(t, testQ(), func(n, k *numeral.Rational) error { return g.ext.edgeParts(i, n, k) })
	}
	for source := range g.rows {
		dist := make([]*smtDenseTestValue, len(g.rows))
		dist[source] = &smtDenseTestValue{}
		for pass := 1; pass < len(g.rows); pass++ {
			for i, e := range g.edges[1:] {
				if dist[e.source] == nil {
					continue
				}
				value := new(smtDenseTestValue).add(dist[e.source], &offsets[i+1])
				if dist[e.target] == nil || value.cmp(dist[e.target]) < 0 {
					dist[e.target] = value
				}
			}
		}
		for target := range g.rows {
			connected := g.connected(source, target)
			if connected != (dist[target] != nil) {
				t.Fatal("incremental closure connectivity differs from Bellman-Ford", source, target)
			}
			if !connected {
				continue
			}
			value := smtDenseTestDistance(t, g.ext, source, target)
			if value.cmp(dist[target]) != 0 {
				t.Fatal("incremental closure differs from Bellman-Ford", source, target)
			}
			antecedents, err := g.antecedents(source, target)
			astTestMust(t, err)
			var sum smtDenseTestValue
			for _, literal := range antecedents {
				found := false
				for i, edge := range g.edges[1:] {
					if edge.literal == literal {
						sum.add(&sum, &offsets[i+1])
						found = true
						break
					}
				}
				if !found {
					t.Fatal("distance explanation contains an unasserted literal", literal)
				}
			}
			if sum.cmp(&value) != 0 {
				t.Fatal("distance explanation has the wrong total weight", source, target)
			}
		}
	}
	astTestMust(t, g.ext.initModel(g))
	for i, edge := range g.edges[1:] {
		x := smtDenseTestParts(t, testQ(), func(n, k *numeral.Rational) error { return g.ext.assignmentParts(edge.target, n, k) })
		y := smtDenseTestParts(t, testQ(), func(n, k *numeral.Rational) error { return g.ext.assignmentParts(edge.source, n, k) })
		var limit smtDenseTestValue
		limit.add(&y, &offsets[i+1])
		if x.cmp(&limit) > 0 {
			t.Fatal("dense model violates an asserted edge", edge.source, edge.target)
		}
	}
}

func TestSMTDenseDifferenceClosureExplanationsAndRollback(t *testing.T) {
	for _, kind := range []smtDenseExtKind{smtDenseMI, smtDenseI, smtDenseSMI, smtDenseSI} {
		infinitesimal := kind == smtDenseMI || kind == smtDenseSMI
		rational := kind == smtDenseMI || kind == smtDenseI
		for order := 0; order < 7; order++ {
			t.Run(fmt.Sprint(kind, "/", order), func(t *testing.T) {
				ext := newSMTDenseExt(kind, testQ())
				defer func() { astTestMust(t, ext.close()) }()
				g := newSMTDenseDiffMatrix(ext)
				for i := 0; i < 6; i++ {
					g.addVariable()
				}
				g.push()
				// Nonnegative reduced weights provide consistent inputs with
				// negative original edge weights.
				for i := 0; i < 30; i++ {
					index := (i*7 + order*11) % 30
					source := index / 5
					target := index % 5
					if target >= source {
						target++
					}
					main := big.NewRat(int64((target-source)*7+index%4), 1)
					if rational {
						main.Quo(main, big.NewRat(3, 1))
					}
					e := 0
					if infinitesimal {
						e = index % 3
					}
					smtDenseTestSetK(t, ext, main, e)
					ok, _, err := g.addEdge(source, target, SMTLiteral(index+2))
					astTestMust(t, err)
					if !ok {
						t.Fatal("consistent edge rejected", index)
					}
					smtCheckDenseClosure(t, g)
				}
				if !g.connected(1, 0) {
					t.Fatal("test graph should be connected")
				}
				// k = -d(1, 0) - epsilon closes a negative cycle.
				reverse := smtDenseTestDistance(t, ext, 1, 0)
				reverse.main.Neg(&reverse.main)
				if infinitesimal {
					smtDenseTestSetK(t, ext, &reverse.main, -int(reverse.epsilon.Num().Int64())-1)
				} else {
					smtDenseTestSetK(t, ext, &reverse.main, 0)
					astTestMust(t, ext.subKEpsilon(false))
				}
				edges, trail := len(g.edges), len(g.trail)
				ok, reason, err := g.addEdge(0, 1, SMTLiteral(100))
				astTestMust(t, err)
				if ok || len(reason) < 2 || reason[len(reason)-1] != SMTLiteral(100) || len(g.edges) != edges || len(g.trail) != trail {
					t.Fatal("negative cycle was not rejected atomically", reason)
				}
				g.push()
				v := g.addVariable()
				astTestMust(t, ext.setK(testQR(big.NewRat(0, 1))))
				ok, _, err = g.addEdge(v, 0, SMTLiteral(101))
				astTestMust(t, err)
				if !ok {
					t.Fatal("new-variable edge rejected")
				}
				smtCheckDenseClosure(t, g)
				astTestMust(t, g.pop(1))
				smtCheckDenseClosure(t, g)
				astTestMust(t, g.pop(1))
				smtCheckDenseClosure(t, g)
				if len(g.edges) != 1 || len(g.trail) != 0 || len(g.rows) != 6 {
					t.Fatal("scope pop retained child closure state")
				}
			})
		}
	}
}

// s_integer and inf_s_integer arithmetic panics where z3's int arithmetic
// would overflow (undefined behavior), and the rational constructor panics
// outside the int range.
func TestSMTSIntegerOverflowPanics(t *testing.T) {
	mustPanic := func(name string, f func()) {
		t.Helper()
		defer func() {
			r := recover()
			if r == nil || !strings.Contains(fmt.Sprint(r), "undefined behavior") && !strings.Contains(fmt.Sprint(r), "no defined value") {
				t.Fatalf("%s: panic %v", name, r)
			}
		}()
		f()
	}
	mustPanic("add", func() { smtSInteger{math.MaxInt32}.add(smtSInteger{1}) })
	mustPanic("sub", func() { smtSInteger{math.MinInt32}.sub(smtSInteger{1}) })
	mustPanic("neg", func() { smtSInteger{math.MinInt32}.neg() })
	mustPanic("inf add", func() { smtInfSInteger{0, math.MaxInt32}.add(smtInfSInteger{0, 1}) })
	mustPanic("inf neg", func() { smtInfSInteger{math.MinInt32, 0}.neg() })
	mustPanic("rational", func() { smtSIntegerOfRational(testQ(), testQR(big.NewRat(1<<31, 1))) })
	mustPanic("fraction", func() { smtSIntegerOfRational(testQ(), testQR(big.NewRat(1, 2))) })
	if v := smtSIntegerOfRational(testQ(), testQR(big.NewRat(math.MinInt32, 1))); v != math.MinInt32 {
		t.Fatal("INT_MIN", v)
	}
	if got := (smtInfSInteger{3, -1}).String(); got != "(3 -e*-1)" {
		t.Fatal("inf_s_integer operator<<", got)
	}
	if got := (smtInfSInteger{3, -1}).widthText(10); got != "(         3 -e*-1)" {
		t.Fatalf("inf_s_integer width %q", got)
	}
	if got := (smtInfSInteger{-3, 0}).widthText(10); got != "-3        " {
		t.Fatalf("inf_s_integer width %q", got)
	}
}
