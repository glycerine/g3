package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATSolverNNFStrengtheningAndScope(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	// nnf_strengthening/weakening_extrapolate use the tactic's default
	// m_is_relevant and m_mk_atom, not a solver_context's.
	defaults := &satNNFDefault{manager: m}
	normalizer, err := NewQENNFNormalizer(m, defaults, defaults, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, normalizer.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	p := f.constant("strengthen-p", f.boolean)
	q := f.constant("strengthen-q", f.boolean)
	b := f.own(m.MkOr(p, q))
	truth := f.own(m.MkTrue())
	fmMust(t, solver.AssertFormula(b))
	checkpoints := 0
	result, err := satStrengthen(m, normalizer, solver, rw, truth, b, func() error { checkpoints++; return nil })
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	if h != q {
		t.Fatal("expected q after strengthening", h, q)
	}
	if checkpoints != 2 {
		t.Fatalf("got %d checkpoints", checkpoints)
	}
	// The successful assumption p=false belonged to the temporary scope.
	fmMust(t, solver.Push())
	fmMust(t, solver.AssertFormula(p))
	notQ := f.own(m.MkNot(q))
	fmMust(t, solver.AssertFormula(notQ))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("strengthening leaked assertions", status)
	}
	fmMust(t, solver.Pop(1))
}

// AUDIT.md qe_05-F4: nnf_strengthen/nnf_weaken iterate the atom_set in
// obj_hashtable order, not in AST id order.
func TestSATExclusiveAtomsUseTableOrder(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	defaults := &satNNFDefault{manager: m}
	normalizer, err := NewQENNFNormalizer(m, defaults, defaults, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, normalizer.Close()) }()
	var args []ast.ASTHandle
	for _, name := range []string{"order-a", "order-b", "order-c", "order-d", "order-e", "order-f"} {
		args = append(args, f.constant(name, f.boolean))
	}
	b := f.own(m.MkOrArray(args))
	ref, err := m.NewRef(b)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&ref)) }()
	atoms, err := normalizer.Normalize(ref)
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	got, err := satExclusiveAtoms(atoms, true)
	fmMust(t, err)
	want, err := atoms.positiveTable()
	fmMust(t, err)
	if len(got) != len(args) || len(got) != len(want) {
		t.Fatal("atom count", len(got), len(want))
	}
	byID := true
	for i := range got {
		if got[i] != want[i] {
			t.Fatal("atom order differs from obj_hashtable order at", i)
		}
		if i > 0 {
			a, e := m.GetASTID(got[i-1])
			fmMust(t, e)
			b, e := m.GetASTID(got[i])
			fmMust(t, e)
			byID = byID && a < b
		}
	}
	if byID {
		t.Log("fixture table order coincides with AST id order")
	}
}

func TestSATSolverNNFStrengtheningSkipsMixedPolarityAndCleansFailure(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	// nnf_strengthening/weakening_extrapolate use the tactic's default
	// m_is_relevant and m_mk_atom, not a solver_context's.
	defaults := &satNNFDefault{manager: m}
	normalizer, err := NewQENNFNormalizer(m, defaults, defaults, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, normalizer.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	p := f.constant("mixed-p", f.boolean)
	q := f.constant("mixed-q", f.boolean)
	notP := f.own(m.MkNot(p))
	left := f.own(m.MkOr(p, q))
	right := f.own(m.MkOr(notP, q))
	b := f.own(m.MkAnd(left, right))
	truth := f.own(m.MkTrue())
	fmMust(t, solver.AssertFormula(b))
	count := 0
	result, err := satStrengthen(m, normalizer, solver, rw, truth, b, func() error { count++; return nil })
	fmMust(t, err)
	fmMust(t, m.DelRef(&result))
	if count != 1 {
		t.Fatalf("mixed-polarity atom was considered: %d checkpoints", count)
	}
	stopped := errors.New("stop")
	_, err = satStrengthen(m, normalizer, solver, rw, truth, b, func() error { return stopped })
	if !errors.Is(err, stopped) {
		t.Fatal("checkpoint failure not propagated", err)
	}
	fmMust(t, solver.Push())
	fmMust(t, solver.AssertFormula(p))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("failed strengthening left a solver scope", status)
	}
	fmMust(t, solver.Pop(1))
}
