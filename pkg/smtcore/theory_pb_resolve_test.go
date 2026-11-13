package smtcore

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func smtPBResolveExpression(t *testing.T, m *ast.ASTManager, family ast.FamilyID, args []ast.ASTHandle, coefficients []int32, k int32) ast.ASTHandle {
	t.Helper()
	var parameters []ast.ASTParameter
	for _, value := range append([]int32{k}, coefficients...) {
		p, err := m.NewParameterInt(value)
		astTestMust(t, err)
		parameters = append(parameters, p)
	}
	h, err := m.MkFamilyApp(family, ast.ASTPBOpGE, parameters, args)
	astTestMust(t, err)
	for i := range parameters {
		astTestMust(t, m.DelParameter(&parameters[i]))
	}
	return h
}

func smtPBResolutionRoot(t *testing.T, s *SMTInternalizerState, literal SMTLiteral, proofs bool) {
	t.Helper()
	if !proofs {
		astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
		return
	}
	h, err := s.BoolExpression(literal.Var())
	astTestMust(t, err)
	if literal.Sign() {
		h, err = s.manager.MkNot(h)
		astTestMust(t, err)
	}
	p, err := s.manager.MkProofAsserted(h)
	astTestMust(t, err)
	astTestMust(t, s.assignRootLiteral(literal, p))
}

func TestSMTPBResolutionNativeCutPreservesAssumptionGuardsAndProof(t *testing.T) {
	for _, test := range []struct {
		frequency uint32
		prior     uint32
		cut       bool
	}{{0, 0, true}, {1, 0, true}, {3, 0, false}, {3, 1, true}} {
		for _, proofs := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/%d/%d", proofs, test.frequency, test.prior), func(t *testing.T) {
				m, s, pb, a, inputs := smtPBWatchFixture(t, []int64{4, 3, 2}, 6, false)
				if proofs {
					astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				x, err := s.BoolExpression(inputs[0].Var())
				astTestMust(t, err)
				notX, err := m.MkNot(x)
				astTestMust(t, err)
				z := smtRelevancyBool(t, m, "resolve-z")
				w := smtRelevancyBool(t, m, "resolve-w")
				h := smtPBResolveExpression(t, m, pb.FamilyID(), []ast.ASTHandle{notX, z, w}, []int32{3, 2, 1}, 4)
				astTestMust(t, s.Internalize(h, true))
				b, err := s.Literal(h)
				astTestMust(t, err)
				pb.controls.ConflictFrequency = test.frequency
				pb.constraints[1].propagations = test.prior
				astTestMust(t, s.PushScope())
				smtPBResolutionRoot(t, s, a, proofs)
				smtPBResolutionRoot(t, s, b, proofs)
				s.booleanData[a.Var()].Assumption = true
				s.booleanData[b.Var()].Assumption = true
				_, err = s.PropagateCore()
				astTestMust(t, err)
				astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
				s.searchUserBaseLevel = 0
				defer func() { astTestMust(t, s.endSearchPersistence()) }()
				astTestMust(t, s.PushScope())
				smtPBWatchAssign(t, s, inputs[1].Negated())
				conflict, found := s.Conflict()
				if !found {
					t.Fatal("native weighted cut did not publish conflict")
				}
				clause, found := conflict.Clause()
				if !found || clause == nil || !clause.IsAuxLemma() {
					t.Fatal("weighted cut was not an auxiliary lemma")
				}
				if !test.cut {
					expectSMTRawCore(t, clause.literals, []SMTLiteral{inputs[0].Negated(), b.Negated()})
					if pb.statistics.conflicts != 1 || pb.constraints[1].propagations != test.prior+1 {
						t.Fatal("conflict-frequency gate did not skip this cut")
					}
					return
				}
				// mk_clause sorts CLS_AUX_LEMMA literals (simplify_aux_lemma_literals).
				expectSMTRawCore(t, clause.literals, []SMTLiteral{a.Negated(), b.Negated()})
				if pb.statistics.conflicts != 2 || pb.constraints[1].propagations != test.prior+2 {
					t.Fatal("strengthening path was skipped", pb.statistics)
				}
				resolver := smtCoreWalkResolver(t, s)
				core, err := resolver.UnsatCore(conflict, s.ConflictLiteral())
				astTestMust(t, err)
				seen := map[SMTLiteral]bool{}
				for _, l := range core {
					seen[l] = true
				}
				if !seen[a] || !seen[b] {
					t.Fatal("cut dropped an assumption guard", core)
				}
				if proofs {
					// FORK FIX (z3 4.7.1 bug; upstream removed this code in its pb
					// rewrite, first in 4.8.5): the cut is a heap theory lemma.
					if _, ok := clause.Justification().(*SMTTheoryLemmaJustification); !ok {
						t.Fatal("cut lost its theory-lemma justification")
					}
					proof, err := resolver.conflictProof(conflict, s.ConflictLiteral(), nil)
					astTestMust(t, err)
					fact, err := m.GetProofFact(proof)
					astTestMust(t, err)
					if !m.IsFalse(fact) {
						t.Fatal("cut conflict proof is not false")
					}
				}
			})
		}
	}
}

func TestSMTPBResolutionCoefficientAlignmentAndComplementControls(t *testing.T) {
	m, s, pb, atom, inputs := smtPBWatchFixture(t, []int64{4, 3, 2}, 6, false)
	z := smtRelevancyBool(t, m, "cut-scale-z")
	astTestMust(t, s.Internalize(z, true))
	zl, err := s.Literal(z)
	astTestMust(t, err)
	astTestMust(t, s.PushScope())
	for _, l := range []SMTLiteral{atom, inputs[0], inputs[1].Negated(), zl} {
		astTestMust(t, s.AssignLiteral(l, NewSMTBooleanAxiom()))
	}
	r := smtPBResolution{theory: pb, base: 0, level: 1, marks: map[int32]int{zl.Var(): 0}, numMarks: 1}
	r.lemma.terms = []smtPBTerm{{literal: zl.Negated(), coeff: *big.NewRat(2, 1)}}
	r.lemma.bound.SetInt64(1)
	coefficient := big.NewRat(2, 1)
	astTestMust(t, r.inequality(pb.constraints[0], inputs[0], coefficient))
	if coefficient.RatString() != "2" || r.lemma.bound.RatString() != "2" || len(r.lemma.terms) != 2 || r.lemma.terms[0].coeff.RatString() != "4" || r.lemma.terms[1].coeff.RatString() != "3" || r.numMarks != 2 {
		t.Fatal("LCM alignment or weakening differs from source", r.lemma, r.numMarks)
	}
	expectSMTRawCore(t, r.guards, []SMTLiteral{atom})
	for _, enabled := range []bool{false, true} {
		pb.controls.LearnComplements = enabled
		r = smtPBResolution{theory: pb, base: 0, level: 1, marks: map[int32]int{inputs[0].Var(): 0}, numMarks: 1}
		r.lemma.terms = []smtPBTerm{{literal: inputs[0].Negated(), coeff: *big.NewRat(3, 1)}}
		r.lemma.bound.SetInt64(4)
		astTestMust(t, r.antecedent(inputs[0], big.NewRat(2, 1)))
		want := "3"
		if enabled {
			want = "1"
		}
		if r.lemma.terms[0].coeff.RatString() != want || r.lemma.bound.RatString() != "2" {
			t.Fatal("complement-learning control ignored", enabled, r.lemma)
		}
		astTestMust(t, r.antecedent(inputs[0], big.NewRat(2, 1)))
		if enabled && (len(r.lemma.terms) != 0 || len(r.marks) != 0 || r.numMarks != 0) {
			t.Fatal("complement cancellation did not remove mark")
		}
	}
}

func TestSMTPBResolutionResidualPredicateRetainsScopedFalsePremises(t *testing.T) {
	_, s, pb, atom, inputs := smtPBWatchFixture(t, []int64{2, 1, 1, 1}, 3, false)
	astTestMust(t, s.PushScope())
	for _, l := range []SMTLiteral{atom, inputs[1].Negated(), inputs[2].Negated()} {
		astTestMust(t, s.AssignLiteral(l, NewSMTBooleanAxiom()))
		s.booleanData[l.Var()].Assumption = true
	}
	astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
	s.searchUserBaseLevel = 0
	defer func() { astTestMust(t, s.endSearchPersistence()) }()
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(inputs[0].Negated(), NewSMTBooleanAxiom()))
	// This explicit pending callback order exercises the source's
	// asynchronous-resolution branch without silently processing the older
	// false input before entering the new scope.
	resolved, err := pb.resolveConflict(pb.constraints[0])
	astTestMust(t, err)
	if !resolved || s.inconsistent || len(pb.constraints) != 2 {
		t.Fatal("residual predicate folded scoped false inputs into an axiom")
	}
	residual := pb.constraints[1]
	if len(residual.current().terms) != 2 || residual.current().bound.RatString() != "2" {
		t.Fatal("wrong residual cardinality")
	}
	astTestMust(t, pb.Assign(residual.literal.Var(), true))
	if !s.inconsistent {
		t.Fatal("residual callback did not expose its scoped false premises")
	}
	// A redundant decision premise keeps the original decision as the UIP,
	// forcing Boolean resolution to expand the strengthened predicate reason.
	conflict := smtCoreWalkReason(t, residual.literal, inputs[0].Negated(), inputs[1].Negated(), inputs[2].Negated())
	resolver := smtCoreWalkResolver(t, s)
	result, err := resolver.resolve(conflict, SMTNullLiteral, 1, 0)
	astTestMust(t, err)
	expectSMTRawCore(t, result.Lemma, []SMTLiteral{inputs[0], inputs[1], inputs[2], atom.Negated()})
	for bits := uint32(0); bits < 16; bits++ {
		values := map[int32]bool{}
		sum := int64(0)
		for i, w := range []int64{2, 1, 1, 1} {
			values[inputs[i].Var()] = bits&(1<<uint(i)) != 0
			if values[inputs[i].Var()] {
				sum += w
			}
		}
		values[atom.Var()] = sum >= 3
		holds := false
		for _, l := range result.Lemma {
			v := values[l.Var()]
			if l.Sign() {
				v = !v
			}
			holds = holds || v
		}
		if !holds {
			t.Fatal("learned PB lemma is not valid for the original weighted relation", bits, result.Lemma)
		}
	}
}

func TestSMTPBResolutionClauseAndForeignReasonBarriers(t *testing.T) {
	for _, kind := range []string{"foreign-object", "opaque-clause", "plain-clause", "proof-wrapper-clause", "binary"} {
		t.Run(kind, func(t *testing.T) {
			m, s, pb, atom, inputs := smtPBWatchFixture(t, []int64{4, 3, 2}, 6, false)
			if kind == "proof-wrapper-clause" {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			p := smtRelevancyBool(t, m, "cut-reason-p")
			astTestMust(t, s.Internalize(p, true))
			pl, err := s.Literal(p)
			astTestMust(t, err)
			astTestMust(t, s.AssignLiteral(inputs[1], NewSMTBooleanAxiom()))
			astTestMust(t, s.PushScope())
			astTestMust(t, s.AssignLiteral(atom, NewSMTBooleanAxiom()))
			astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
			s.searchUserBaseLevel = 0
			defer func() { astTestMust(t, s.endSearchPersistence()) }()
			astTestMust(t, s.PushScope())
			astTestMust(t, s.AssignLiteral(pl, NewSMTBooleanAxiom()))
			var reason SMTBooleanJustification
			expand := kind == "plain-clause" || kind == "proof-wrapper-clause" || kind == "binary"
			switch kind {
			case "foreign-object":
				owner, err := NewSMTSimpleJustification(true, nil)
				astTestMust(t, err)
				s.ownedJustifications = append(s.ownedJustifications, owner)
				reason, err = NewSMTBooleanJustification(owner)
				astTestMust(t, err)
			case "binary":
				reason = NewSMTBooleanBinaryClause(pl)
			default:
				var owner SMTJustification
				if kind == "opaque-clause" {
					owner, err = NewSMTSimpleJustification(false, nil)
					astTestMust(t, err)
				}
				if kind == "proof-wrapper-clause" {
					x, err := s.BoolExpression(inputs[0].Var())
					astTestMust(t, err)
					nx, err := m.MkNot(x)
					astTestMust(t, err)
					np, err := m.MkNot(p)
					astTestMust(t, err)
					fact, err := m.MkOrArgs([]ast.ASTHandle{nx, np})
					astTestMust(t, err)
					proof, err := m.MkProofAsserted(fact)
					astTestMust(t, err)
					owner, err = NewSMTJustificationProofWrapper(m, proof, false)
					astTestMust(t, err)
				}
				clause, err := NewSMTClause(m, []SMTLiteral{inputs[0].Negated(), pl.Negated()}, SMTClauseAux, owner, nil, false, nil)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, clause.Close()) })
				reason = NewSMTBooleanClause(clause)
			}
			astTestMust(t, s.AssignLiteral(inputs[0].Negated(), reason))
			astTestMust(t, s.AssignLiteral(inputs[2].Negated(), NewSMTBooleanAxiom()))
			var trace bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&trace)
			tags.Enable("pb")
			pb.trace = tags
			resolved, err := pb.resolveConflict(pb.constraints[0])
			astTestMust(t, err)
			if !resolved || !s.inconsistent {
				t.Fatal("reason branch did not publish a guarded cut")
			}
			if tags.ShouldTrace("pb") {
				line := 1699
				switch kind {
				case "foreign-object":
					line = 1733
				case "opaque-clause":
					line = 1683
				case "binary":
					line = 1704
				}
				if !strings.Contains(trace.String(), fmt.Sprintf("[pb] resolve_conflict src/smt/theory_pb.cpp:%d", line)) {
					t.Fatal("missing concrete reason source trace", kind, trace.String())
				}
				if (kind == "foreign-object" || kind == "opaque-clause") && !strings.Contains(trace.String(), " simple\n") {
					t.Fatal("lost concrete justification identity", trace.String())
				}
			}
			conflict, _ := s.Conflict()
			clause, ok := conflict.Clause()
			if !ok {
				t.Fatal("missing source auxiliary clause")
			}
			last := inputs[0]
			if expand {
				last = pl.Negated()
			}
			// Context selects the two conflict-level watched positions first;
			// the older predicate guard remains as the third literal.
			// mk_clause sorts CLS_AUX_LEMMA literals by index before choosing
			// watches; the unassigned atom literal keeps the last slot here.
			first, second := inputs[2], last
			if second.Index() < first.Index() {
				first, second = second, first
			}
			expectSMTRawCore(t, clause.literals, []SMTLiteral{first, second, atom.Negated()})
		})
	}
}
