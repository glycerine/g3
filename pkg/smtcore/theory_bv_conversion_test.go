package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func bvConversionFixture(t *testing.T, relevance uint32) (*ast.ASTManager, *SMTSolver, *SMTBitVectorTheory) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	p, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
	key := "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&p, &key, false))
	key = "relevancy"
	astTestMust(t, cfg.Context.SetUIntName(&p, &key, relevance))
	solver, err := NewSMTSolver(m, &p, "", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	solver.kernel.preprocess = false
	astTestMust(t, solver.Push())
	s := solver.kernel.internalizer
	family, err := m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	bv, ok := s.theories[family].(*SMTBitVectorTheory)
	if !ok || s.relevancy == nil || s.relevancy.enabled != (relevance != 0) || s.theoryRewriter == nil {
		t.Fatal("actual conversion context setup")
	}
	return m, solver, bv
}

func bvConversionUnits(s *SMTInternalizerState, bv *SMTBitVectorTheory) []SMTLiteral {
	var result []SMTLiteral
	for _, owner := range s.ownedJustifications {
		if j, ok := owner.(*SMTTheoryAxiomJustification); ok && j.FromTheory() == bv.FamilyID() && len(j.Literals) == 1 {
			result = append(result, j.Literals[0])
		}
	}
	return result
}

func bvConversionInt(t *testing.T, m *ast.ASTManager, text string) ast.ASTHandle {
	t.Helper()
	h, err := m.ArithmeticIntegerNumeralText(text)
	astTestMust(t, err)
	return h
}

func TestSMTBVConversionSourceAxiomsAndRelevance(t *testing.T) {
	for _, level := range []uint32{0, 2} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			m, solver, bv := bvConversionFixture(t, level)
			s := solver.kernel.internalizer
			zero := bvConversionInt(t, m, "0")
			sort, err := m.GetSort(zero)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "conversion-native-x"), sort)
			astTestMust(t, err)
			term, err := m.MkIntToBitVector(4, x)
			astTestMust(t, err)
			astTestMust(t, s.Internalize(term, false))
			if level != 0 {
				if got := len(bvConversionUnits(s, bv)); got != 0 {
					t.Fatal("axioms published before relevance", got)
				}
				astTestMust(t, s.relevancy.MarkRelevant(term))
				astTestMust(t, s.relevancy.Propagate())
				if !s.relevancy.IsRelevant(x) {
					t.Fatal("conversion operand not relevant")
				}
			}
			units := bvConversionUnits(s, bv)
			if len(units) != 6 {
				t.Fatal("source weighted sum, top modulus, and four bit equations", len(units))
			}
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle {
				h, e := m.MkFamilyApp(family, op, nil, args)
				astTestMust(t, e)
				return h
			}
			expected := make(map[ast.ASTHandle]bool)
			var expectedOrder []ast.ASTHandle
			top, err := m.MkBitVectorToInt(term)
			astTestMust(t, err)
			topMod := app(13, x, bvConversionInt(t, m, "16"))
			eq, err := s.mkEqualityAtom(top, topMod)
			astTestMust(t, err)
			expected[eq] = true
			expectedOrder = append(expectedOrder, eq)
			variable := s.context.enodes.get(term).TheoryVar(bv.FamilyID())
			for i, bit := range bv.bitLiterals[variable] {
				division := app(11, x, bvConversionInt(t, m, fmt.Sprint(1<<uint(i))))
				parity := app(13, division, bvConversionInt(t, m, "2"))
				condition, err := m.MkEq(parity, bvConversionInt(t, m, "1"))
				astTestMust(t, err)
				bitExpr, err := s.literalExpression(bit)
				astTestMust(t, err)
				eq, err := s.mkEqualityAtom(bitExpr, condition)
				astTestMust(t, err)
				expected[eq] = true
				expectedOrder = append(expectedOrder, eq)
			}
			position := 0
			for _, unit := range units {
				expression, err := s.BoolExpression(unit.Var())
				astTestMust(t, err)
				if expected[expression] {
					if position >= len(expectedOrder) || expression != expectedOrder[position] {
						t.Fatal("conversion equations published out of source order", position)
					}
					position++
				}
				delete(expected, expression)
				assignment, err := s.Assignment(unit)
				astTestMust(t, err)
				if assignment != lbool.True {
					t.Fatal("conversion axiom not a unit", unit)
				}
				if level != 0 && !s.relevancy.IsRelevant(expression) {
					t.Fatal("axiom literal was not marked relevant", unit)
				}
			}
			if len(expected) != 0 {
				t.Fatal("missing source div/mod bit equations", len(expected))
			}
		})
	}
}

func TestSMTBVConversionNativeSignedBitsAndMissingEnode(t *testing.T) {
	m, solver, bv := bvConversionFixture(t, 2)
	s := solver.kernel.internalizer
	p, q := smtRelevancyBool(t, m, "conversion-native-p"), smtRelevancyBool(t, m, "conversion-native-q")
	notQ, err := m.MkNot(q)
	astTestMust(t, err)
	node, _ := bvNativeWord(t, m, s, bv, p, notQ)
	astTestMust(t, bv.Relevant(node.owner))
	if !s.relevancy.IsRelevant(p) || !s.relevancy.IsRelevant(q) || s.context.relevant.get(notQ) {
		t.Fatal("native signed literal relevance must use the positive atom")
	}
	converted, err := m.MkBitVectorToInt(node.owner)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(converted, false))
	if len(bvConversionUnits(s, bv)) != 0 {
		t.Fatal("premature signed-word axiom")
	}
	astTestMust(t, s.relevancy.MarkRelevant(converted))
	units := bvConversionUnits(s, bv)
	if len(units) != 1 {
		t.Fatal("signed-word conversion axiom", len(units))
	}
	// Rewriter input is the source native literals p and (not q), rather than
	// freshly constructed projections of the composite MKBV term.
	one, two, zero := bvConversionInt(t, m, "1"), bvConversionInt(t, m, "2"), bvConversionInt(t, m, "0")
	a, err := m.MkIte(p, one, zero)
	astTestMust(t, err)
	b, err := m.MkIte(notQ, two, zero)
	astTestMust(t, err)
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	sum, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{a, b})
	astTestMust(t, err)
	ref, err := m.NewRef(sum)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&ref)) }()
	astTestMust(t, s.theoryRewriter.Rewrite(sum, ref))
	sum, err = m.RefGet(ref)
	astTestMust(t, err)
	expected, err := s.mkEqualityAtom(converted, sum)
	astTestMust(t, err)
	actual, err := s.BoolExpression(units[0].Var())
	astTestMust(t, err)
	if actual != expected {
		t.Fatal("conversion did not use the rewritten native signed bits")
	}
	sort, err := m.GetSort(node.owner)
	astTestMust(t, err)
	missing, err := m.MkConst(astTestSymbol(t, m, "uninternalized-relevant-bv"), sort)
	astTestMust(t, err)
	before := len(bv.bits)
	astTestMust(t, bv.Relevant(missing))
	if len(bv.bits) != before || s.context.enodes.get(missing) != nil {
		t.Fatal("relevance fabricated a missing enode or circuit")
	}
}

func TestSMTBVComparisonDefinitionSurvivesRelevancePop(t *testing.T) {
	for _, level := range []uint32{0, 2} {
		for _, lazy := range []bool{false, true} {
			t.Run(fmt.Sprint(level, "/", lazy), func(t *testing.T) {
				m, solver, bv := bvConversionFixture(t, level)
				s := solver.kernel.internalizer
				bv.config.LazyLE = lazy
				zero, err := m.BitVectorNumeralUint64(0, 4)
				astTestMust(t, err)
				sort, err := m.GetSort(zero)
				astTestMust(t, err)
				x, err := m.MkConst(astTestSymbol(t, m, "comparison-source-x"), sort)
				astTestMust(t, err)
				y, err := m.MkConst(astTestSymbol(t, m, "comparison-source-y"), sort)
				astTestMust(t, err)
				term, err := m.MkBitVectorULE(x, y)
				astTestMust(t, err)
				astTestMust(t, s.Internalize(term, true))
				variable := s.boolVar.get(term)
				definition, ok := bv.comparisonAtoms[variable]
				if !ok {
					t.Fatal("source comparison definition was not retained")
				}
				before := len(s.clauses)
				bridges := 0
				for _, clause := range s.clauses {
					if len(clause.Literals) != 2 {
						continue
					}
					l := NewSMTLiteral(variable, false)
					got := boundClauseSet(clause.Literals)
					if got == boundClauseSet([]SMTLiteral{l, definition.Negated()}) || got == boundClauseSet([]SMTLiteral{l.Negated(), definition}) {
						bridges++
					}
				}
				wantBridges := 2
				if level != 0 && lazy {
					wantBridges = 0
				}
				if bridges != wantBridges {
					t.Fatal("source comparison bridge timing", bridges, wantBridges)
				}
				if level == 0 {
					return
				}
				for attempt := 0; attempt < 2; attempt++ {
					astTestMust(t, solver.Push())
					before = len(s.clauses)
					astTestMust(t, s.relevancy.MarkRelevant(term))
					expr, err := s.BoolExpression(definition.Var())
					astTestMust(t, err)
					if !s.relevancy.IsRelevant(expr) {
						t.Fatal("comparison definition not relevant")
					}
					if lazy && len(s.clauses) != before+2 {
						t.Fatal("lazy bridge missing after re-entry", attempt, before, len(s.clauses))
					}
					astTestMust(t, solver.Pop(1))
					if got, ok := bv.comparisonAtoms[variable]; !ok || got != definition {
						t.Fatal("relevance pop discarded base definition")
					}
				}
				astTestMust(t, solver.Pop(1))
				if len(bv.comparisonAtoms) != 0 {
					t.Fatal("comparison native atom survived its scope")
				}
			})
		}
	}
}

func TestSMTBVConversionRelevanceReentryAndDisabledFallback(t *testing.T) {
	for _, toBV := range []bool{false, true} {
		t.Run(fmt.Sprint(toBV), func(t *testing.T) {
			m, solver, bv := bvConversionFixture(t, 2)
			s := solver.kernel.internalizer
			var term ast.ASTHandle
			if toBV {
				zero := bvConversionInt(t, m, "0")
				sort, err := m.GetSort(zero)
				astTestMust(t, err)
				x, err := m.MkConst(astTestSymbol(t, m, "conversion-reentry-int"), sort)
				astTestMust(t, err)
				term, err = m.MkIntToBitVector(3, x)
				astTestMust(t, err)
			} else {
				_, v := bvFixedSymbolic(t, m, s, bv, "conversion-reentry-bv", 3)
				var err error
				term, err = m.MkBitVectorToInt(bv.variables[v].owner)
				astTestMust(t, err)
			}
			root, err := m.NewRef(term)
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&root)) }()

			astTestMust(t, s.Internalize(term, false))
			for attempt := 0; attempt < 2; attempt++ {
				astTestMust(t, solver.Push())
				astTestMust(t, s.relevancy.MarkRelevant(term))
				astTestMust(t, s.relevancy.Propagate())
				want := 1
				if toBV {
					want = 5
				}
				if got := len(bvConversionUnits(s, bv)); got != want {
					t.Fatal("scoped conversion axioms", attempt, got, want)
				}
				astTestMust(t, solver.Pop(1))
				if s.relevancy.IsRelevant(term) || len(bvConversionUnits(s, bv)) != 0 {
					t.Fatal("conversion relevance/axioms escaped scope")
				}
			}
			// Disabling the native option uses the retained core UF path. It must
			// not publish either conversion's arithmetic equations.
			bv.config.EnableInt2BV = false
			astTestMust(t, solver.Pop(1))
			astTestMust(t, s.Internalize(term, false))
			astTestMust(t, s.relevancy.MarkRelevant(term))
			astTestMust(t, s.relevancy.Propagate())
			if len(bvConversionUnits(s, bv)) != 0 {
				t.Fatal("disabled conversion emitted native axioms")
			}
		})
	}
}

func TestSMTBVConversionAllocationRefusalAndScopedRetry(t *testing.T) {
	failures := 0
	for step := int64(0); step < 8; step++ {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			m, solver, bv := bvConversionFixture(t, 2)
			s := solver.kernel.internalizer
			_, v := bvFixedSymbolic(t, m, s, bv, "conversion-cold-wide", 257)
			term, err := m.MkBitVectorToInt(bv.variables[v].owner)
			astTestMust(t, err)
			astTestMust(t, s.Internalize(term, false))
			astTestMust(t, solver.Push())
			m.Budget().FailAfter(step)
			err = s.relevancy.MarkRelevant(term)
			m.Budget().FailAfter(-1)
			if err != nil {
				failures++
			}
			astTestMust(t, solver.Pop(1))
			astTestMust(t, solver.Push())
			astTestMust(t, s.relevancy.MarkRelevant(term))
			if len(bvConversionUnits(s, bv)) != 1 {
				t.Fatal("conversion retry did not publish its unit")
			}
			astTestMust(t, solver.Pop(1))
		})
	}
	if failures == 0 {
		t.Fatal("cold conversion never reached an injected allocation refusal")
	}
}
