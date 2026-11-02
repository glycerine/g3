package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestAPIFixedpointLastModelExpression(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			astTestMust(t, f.c.SetFixedpointEngine(backend))
			if md, err := f.fp.LastModel(); err == nil || md != nil {
				t.Fatal("unsolved model", md, err)
			}
			astTestMust(t, f.fp.AddRule(f.pAtom, "fact"))
			query := f.pin(f.m.MkAndArgs([]ast.ASTHandle{f.pAtom, f.qAtom}))
			status, err := f.fp.QueryExpression(query)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("query", status)
			}
			md, err := f.fp.LastModel()
			astTestMust(t, err)
			astTestMust(t, f.fp.Close())
			// Closed query ownership cannot invalidate the caller's model. The fact
			// must be true and the compound query false without model completion.
			p, err := md.Core().ConstantInterpretation(f.p)
			astTestMust(t, err)
			q, err := md.Core().ConstantInterpretation(f.q)
			astTestMust(t, err)
			if !f.m.IsTrue(p) || !f.m.IsFalse(q) {
				t.Fatal("model violates fact or query", p, q)
			}
			astTestMust(t, md.Close())
			if closed, err := f.fp.LastModel(); err == nil || closed != nil {
				t.Fatal("closed model accessor", closed, err)
			}
		})
	}
}
