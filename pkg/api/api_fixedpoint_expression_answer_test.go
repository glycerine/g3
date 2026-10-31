package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointExpressionAnswerBackends(t *testing.T) {
	for _, backend := range []string{"datalog", "pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			astTestMust(t, f.c.SetFixedpointEngine(backend))
			if _, err := f.fp.LastAnswer(); err == nil {
				t.Fatal("answer before query")
			}
			astTestMust(t, f.fp.AddFact(f.p, nil))
			astTestMust(t, f.fp.AddFact(f.q, nil))
			query := f.pin(f.m.MkAndArgs([]ast.ASTHandle{f.pAtom, f.qAtom}))
			result, err := f.fp.QueryExpression(query)
			astTestMust(t, err)
			if result != lbool.True {
				t.Fatal("query", result)
			}
			ref, err := f.fp.LastAnswer()
			astTestMust(t, err)
			defer func() { astTestMust(t, f.m.DelRef(&ref)) }()
			answer, err := f.m.RefGet(ref)
			astTestMust(t, err)
			sort, err := f.m.GetSort(answer)
			astTestMust(t, err)
			boolean, err := f.m.MkBoolSort()
			astTestMust(t, err)
			if sort != boolean {
				t.Fatal("answer sort", sort)
			}
			if backend == "datalog" && !f.m.IsTrue(answer) {
				t.Fatal("datalog answer", answer)
			}
			astTestMust(t, f.fp.AddFact(f.u, nil))
			if _, err = f.fp.LastAnswer(); err == nil {
				t.Fatal("stale answer")
			}
		})
	}
}

func TestAPIFixedpointExpressionAnswerUnreachableBackends(t *testing.T) {
	for _, backend := range []string{"datalog", "pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			astTestMust(t, f.c.SetFixedpointEngine(backend))
			astTestMust(t, f.fp.AddFact(f.p, nil))
			query := f.pin(f.m.MkAndArgs([]ast.ASTHandle{f.pAtom, f.qAtom}))
			status, err := f.fp.QueryExpression(query)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("query", status)
			}
			ref, err := f.fp.LastAnswer()
			astTestMust(t, err)
			defer func() { astTestMust(t, f.m.DelRef(&ref)) }()
			answer, err := f.m.RefGet(ref)
			astTestMust(t, err)
			sort, err := f.m.GetSort(answer)
			astTestMust(t, err)
			boolean, err := f.m.MkBoolSort()
			astTestMust(t, err)
			if sort != boolean {
				t.Fatal("answer sort", sort)
			}
			if backend == "datalog" && !f.m.IsFalse(answer) {
				t.Fatal("datalog answer", answer)
			}
		})
	}
}
