package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTLRAConflictUsesScopedExtendedSourceJustification(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			if proofs {
				astTestMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
			}
			x, y := f.variable("owner-conflict-x"), f.variable("owner-conflict-y")
			astTestMust(t, f.s.PushScope())
			var want []SMTLiteral
			for _, h := range []ast.ASTHandle{f.app(smtArithOpGE, x, f.number(0)), f.app(smtArithOpGE, y, f.number(0)), f.app(smtArithOpLE, f.app(smtArithOpAdd, x, y), f.number(-1))} {
				f.assert(h, true)
				literal, err := f.s.Literal(h)
				astTestMust(t, err)
				want = append(want, literal)
			}
			f.propagate(false)
			conflict, found := f.s.Conflict()
			if !found {
				t.Fatal("missing numerical conflict")
			}
			object, found := conflict.Justification()
			if !found {
				t.Fatal("conflict has no source owner")
			}
			owner, ok := object.(*SMTExtTheoryConflictJustification)
			if !ok {
				t.Fatalf("LRA source requires extended conflict even without equality premises: %T", object)
			}
			if owner.Name() != "ext-theory-conflict" || owner.FromTheory() != f.a.FamilyID() || !owner.InRegion() || len(owner.Equalities) != 0 || len(owner.Parameters()) != 0 || len(owner.Literals) != 3 || boundClauseSet(owner.Literals) != boundClauseSet(want) {
				t.Fatal("source conflict data", owner)
			}
			collector := &smtAntecedentCollector{}
			astTestMust(t, owner.MarkAntecedents(collector))
			if len(collector.equalities) != 0 || len(collector.literals) != 3 || boundClauseSet(collector.literals) != boundClauseSet(want) {
				t.Fatal("extended antecedent traversal", collector)
			}
			owned := 0
			for _, j := range f.s.ownedJustifications {
				if j == owner {
					owned++
				}
			}
			if owned != 1 || f.s.statistics.NumConflicts != 0 { // resolve_conflict counts, set_conflict does not
				t.Fatal("conflict owner registration/count", owned, f.s.statistics.NumConflicts)
			}
			if proofs {
				builder := &smtConflictProofBuilder{state: f.s, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle), objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle)}
				proof := f.pin(builder.objectProof(owner))
				fact, err := f.m.GetProofFact(proof)
				astTestMust(t, err)
				if !f.m.IsFalse(fact) {
					t.Fatal("extended conflict proof fact", fact)
				}
			}
			astTestMust(t, f.s.PopScopes(1))
			if _, found := f.s.Conflict(); found {
				t.Fatal("child conflict survived pop")
			}
			for _, j := range f.s.ownedJustifications {
				if j == owner {
					t.Fatal("child conflict owner survived pop")
				}
			}
		})
	}
}

func TestSMTLRABasicExternalReasonKeepsFarkasWithoutProofs(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			if proofs {
				astTestMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
			}
			config := f.a.config
			config.SmallLemmaSize = 0
			astTestMust(t, f.a.ConfigureArithmetic(config))
			x := f.variable("owner-basic-x")
			weak, strong := f.app(smtArithOpGE, x, f.number(1)), f.app(smtArithOpGE, x, f.number(2))
			astTestMust(t, f.s.Internalize(weak, true))
			astTestMust(t, f.s.Internalize(strong, true))
			astTestMust(t, f.a.Propagate())
			astTestMust(t, f.s.PushScope())
			f.assert(strong, true)
			weakVar, _ := f.s.BoolVar(weak)
			strongVar, _ := f.s.BoolVar(strong)
			literal := NewSMTLiteral(weakVar, false)
			// Checked validation initializes persistent declaration-plugin
			// parameters on this shared manager; warm them before the census.
			astTestMust(t, f.a.checkValidation(smtLRAValidateAssignment, []SMTLiteral{literal}, literal, nil, nil))
			before := f.m.Stats().LiveParameters
			astTestMust(t, f.a.propagateBasicBound(f.a.boundByBoolean[strongVar], true))
			data, err := f.s.BooleanData(weakVar)
			astTestMust(t, err)
			object, found := data.Justification().Justification()
			owner, ok := object.(*SMTExtTheoryPropagationJustification)
			if !found || !ok {
				t.Fatalf("source external propagation owner: %T", object)
			}
			if owner.Consequent != literal || len(owner.Literals) != 1 || owner.Literals[0] != literal || len(owner.Equalities) != 0 {
				t.Fatal("native self-premise changed", owner)
			}
			parameters := owner.Parameters()
			if len(parameters) != 3 {
				t.Fatal("Farkas metadata dropped with proofs", proofs, len(parameters))
			}
			symbol, err := f.m.ParameterSymbol(parameters[0])
			astTestMust(t, err)
			name, _, err := f.m.Symbols.BareString(symbol)
			astTestMust(t, err)
			if name != "farkas" {
				t.Fatal(name)
			}
			for _, p := range parameters[1:] {
				one, err := f.m.NewParameterRat(big.NewRat(1, 1))
				astTestMust(t, err)
				equal, err := f.m.ParameterEqual(p, one)
				astTestMust(t, err)
				astTestMust(t, f.m.DelParameter(&one))
				if !equal {
					t.Fatal("nonunit Farkas coefficient")
				}
			}
			if f.m.Stats().LiveParameters != before+3 {
				t.Fatal("external reason did not retain exactly its parameter copy", before, f.m.Stats().LiveParameters)
			}
			value, err := f.s.Assignment(literal)
			astTestMust(t, err)
			if value != lbool.True {
				t.Fatal("external assignment missing", value)
			}
			astTestMust(t, f.s.PopScopes(1))
			for _, p := range parameters {
				if _, err := f.m.ParameterKind(p); !errors.Is(err, ast.ErrASTHandle) {
					t.Fatal("scoped parameter remained live", err)
				}
			}
			if f.m.Stats().LiveParameters != before {
				t.Fatal("scoped parameter owner leaked", before, f.m.Stats().LiveParameters)
			}
		})
	}
}

func TestSMTLRABasicTautologyRetiresParametersWithoutProofs(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("owner-tautology-x")
	weak, strong := f.app(smtArithOpGE, x, f.number(1)), f.app(smtArithOpGE, x, f.number(2))
	astTestMust(t, f.s.Internalize(weak, true))
	astTestMust(t, f.s.Internalize(strong, true))
	astTestMust(t, f.a.Propagate())
	wv, _ := f.s.BoolVar(weak)
	literal := NewSMTLiteral(wv, false)
	astTestMust(t, f.a.checkValidation(smtLRAValidateAssignment, []SMTLiteral{literal}, literal, nil, nil))
	before := f.m.Stats().LiveParameters
	owners := len(f.s.ownedJustifications)
	clauses := len(f.s.Clauses())
	bv, _ := f.s.BoolVar(strong)
	astTestMust(t, f.a.propagateBasicBound(f.a.boundByBoolean[bv], true))
	if f.m.Stats().LiveParameters != before || len(f.s.ownedJustifications) != owners || len(f.s.Clauses()) != clauses {
		t.Fatal("discarded self-premise lemma retained a parameter owner")
	}
	value, err := f.s.Assignment(NewSMTLiteral(wv, false))
	astTestMust(t, err)
	if value != lbool.Undef {
		t.Fatal("native tautology became a new implication")
	}
}

func TestSMTLRABasicNoProofParameterRefusalKeepsOwners(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("owner-refusal-x")
	weak, strong := f.app(smtArithOpGE, x, f.number(1)), f.app(smtArithOpGE, x, f.number(2))
	astTestMust(t, f.s.Internalize(weak, true))
	astTestMust(t, f.s.Internalize(strong, true))
	astTestMust(t, f.a.Propagate())
	var padding []ast.ASTParameter
	defer func() {
		f.m.Budget().FailAfter(-1)
		for i := range padding {
			astTestMust(t, f.m.DelParameter(&padding[i]))
		}
	}()
	// Hold every reusable parameter slot so the next source parameter creation
	// must cross a real physical frontier rather than an already warm page.
	f.m.Budget().FailAfter(0)
	for i := 0; i < 8192; i++ {
		p, err := f.m.NewParameterRat(big.NewRat(1, 1))
		if err != nil {
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
			break
		}
		padding = append(padding, p)
		if i == 8191 {
			t.Fatal("did not reach a cold parameter frontier")
		}
	}
	before := f.m.Stats().LiveParameters
	owners := len(f.s.ownedJustifications)
	bv, _ := f.s.BoolVar(strong)
	err := f.a.propagateBasicBound(f.a.boundByBoolean[bv], true)
	f.m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("source parameter reservation was skipped", err)
	}
	if f.m.Stats().LiveParameters != before || len(f.s.ownedJustifications) != owners || f.a.statistics.boundsPropagations != 0 {
		t.Fatal("refused parameter creation published or leaked an owner")
	}
}
