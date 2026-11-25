package smtcore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type nestedClauseJustification struct {
	smtClauseTestJustification
	insert func()
}

func (j *nestedClauseJustification) InRegion() bool {
	if j.insert != nil {
		insert := j.insert
		j.insert = nil
		insert()
	}
	return false
}

func TestSMTClauseScratchAcrossNestedInsertion(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	var ls []SMTLiteral
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		ls = append(ls, scopeRestorationLiterals(t, s, smtRelevancyBool(t, m, name))...)
	}
	astTestMust(t, s.addClause(ls[:3], false, ast.ASTHandle{})) // warm the scratch
	j := &nestedClauseJustification{insert: func() {
		astTestMust(t, s.AddLearnedClause(ls[3:]))
	}}
	astTestMust(t, s.addClauseKindWithJustificationAndDeleteHandler(ls[:3], false, SMTClauseLearned, ast.ASTHandle{}, j, nil))
	if len(s.lemmas) != 2 || !reflect.DeepEqual(s.lemmas[0].activeClause().Literals(), ls[3:]) || !reflect.DeepEqual(s.lemmas[1].activeClause().Literals(), ls[:3]) {
		t.Fatal("nested clause insertion changed the outer or inner literals")
	}
}

func TestSMTClauseAtomPoolMixedLifetimes(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "saved-atom")
	astTestPin(t, m, a)
	expressions := []ast.ASTHandle{{}, a}
	lit := NewSMTLiteral(1, false)
	var pool smtClauseAtomPool
	makeClause := func(ls []SMTLiteral) *SMTClause {
		t.Helper()
		c, err := newSMTClause(m, ls, SMTClauseLearned, nil, nil, true, expressions, &pool, false)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, c.Close()) })
		return c
	}
	short := []SMTLiteral{lit, lit.Negated()}
	var live []*SMTClause
	for range len(pool.pages) * smtAtomSlotsPerPage {
		live = append(live, makeClause(short))
	}
	if makeClause(short).atomPage != nil || makeClause([]SMTLiteral{lit, lit, lit, lit, lit}).atomPage != nil {
		t.Fatal("full pool or long clause used a short atom slot")
	}
	c := live[1]
	page, slot := c.atomPage, c.atomSlot
	if page == nil {
		t.Fatal("short clause missed an available slot")
	}
	astTestMust(t, c.ReleaseAtoms())
	if page.free != 0 {
		t.Fatal("public ReleaseAtoms invalidated the clause's Atom view")
	}
	astTestMust(t, c.Close())
	// Fail after retaining the first atom. Close must release it and return the slot.
	_, err := newSMTClause(m, []SMTLiteral{lit, NewSMTLiteral(2, false)}, SMTClauseLearned, nil, nil, true, expressions, &pool, false)
	if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid atom", err)
	}
	reused := makeClause([]SMTLiteral{lit.Negated(), lit})
	if reused.atomPage != page || reused.atomSlot != slot {
		t.Fatal("released slot could not be reused beside a long-lived clause")
	}
	for _, c := range []*SMTClause{live[0], live[len(live)-1], reused} {
		h, sign, err := c.Atom(0)
		if err != nil || h != a || sign != (c == reused) {
			t.Fatal("one clause changed another clause's saved atom", h, sign, err)
		}
	}
}
