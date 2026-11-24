package smtcore

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTBinaryClauseDirectoryWidens(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a := smtRelevancyBool(t, m, "binary-wide")
	proof, err := m.MkProofAsserted(a)
	astTestMust(t, err)
	astTestPin(t, m, proof)

	var d smtBinaryClauseDirectory
	pairs := [][2]SMTLiteral{
		{math.MaxUint16 - 1, math.MaxUint16},
		{3, 4},
		{math.MaxUint16 + 1, 2},
		{math.MaxInt32 - 1, math.MaxInt32},
		{7, 8},
	}
	var before []SMTInternalizedClause
	for i, p := range pairs {
		var reason ast.ASTHandle
		if i%2 == 0 {
			reason = proof
		}
		d.add(p[0], p[1], i%2 == 0, i%3 == 0, reason)
		if i == 1 {
			if len(d.short) != 2 || d.wide != nil {
				t.Fatal("widened before a literal exceeded 16 bits")
			}
			before = d.appendSnapshots(nil)
		}
	}
	if d.short != nil || len(d.wide) != len(pairs) || d.count() != len(pairs) {
		t.Fatal("wide directory lost entries or retained the old backing array")
	}
	records := d.appendSnapshots(nil)
	for i, pair := range pairs {
		r := records[i]
		wantProof := ast.ASTHandle{}
		if i%2 == 0 {
			wantProof = proof
		}
		if d.pair(i) != pair || len(r.Literals) != 2 || [2]SMTLiteral(r.Literals) != pair ||
			!r.Binary || r.Root != (i%2 == 0) || r.Lemma != (i%3 == 0) || r.Proof != wantProof {
			t.Fatalf("incorrect entry at %d: %+v", i, r)
		}
	}
	before[0].Literals[0] = SMTNullLiteral
	if d.pair(0) != pairs[0] || [2]SMTLiteral(records[0].Literals) != pairs[0] {
		t.Fatal("earlier snapshot still aliases stored literals")
	}
}

func TestSMTBinaryClauseCloseResumesSecondWatch(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	astTestMust(t, s.SetBinaryClauseOptimization(true))
	var literals []SMTLiteral
	for _, name := range []string{"binary-a", "binary-b", "binary-c"} {
		atom := smtRelevancyBool(t, m, name)
		astTestMust(t, s.Internalize(atom, false))
		variable, ok := s.BoolVar(atom)
		if !ok {
			t.Fatal("missing Boolean", name)
		}
		literals = append(literals, NewSMTLiteral(variable, false))
	}
	astTestMust(t, s.addClause(literals[:2], true, ast.ASTHandle{}))
	astTestMust(t, s.addClause(literals[1:], false, ast.ASTHandle{}))
	published := s.Clauses()
	if len(published) != 2 || !published[0].Binary || !published[1].Binary {
		t.Fatal("expected two watch-only binary clauses")
	}
	first, second := published[0].Literals[0], published[0].Literals[1]
	if !s.watches[second.Negated().Index()].RemoveLiteral(first) {
		t.Fatal("missing initial reverse watch")
	}
	if err := s.Close(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("expected refusal on missing reverse watch", err)
	}
	partial := s.Clauses()
	if len(partial) != 2 || !partial[0].Binary || !partial[0].Root || partial[1].Binary || !published[1].Binary {
		t.Fatal("incorrect snapshot after interrupted flush", partial)
	}
	if s.watches[first.Negated().Index()].FindLiteral(second) >= 0 {
		t.Fatal("first watch still installed")
	}
	astTestMust(t, s.watches[second.Negated().Index()].InsertLiteral(first))
	astTestMust(t, s.Close())
	astTestMust(t, s.Close())
	if len(s.Clauses()) != 0 {
		t.Fatal("closed solver retained binary clauses")
	}
}
