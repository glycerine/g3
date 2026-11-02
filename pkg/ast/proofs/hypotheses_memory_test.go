package proofs

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestProofHypothesisSetMemoryWrappers(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	m := proofManager(t, tags)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		proofMust(t, m.IncRef(h))
		t.Cleanup(func() { proofMust(t, m.DecRef(h)) })
		return h
	}
	sort, err := m.MkBoolSort()
	proofMust(t, err)
	a, err := m.MkConst(symbol.Numerical(801), sort)
	proofMust(t, err)
	a = pin(a)
	b, err := m.MkConst(symbol.Numerical(802), sort)
	proofMust(t, err)
	b = pin(b)
	c, err := m.MkConst(symbol.Numerical(803), sort)
	proofMust(t, err)
	c = pin(c)
	ab, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpIff, nil, []ast.ASTHandle{a, b})
	proofMust(t, err)
	ab = pin(ab)
	bc, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpIff, nil, []ast.ASTHandle{b, c})
	proofMust(t, err)
	bc = pin(bc)
	p, err := m.MkProofHypothesis(ab)
	proofMust(t, err)
	p = pin(p)
	q, err := m.MkProofHypothesis(bc)
	proofMust(t, err)
	q = pin(q)
	joined, err := m.MkProofTransitivity(p, q)
	proofMust(t, err)
	joined = pin(joined)
	tags.Enable("memory")
	h := newHypothesisSets(m)
	for _, proof := range []ast.ASTHandle{p, q, joined} {
		proofMust(t, h.add(proof))
	}
	if len(h.owned) != 3 || len(h.byProof[joined]) != 2 {
		t.Fatal("hypothesis set allocations did not follow source branches")
	}
	owners := append([]*hypothesisSetOwner(nil), h.owned...)
	h.reset()
	used := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	refusal := h.add(p)
	m.BudgetField.FailAfter(-1)
	if refusal == nil || len(h.owned) != 0 || len(h.byProof) != 0 || m.BudgetField.Used() != used {
		t.Fatal("refused set owner allocation changed proof state or budget", refusal)
	}
	got := output.String()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		if got != "" {
			t.Fatal("build without memory tracing emitted debug memory records", got)
		}
		return
	}
	if strings.Count(got, "[memory] allocate ") != 3 || strings.Count(got, "[memory] deallocate ") != 3 {
		t.Fatal("hypothesis set memory record count", got)
	}
	size := strconv.FormatUint(uint64(reflect.TypeOf(hypothesisSetOwner{}).Size()), 10)
	last := -1
	for i, owner := range owners {
		line := 398
		if i == 2 {
			line = 412
		}
		part := "alloc " + fmt.Sprintf("%p", owner) + " src/ast/proofs/proof_utils.cpp:" + strconv.Itoa(line) + " expr_set " + size + "\n"
		at := strings.Index(got, part)
		if at <= last || strings.Count(got, part) != 1 {
			t.Fatalf("allocation source order for %q in %q", part, got)
		}
		last = at
	}
	for _, owner := range owners {
		part := "dealloc " + fmt.Sprintf("%p", owner) + " src/util/util.h:224\n"
		at := strings.Index(got, part)
		if at <= last || strings.Count(got, part) != 1 {
			t.Fatalf("delete_proc source order for %q in %q", part, got)
		}
		last = at
	}
}

func TestProofLemmaEmptyHypothesisSetImmediateDeallocation(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	m := proofManager(t, tags)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	falsehood, err := m.MkFalse()
	proofMust(t, err)
	notFalse, err := m.MkNot(falsehood)
	proofMust(t, err)
	proofMust(t, m.IncRef(notFalse))
	t.Cleanup(func() { proofMust(t, m.DecRef(notFalse)) })
	hypothesis, err := m.MkProofHypothesis(falsehood)
	proofMust(t, err)
	proofMust(t, m.IncRef(hypothesis))
	t.Cleanup(func() { proofMust(t, m.DecRef(hypothesis)) })
	lemma, err := m.MkProofLemma(hypothesis, notFalse)
	proofMust(t, err)
	ref, err := m.NewRef(lemma)
	proofMust(t, err)
	defer func() { proofMust(t, m.DelRef(&ref)) }()
	tags.Enable("memory")
	proofMust(t, ReduceHypotheses(m, ref))
	got := output.String()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		if got != "" {
			t.Fatal("build without memory tracing emitted debug memory records", got)
		}
		return
	}
	alloc := strings.Index(got, "src/ast/proofs/proof_utils.cpp:517 expr_set ")
	free := strings.Index(got, "src/ast/proofs/proof_utils.cpp:569\n")
	if alloc < 0 || free <= alloc || strings.Count(got, "src/ast/proofs/proof_utils.cpp:517 expr_set ") != 1 || strings.Count(got, "src/ast/proofs/proof_utils.cpp:569\n") != 1 {
		t.Fatal("lemma clone did not immediately release its empty set", got)
	}
}
