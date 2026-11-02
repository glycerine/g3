package duality_test

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityFailedQueryInvalidatesBorrowedCounterexample(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(owner.Close()) }()
	m, err := owner.Manager()
	must(err)
	config, err := owner.SolverTranslationConfig()
	must(err)
	backend, err := duality.NewBackend(m, config, true)
	must(err)
	defer func() { must(backend.Close()) }()

	var refs []ast.ASTRef
	defer func() {
		for i := range refs {
			must(m.DelRef(&refs[i]))
		}
	}()
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		ref, err := m.NewRef(h)
		must(err)
		refs = append(refs, ref)
		return h
	}
	boolean := pin(m.MkBoolSort())
	name, err := m.Symbols.Intern("counterexample-invalidation-P")
	must(err)
	predicate := pin(m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{}))
	fact := pin(m.MkApp(predicate, nil))
	prepared, err := duality.PrepareQuery(m, []ast.ASTHandle{fact}, []uint32{math.MaxUint32}, fact, []ast.ASTHandle{predicate}, math.MaxUint32)
	must(err)
	defer func() { must(prepared.Close()) }()

	query := func() {
		t.Helper()
		status, err := backend.Query(prepared)
		must(err)
		if status != lbool.True {
			t.Fatal("reachable fixture", status)
		}
	}
	query()
	previous := backend.Counterexample()
	if previous == nil || previous.Root() == nil {
		t.Fatal("missing initial counterexample")
	}
	if status, err := backend.Query(nil); status != lbool.Undef || err == nil {
		t.Fatal("nil query unexpectedly succeeded", status, err)
	}
	if previous.Root() != nil || previous.Tree() != nil || backend.Counterexample() != nil {
		t.Fatal("failed query left borrowed counterexample live")
	}

	query()
	previous = backend.Counterexample()
	m.Budget().FailAfter(0)
	status, failure := backend.Query(prepared)
	m.Budget().FailAfter(-1)
	if status != lbool.Undef || failure == nil {
		t.Fatal("allocation-refused query unexpectedly succeeded", status, failure)
	}
	if previous.Root() != nil || previous.Tree() != nil || backend.Counterexample() != nil {
		t.Fatal("allocation failure left borrowed counterexample live")
	}
	query()
	if backend.Counterexample() == nil || backend.Counterexample().Root() == nil {
		t.Fatal("retry did not publish a fresh counterexample")
	}
}
