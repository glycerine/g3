package api

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGuardedUniversalEquivalence(t *testing.T) {
	const declarations = "(declare-const k (_ BitVec 1))(declare-const b Bool)(declare-fun p ((_ BitVec 1)) Bool)(declare-fun f ((_ BitVec 1)) (_ BitVec 1))"
	for _, tc := range []struct {
		name, formula string
		changed       bool
	}{
		{"direct", "(forall ((x (_ BitVec 1))) (=> (= x k) (p x)))", true},
		{"nested", "(forall ((x (_ BitVec 1))) (=> b (=> (and (p x) (= k x)) (p (f x)))))", true},
		{"negative-position", "(not (forall ((x (_ BitVec 1))) (=> (= k x) (p x))))", true},
		{"compound-ground", "(forall ((x (_ BitVec 1))) (=> (= (f k) x) (p x)))", true},
		{"outer-index", "(forall ((y (_ BitVec 1))) (or (p y) (forall ((x (_ BitVec 1))) (=> (= x k) (= x (f y))))))", true},
		{"shadowed", "(forall ((x (_ BitVec 1))) (or (p x) (forall ((x (_ BitVec 1))) (=> (= x k) (p x)))))", true},
		{"outer-only", "(forall ((y (_ BitVec 1))) (or (p y) (forall ((x (_ BitVec 1))) (=> (= y k) (p x)))))", false},
		{"disjunction", "(forall ((x (_ BitVec 1))) (=> (or b (= x k)) (p x)))", false},
		{"negation", "(forall ((x (_ BitVec 1))) (=> (not (= x k)) (p x)))", false},
		{"conclusion", "(forall ((x (_ BitVec 1))) (=> (p x) (= x k)))", false},
		{"cyclic", "(forall ((x (_ BitVec 1))) (=> (= x (f x)) (p x)))", false},
		{"extra-binder", "(forall ((x (_ BitVec 1)) (y (_ BitVec 1))) (=> (= x k) (p y)))", false},
		{"pattern", "(forall ((x (_ BitVec 1))) (! (=> (= x k) (p x)) :pattern ((p x))))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewSMTContext()
			astTestMust(t, err)
			defer func() { astTestMust(t, c.Close()) }()
			m, err := c.Manager()
			astTestMust(t, err)
			input, err := c.ParseSMT2String(declarations+"(assert "+tc.formula+")", SMT2Bindings{})
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&input)) }()
			source, err := m.RefGet(input)
			astTestMust(t, err)
			result, err := c.SimplifyVerificationCondition(source)
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&result)) }()
			reduced, err := m.RefGet(result)
			astTestMust(t, err)
			want := tc.changed && !ast.ASTSourceDebug && !diagnostic.TraceCompiled()
			if (source != reduced) != want {
				t.Fatalf("changed = %t, want %t", source != reduced, want)
			}
			diff, err := m.MkXor(source, reduced)
			astTestMust(t, err)
			astTestMust(t, c.Assert(diff))
			answer, err := c.CheckSat()
			astTestMust(t, err)
			if answer != lbool.False {
				t.Fatalf("source and rewritten formula can differ: %v", answer)
			}
		})
	}
}

func TestGuardedUniversalFiniteTables(t *testing.T) {
	for key := 0; key < 2; key++ {
		for p := 0; p < 4; p++ {
			for q := 0; q < 4; q++ {
				for _, gate := range []bool{false, true} {
					t.Run(fmt.Sprintf("%d-%d-%d-%t", key, p, q, gate), func(t *testing.T) {
						const input = "(define-fun k () (_ BitVec 1) #b%d)" +
							"(define-fun p ((x (_ BitVec 1))) Bool (ite (= x #b0) %t %t))" +
							"(define-fun q ((x (_ BitVec 1))) Bool (ite (= x #b0) %t %t))" +
							"(assert (forall ((x (_ BitVec 1))) (=> %t (=> (and (= x k) (p x)) (q x)))))"
						c, err := NewSMTContext()
						astTestMust(t, err)
						defer func() { astTestMust(t, c.Close()) }()
						m, err := c.Manager()
						astTestMust(t, err)
						in, err := c.ParseSMT2String(fmt.Sprintf(input, key, p&1 != 0, p&2 != 0, q&1 != 0, q&2 != 0, gate), SMT2Bindings{})
						astTestMust(t, err)
						h, err := m.RefGet(in)
						astTestMust(t, err)
						result, err := c.SimplifyVerificationCondition(h)
						astTestMust(t, err)
						astTestMust(t, m.DelRef(&in))
						h, err = m.RefGet(result)
						astTestMust(t, err)
						astTestMust(t, c.Assert(h))
						astTestMust(t, m.DelRef(&result))
						got, err := c.CheckSat()
						astTestMust(t, err)
						want := lbool.True
						if gate && p&(1<<key) != 0 && q&(1<<key) == 0 {
							want = lbool.False
						}
						if got != want {
							t.Fatalf("result %v, truth table gives %v", got, want)
						}
					})
				}
			}
		}
	}
}

// Each operation runs a fresh incremental check on the captured Hermes base
// and the exact nested goal presented by GoIvy. Include reduction time.
func BenchmarkHermesGuardedUniversal(b *testing.B) {
	for _, batch := range []string{"27", "29"} {
		b.Run(batch, func(b *testing.B) {
			c, forms := loadVerificationGoals(b, "test_vectors/hermes_guarded_"+batch+".smt2.gz", 1)
			m, err := c.Manager()
			if err != nil {
				b.Fatal(err)
			}
			for _, mode := range []string{"Original", "Simplified"} {
				b.Run(mode, func(b *testing.B) {
					for b.Loop() {
						goal := forms[1]
						var held ast.ASTRef
						if mode == "Simplified" {
							var err error
							held, err = c.SimplifyVerificationCondition(goal)
							if err != nil {
								b.Fatal(err)
							}
							goal, err = m.RefGet(held)
							if err != nil {
								b.Fatal(err)
							}
						}
						_, err := checkVerificationSiblings(c, []ast.ASTHandle{forms[0], goal}, false)
						if mode == "Simplified" {
							if release := m.DelRef(&held); release != nil {
								b.Fatal(release)
							}
						}
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
