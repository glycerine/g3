package duality_test

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPreparedQueryCopyCannotMutateOrReleaseOriginalOwners(t *testing.T) {
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
	truth, falsity := pin(m.MkTrue()), pin(m.MkFalse())
	prepared, err := duality.PrepareQuery(m, nil, nil, falsity, nil, math.MaxUint32)
	must(err)
	must(prepared.AddBackground(truth))
	before := m.Stats().LiveReferences
	copied := *prepared
	if err := copied.AddBackground(truth); !errors.Is(err, graph.ErrWrapperOwner) {
		t.Fatal("copied query accepted background", err)
	}
	if _, err := copied.Clauses(); !errors.Is(err, graph.ErrWrapperOwner) {
		t.Fatal("copied query exposed clauses", err)
	}
	if _, err := copied.Bounds(); !errors.Is(err, graph.ErrWrapperOwner) {
		t.Fatal("copied query exposed bounds", err)
	}
	if err := copied.Close(); !errors.Is(err, graph.ErrWrapperOwner) {
		t.Fatal("copied query released original owners", err)
	}
	if after := m.Stats().LiveReferences; after != before {
		t.Fatal("copied query changed reference ownership", before, after)
	}
	if status, err := backend.Query(&copied); status != lbool.Undef || !errors.Is(err, graph.ErrWrapperOwner) {
		t.Fatal("backend accepted copied query", status, err)
	}
	status, err := backend.Query(prepared)
	must(err)
	if status != lbool.False {
		t.Fatal("original query damaged by copied operations", status)
	}
	must(prepared.Close())
	must(prepared.Close())
	if _, err := prepared.Clauses(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed original query still exposed clauses", err)
	}
}
