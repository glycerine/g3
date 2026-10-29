package base_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"testing"
)

func TestEngineBaseModelProofNoopsAndLifecycle(t *testing.T) {
	f := newRuleSetFixture(t)
	truth, e := f.manager.MkTrue()
	ruleMust(t, e)
	ref, e := f.manager.NewRef(truth)
	ruleMust(t, e)
	defer f.manager.DelRef(&ref)
	core := &engineFake{manager: f.manager, answer: ref}
	engine, e := base.NewEngineBase(f.manager, "life", core)
	ruleMust(t, e)
	model, e := engine.Model()
	ruleMust(t, e)
	ruleMust(t, model.Close())
	proofs, e := f.manager.ProofsEnabled()
	ruleMust(t, e)
	if proofs {
		p, e := engine.Proof()
		ruleMust(t, e)
		ruleMust(t, f.manager.DelRef(&p))
	}
	ruleMust(t, engine.ResetStatistics())
	ruleMust(t, engine.Cancel())
	ruleMust(t, engine.Cleanup())
	ruleMust(t, engine.UpdateParameters())
	ruleMust(t, engine.Close())
	ruleMust(t, engine.Close())
	if _, e = engine.Query(ast.ASTHandle{}); e == nil {
		t.Fatal("closed engine query")
	}
}
