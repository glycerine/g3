package api

// Source: src/test/smt2print_parse.cpp.
import "testing"

func TestSMT2PrintParseSourceBenchmarks(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"parametric datatype", `(declare-datatypes (T) ((list (nil) (cons (car T) (cdr list)))))
(declare-const x Int)(declare-const l (list Int))(declare-fun f ((list Int)) Bool)(assert (f (cons x l)))`},
		{"arrays", `(declare-const x Int)(declare-const a (Array Int Int))(declare-const b (Array (Array Int Int) Bool))
(assert (select b a))(assert (= b ((as const (Array (Array Int Int) Bool)) true)))(assert (= b (store b a true)))
(declare-const b1 (Array Bool Bool))(declare-const b2 (Array Bool Bool))(assert (= ((as const (Array Bool Bool)) false) ((_ map and) b1 b2)))`},
		{"mutual datatypes", `(declare-datatypes () ((list (nil) (cons (car tree) (cdr list))) (tree (leaf) (node (n list)))))
(declare-const x tree)(declare-const l list)(declare-fun f (list) Bool)(assert (f (cons x l)))`},
		{"arithmetic", `(declare-const x Real)(declare-const y Int)(assert (= x 0.0))(assert (= y 6))(assert (> (/ x 1.4) (to_real y)))`},
		{"bitvectors", `(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))(assert (bvule x (bvmul y (concat ((_ extract 2 0) x) ((_ extract 3 3) #xf0)))))`},
		{"strings", `(assert (= "abc" "abc"))`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSMTAPIFixture(t, "ALL", nil)
			input, err := f.c.ParseSMT2String(tc.input, SMT2Bindings{})
			astTestMust(t, err)
			defer func() { astTestMust(t, f.m.DelRef(&input)) }()
			h, err := f.m.RefGet(input)
			astTestMust(t, err)
			benchmark, err := f.c.BenchmarkSMT2(h, APIBenchmarkConfig{Name: "test"})
			astTestMust(t, err)
			parsed, err := f.c.ParseSMT2String(benchmark, SMT2Bindings{})
			if err != nil {
				t.Fatalf("round-trip benchmark: %v\n%s", err, benchmark)
			}
			defer func() { astTestMust(t, f.m.DelRef(&parsed)) }()
			result, err := f.m.RefGet(parsed)
			astTestMust(t, err)
			_, err = f.c.BenchmarkSMT2(result, APIBenchmarkConfig{Name: "test"})
			astTestMust(t, err)
		})
	}
}
