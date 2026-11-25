package smtcore

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextStatisticsClauseOccurrencesLevelsAndProfile(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "context-stat-a")
	b := smtRelevancyBool(t, m, "context-stat-b")
	c := smtRelevancyBool(t, m, "context-stat-c")
	for _, expression := range []ast.ASTHandle{a, b, c} {
		astTestMust(t, state.Internalize(expression, true))
	}
	av, _ := state.BoolVar(a)
	bv, _ := state.BoolVar(b)
	cv, _ := state.BoolVar(c)
	la, lb, lc := NewSMTLiteral(av, false), NewSMTLiteral(bv, false), NewSMTLiteral(cv, false)
	aux, err := NewSMTClause(m, []SMTLiteral{la, lb.Negated()}, SMTClauseAux, nil, nil, false, nil)
	astTestMust(t, err)
	lemma, err := NewSMTClause(m, []SMTLiteral{la.Negated(), lc}, SMTClauseLearned, nil, nil, false, nil)
	astTestMust(t, err)
	astTestMust(t, lemma.SetActivity(5))
	aux.registerDirectory(false, ast.ASTHandle{}, nil)
	state.auxClauses = append(state.auxClauses, aux)
	lemma.registerDirectory(false, ast.ASTHandle{}, nil)
	state.lemmas = append(state.lemmas, lemma)
	t.Cleanup(func() {
		if len(state.scopes) != 0 {
			astTestMust(t, state.PopScopes(uint32(len(state.scopes))))
		}
		state.auxClauses, state.lemmas = nil, nil
		astTestMust(t, lemma.Close())
		astTestMust(t, aux.Close())
	})
	if average, err := state.LemmaAverageActivity(); err != nil || average != 5 {
		t.Fatal("lemma average activity", average, err)
	}

	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(la, NewSMTBooleanAxiom()))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(lb, NewSMTBooleanAxiom()))
	astTestMust(t, state.AssignLiteral(lc, NewSMTBooleanAxiom()))
	var levels bytes.Buffer
	astTestMust(t, state.DisplayAssignedLiteralsPerLevel(&levels))
	if levels.String() != "[0 1 2]" {
		t.Fatal("assigned literals per level", levels.String())
	}

	var histogram bytes.Buffer
	astTestMust(t, state.DisplayVariableOccurrenceHistogram(&histogram))
	if histogram.String() != "number of atoms having k occs:\n1:2 2:1 \n" {
		t.Fatal("variable occurrence histogram", histogram.String())
	}
	var minimum bytes.Buffer
	astTestMust(t, state.DisplayMinimumVariableOccurrences(&minimum))
	wantMinimum := "number of min occs:\n" + strconv.FormatInt(int64(av), 10) + ":2 \n"
	if minimum.String() != wantMinimum {
		t.Fatal("minimum occurrence profile", minimum.String(), wantMinimum)
	}
	var literals bytes.Buffer
	astTestMust(t, state.DisplayLiteralOccurrences(&literals))
	for _, fragment := range []string{"1 0 context-stat-a", "1 1 context-stat-a", "1 1 context-stat-b", "1 0 context-stat-c"} {
		if !strings.Contains(literals.String(), fragment) {
			t.Fatal("literal occurrence profile", fragment, literals.String())
		}
	}

	read, write, err := os.Pipe()
	astTestMust(t, err)
	original := os.Stderr
	os.Stderr = write
	var ignored bytes.Buffer
	profileErr := state.DisplayProfile(&ignored, true)
	os.Stderr = original
	astTestMust(t, write.Close())
	profile, readErr := io.ReadAll(read)
	astTestMust(t, read.Close())
	astTestMust(t, profileErr)
	astTestMust(t, readErr)
	if ignored.Len() != 0 || string(profile) != histogram.String()+minimum.String()+"\n" {
		t.Fatal("source stderr profile", ignored.String(), string(profile))
	}
}
