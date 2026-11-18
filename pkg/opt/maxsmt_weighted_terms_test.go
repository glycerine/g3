package opt

// Source: src/opt/sortmax.cpp unsigned-weight input expansion.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxSMTScaledTermsAndExpansionPreserveMultiplicity(t *testing.T) {
	fixture := newWeightedSoftFixture(t)
	refs := make([]ast.ASTRef, 2)
	var err error
	refs[0], err = fixture.manager.NewRef(fixture.p)
	if err == nil {
		refs[1], err = fixture.manager.NewRef(fixture.q)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.manager.DelRef(&refs[0])
	defer fixture.manager.DelRef(&refs[1])
	objective := &arithmeticObjective{softTerms: refs, softWeights: []numeral.Rational{testRat(2, 1), testRat(3, 1)}}
	q := fixture.manager.NumeralService().Manager
	terms, err := maxSMTRationalTerms(fixture.manager, objective)
	if err != nil || !ratIs(&terms[0].weight, 2, 1) || !ratIs(&terms[1].weight, 3, 1) {
		t.Fatal(terms, err)
	}
	for i := range terms {
		w, unsigned, e := maxSMTUnsigned(q, &terms[i].weight)
		if e != nil || !unsigned {
			t.Fatal("unsigned weight", i, e)
		}
		terms[i].units = w
	}
	expanded := maxSMTExpandTerms(terms, 5)
	if len(expanded) != 5 || expanded[0] != fixture.p || expanded[1] != fixture.p || expanded[2] != fixture.q {
		t.Fatal("expanded weighted terms", expanded)
	}
	if err = releaseMaxSMTTerms(q, terms); err != nil {
		t.Fatal(err)
	}
	// AUDIT.md opt_01-F1: a fractional weight is kept, not refused;
	// sortmax rejects it after find_mutexes.
	objective.softWeights[0] = testRat(3, 2)
	objective.softWeights[1] = testRat(2, 3)
	terms, err = maxSMTRationalTerms(fixture.manager, objective)
	if err != nil || !ratIs(&terms[0].weight, 3, 2) || !ratIs(&terms[1].weight, 2, 3) {
		t.Fatal("fractional weights", terms, err)
	}
	if _, unsigned, e := maxSMTUnsigned(q, &terms[0].weight); e != nil || unsigned {
		t.Fatal("3/2 is not unsigned", e)
	}
	if w, unsigned, e := maxSMTUnsigned(q, testRatPtr(12, 6)); e != nil || !unsigned || w != 2 {
		t.Fatal("12/6 is unsigned 2", w, unsigned, e)
	}
	big32 := testRat(0, 1)
	if err = q.SetUint64(&big32, 1<<32); err != nil {
		t.Fatal(err)
	}
	if _, unsigned, e := maxSMTUnsigned(q, &big32); e != nil || unsigned {
		t.Fatal("2^32 is not unsigned", e)
	}
	if err = errors.Join(q.Del(&big32), releaseMaxSMTTerms(q, terms)); err != nil {
		t.Fatal(err)
	}
}

// AUDIT.md opt_01-F1: maxsmt::operator() builds sortmax for
// maxsat_engine=sortmax whatever the weight total; g3 used to fall back to
// MaxRes above a total of 4096.
func TestSortMaxSelectedForLargeWeightTotal(t *testing.T) {
	fixture := newWeightedSoftFixture(t)
	refs := make([]ast.ASTRef, 2)
	var err error
	refs[0], err = fixture.manager.NewRef(fixture.p)
	if err == nil {
		refs[1], err = fixture.manager.NewRef(fixture.q)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.manager.DelRef(&refs[0])
	defer fixture.manager.DelRef(&refs[1])
	objective := &arithmeticObjective{soft: true, softTerms: refs, softWeights: []numeral.Rational{testRat(100000, 1), testRat(5000, 1)}}
	solver := &ArithmeticSolver{manager: fixture.manager}
	engine, err := selectMaxSMTEngine(solver, objective, 0, maxSMTConfig{engine: "sortmax"})
	if err != nil {
		t.Fatal(err)
	}
	sortmax, ok := engine.(*sortMaxEngine)
	if !ok {
		t.Fatalf("maxsat_engine=sortmax built %T", engine)
	}
	if err = sortmax.close(); err != nil {
		t.Fatal(err)
	}
}
