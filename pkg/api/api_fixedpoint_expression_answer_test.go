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

func TestAPIFixedpointNonlinearBMCAnswersWithoutProofOption(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc"} {
		t.Run(backend, func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			astTestMust(t, f.c.SetFixedpointEngine(backend))
			for _, name := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
				astTestMust(t, f.c.SetBoolOption(name, false))
			}
			astTestMust(t, f.fp.AddFact(f.p, nil))
			astTestMust(t, f.fp.AddFact(f.q, nil))
			query := f.pin(f.m.MkAndArgs([]ast.ASTHandle{f.pAtom, f.qAtom}))
			rule := f.pin(f.m.MkImplies(query, f.uAtom))
			astTestMust(t, f.fp.AddRule(rule, "nonlinear"))
			status, err := f.fp.QueryExpression(query)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("expression query", status)
			}
			first, err := f.fp.LastAnswer()
			astTestMust(t, err)
			defer func() { astTestMust(t, f.m.DelRef(&first)) }()
			status, err = f.fp.Query(f.u)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("relation query", status)
			}
			second, err := f.fp.LastAnswer()
			astTestMust(t, err)
			defer func() { astTestMust(t, f.m.DelRef(&second)) }()
			for _, ref := range []ast.ASTRef{first, second} {
				proof, err := f.m.RefGet(ref)
				astTestMust(t, err)
				if _, err = f.m.GetProofFact(proof); err != nil {
					t.Fatal("answer is not an owned proof", err)
				}
			}
		})
	}
}

func TestAPIFixedpointSpacerExpressionGroundAnswerAndTrace(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.c.SetFixedpointEngine("spacer"))
	astTestMust(t, f.c.SetBoolOption("fixedpoint.xform.inline_eager", false))
	astTestMust(t, f.c.SetBoolOption("fixedpoint.xform.inline_linear", false))
	astTestMust(t, f.fp.AddRule(f.pAtom, "p-fact"))
	astTestMust(t, f.fp.AddRule(f.qAtom, "q-fact"))
	query := f.pin(f.m.MkAndArgs([]ast.ASTHandle{f.pAtom, f.qAtom}))
	status, err := f.fp.QueryExpression(query)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("query", status)
	}
	ground, err := f.fp.LastGroundSatAnswer()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&ground)) }()
	value, err := f.m.RefGet(ground)
	astTestMust(t, err)
	sort, err := f.m.GetSort(value)
	astTestMust(t, err)
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	if sort != boolean {
		t.Fatal("ground sort", sort)
	}
	trace, names, err := f.fp.RulesAlongTrace()
	astTestMust(t, err)
	defer releaseAPIRefs(t, f.m, trace)
	if len(trace) == 0 || len(trace) != len(names) {
		t.Fatal("trace", len(trace), names)
	}
	for i := range trace {
		h, e := f.m.RefGet(trace[i])
		astTestMust(t, e)
		if h.IsNull() {
			t.Fatal("empty trace formula")
		}
	}
	astTestMust(t, f.fp.AddFact(f.u, nil))
	if _, err = f.fp.LastGroundSatAnswer(); err == nil {
		t.Fatal("stale ground answer")
	}
	if _, _, err = f.fp.RulesAlongTrace(); err == nil {
		t.Fatal("stale trace")
	}
}

func TestAPIFixedpointSpacerExpressionQueryFromLevel(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.c.SetFixedpointEngine("spacer"))
	astTestMust(t, f.fp.AddFact(f.p, nil))
	astTestMust(t, f.fp.AddFact(f.q, nil))
	query := f.pin(f.m.MkAndArgs([]ast.ASTHandle{f.pAtom, f.qAtom}))
	status, err := f.fp.QueryExpressionFromLevel(query, 0)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("level query", status)
	}
	answer, err := f.fp.LastAnswer()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&answer)) }()
	h, err := f.m.RefGet(answer)
	astTestMust(t, err)
	if h.IsNull() {
		t.Fatal("empty answer")
	}
	ground, err := f.fp.LastGroundSatAnswer()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&ground)) }()
	h, err = f.m.RefGet(ground)
	astTestMust(t, err)
	if h.IsNull() {
		t.Fatal("empty ground answer")
	}
	status, err = f.fp.QueryExpressionFromLevel(query, 1)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("level one query", status)
	}
}
