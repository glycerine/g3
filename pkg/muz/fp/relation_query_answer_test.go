package fp_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogRelationsAnswerConjoinsRequestedRelations(t *testing.T) {
	for _, tc := range []struct {
		name        string
		p, q, empty bool
		status      lbool.LBool
		answer      bool
	}{
		{"both empty", false, false, false, lbool.False, false},
		{"one nonempty", true, false, false, lbool.True, false},
		{"both nonempty", true, true, false, lbool.True, true},
		{"no requested relations", false, false, true, lbool.True, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newQueryExpressionFixture(t)
			p, _ := f.predicate("multi-p")
			q, _ := f.predicate("multi-q")
			if tc.p {
				queryMust(t, f.engine.AddFact(p, nil))
			}
			if tc.q {
				queryMust(t, f.engine.AddFact(q, nil))
			}
			var rels []ast.ASTHandle
			if !tc.empty {
				rels = []ast.ASTHandle{p, q}
			}
			status, err := f.engine.QueryPredicates(rels)
			queryMust(t, err)
			if status != tc.status {
				t.Fatal("status", status)
			}
			answer, err := f.engine.RelationsAnswer()
			queryMust(t, err)
			defer func() { queryMust(t, f.manager.DelRef(&answer)) }()
			h, err := f.manager.RefGet(answer)
			queryMust(t, err)
			var want ast.ASTHandle
			if tc.empty || tc.status == lbool.False {
				if tc.answer {
					want = f.pin(f.manager.MkTrue())
				} else {
					want = f.pin(f.manager.MkFalse())
				}
			} else {
				yes := f.pin(f.manager.MkTrue())
				no := f.pin(f.manager.MkFalse())
				pAnswer, qAnswer := no, no
				if tc.p {
					pAnswer = yes
				}
				if tc.q {
					qAnswer = yes
				}
				want = f.pin(f.manager.MkAndArray([]ast.ASTHandle{pAnswer, qAnswer}))
			}
			if h != want {
				t.Fatal("answer", h, "want", want)
			}
			_, err = f.engine.QueryPredicate(p)
			queryMust(t, err)
			if _, err = f.engine.RelationsAnswer(); !errors.Is(err, fp.ErrPropositionalEngine) {
				t.Fatal("stale relations answer", err)
			}
		})
	}
}

func TestDatalogRelationsAnswerPreservesTupleFormulas(t *testing.T) {
	f := newQueryExpressionFixture(t)
	sort := f.pin(f.manager.MkBoolSort())
	p, _ := f.predicate("tuple-p", sort)
	q, _ := f.predicate("tuple-q", sort)
	yes := f.pin(f.manager.MkTrue())
	no := f.pin(f.manager.MkFalse())
	queryMust(t, f.engine.AddFact(p, []ast.ASTHandle{yes}))
	queryMust(t, f.engine.AddFact(q, []ast.ASTHandle{no}))
	status, err := f.engine.QueryPredicates([]ast.ASTHandle{p, q})
	queryMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	answer, err := f.engine.RelationsAnswer()
	queryMust(t, err)
	defer func() { queryMust(t, f.manager.DelRef(&answer)) }()
	h, err := f.manager.RefGet(answer)
	queryMust(t, err)
	v := f.pin(f.manager.MkVar(0, sort))
	a := f.pin(f.manager.MkEq(v, yes))
	b := f.pin(f.manager.MkEq(v, no))
	want := f.pin(f.manager.MkAndArray([]ast.ASTHandle{a, b}))
	if h != want {
		t.Fatal("conjoined answer", h, want)
	}
}
