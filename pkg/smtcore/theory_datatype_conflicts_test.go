package smtcore

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtDatatypeConflictFixture struct {
	m                  *ast.ASTManager
	k                  *SMTKernel
	dt                 *SMTDatatypeTheory
	x, y, left, right  ast.ASTHandle
	equality, negative ast.ASTHandle
	recognizer         SMTLiteral
}

func newSMTDatatypeConflictFixture(t *testing.T, proofs bool) smtDatatypeConflictFixture {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	if proofs {
		astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	}
	s, err := NewSMTSolver(m, nil, "QF_DT", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	s.kernel.preprocess = false
	s.kernel.smtParams.datatype.LazySplits = 2
	uSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "conflict-element-sort"), nil)
	astTestMust(t, err)
	var definitions []ast.DatatypeConstructorDefinition
	for _, tag := range []string{"left", "middle", "right"} {
		definitions = append(definitions, ast.DatatypeConstructorDefinition{Name: astTestSymbol(t, m, "conflict-"+tag), Recognizer: astTestSymbol(t, m, "conflict-is-"+tag), Accessors: []ast.DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "conflict-field-"+tag), Range: uSort}}})
	}
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: astTestSymbol(t, m, "conflict-choice"), Constructors: definitions}})
	astTestMust(t, err)
	constructors, err := m.DatatypeConstructors(sorts[0])
	astTestMust(t, err)
	element, err := m.MkConst(astTestSymbol(t, m, "conflict-element"), uSort)
	astTestMust(t, err)
	f := smtDatatypeConflictFixture{m: m, k: s.kernel}
	f.x, err = m.MkConst(astTestSymbol(t, m, "conflict-x"), sorts[0])
	astTestMust(t, err)
	f.y, err = m.MkConst(astTestSymbol(t, m, "conflict-y"), sorts[0])
	astTestMust(t, err)
	f.left, err = m.MkApp(constructors[0], []ast.ASTHandle{element})
	astTestMust(t, err)
	f.right, err = m.MkApp(constructors[2], []ast.ASTHandle{element})
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{f.x, f.y, f.left, f.right} {
		astTestPin(t, m, h)
		astTestMust(t, s.kernel.internalizer.Internalize(h, false))
	}
	family, err := m.DatatypeFamilyID()
	astTestMust(t, err)
	f.dt = s.kernel.internalizer.theories[family].(*SMTDatatypeTheory)
	// Keep the source legacy recognizer declaration to exercise the
	// datatype callback independently of indexed recognizer rewriting.
	recognizer, err := m.DatatypeConstructorRecognizer(constructors[0])
	astTestMust(t, err)
	expression, err := m.MkApp(recognizer, []ast.ASTHandle{f.x})
	astTestMust(t, err)
	astTestMust(t, s.kernel.internalizer.Internalize(expression, true))
	f.recognizer, err = s.kernel.internalizer.Literal(expression)
	astTestMust(t, err)
	f.negative, err = m.MkNot(expression)
	astTestMust(t, err)
	f.equality, err = m.MkEq(f.x, f.left)
	astTestMust(t, err)
	astTestPin(t, m, f.negative)
	astTestPin(t, m, f.equality)
	return f
}

func (f smtDatatypeConflictFixture) assert(t *testing.T, expression ast.ASTHandle) {
	t.Helper()
	proof := ast.ASTHandle{}
	enabled, err := f.m.ProofsEnabled()
	astTestMust(t, err)
	if enabled {
		proof, err = f.m.MkProofAsserted(expression)
		astTestMust(t, err)
	}
	astTestMust(t, f.k.internalizer.Assert(expression, proof, 0))
	_, err = f.k.internalizer.PropagateCore()
	astTestMust(t, err)
}

func TestSMTDatatypeConstructorRecognizerConflictsBeforeFinalCheck(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, mergeFirst := range []bool{false, true} {
			t.Run(fmt.Sprint(proofs, "/", mergeFirst), func(t *testing.T) {
				f := newSMTDatatypeConflictFixture(t, proofs)
				s := f.k.internalizer
				base, child := f.negative, f.equality
				if mergeFirst {
					base, child = child, base
				}
				f.assert(t, base)
				if s.Inconsistent() {
					t.Fatal("consistent base already conflicted")
				}
				for attempt := 0; attempt < 2; attempt++ {
					astTestMust(t, s.PushScope())
					var trace bytes.Buffer
					tags := diagnostic.NewTraceTagsWithWriter(&trace)
					tags.Enable("recognizer_conflict")
					f.dt.trace = tags
					reconstructions := f.dt.statistics.constructorAxioms
					conflicts := s.statistics.NumConflicts
					f.assert(t, child)
					conflict, found := s.Conflict()
					if !found {
						t.Fatal("constructor/recognizer conflict was deferred to final checking")
					}
					owner, _ := conflict.Justification()
					extended, ok := owner.(*SMTExtTheoryConflictJustification)
					if !ok || extended.FromTheory() != f.dt.FamilyID() || len(extended.Literals) != 1 || extended.Literals[0] != f.recognizer.Negated() || len(extended.Equalities) != 1 {
						t.Fatal("conflict did not retain source literal/equality evidence", owner)
					}
					pair := extended.Equalities[0]
					if pair.First.owner != f.left || pair.Second.owner != f.x || pair.First.root != pair.Second.root || f.dt.statistics.constructorAxioms != reconstructions || s.statistics.NumConflicts != conflicts { // set_conflict does not count
						t.Fatal("conflict reconstructed another constructor or lost its actual class evidence")
					}
					wantTrace := ""
					if tags.ShouldTrace("recognizer_conflict") {
						wantTrace = smtDatatypeTraceRecord("recognizer_conflict", "sign_recognizer_conflict", 177, "(conflict-left conflict-element)\n(conflict-is-left conflict-x)\n")
					}
					if trace.String() != wantTrace {
						t.Fatal("constructor/recognizer conflict trace", trace.String(), wantTrace)
					}
					status, err := f.k.resolveCheckConflict()
					astTestMust(t, err)
					if status != lbool.False {
						t.Fatal("conflict resolution did not refute the branch")
					}
					if proofs {
						proof, err := f.k.Proof()
						astTestMust(t, err)
						fact, err := f.m.GetProofFact(proof)
						astTestMust(t, err)
						if !f.m.IsFalse(fact) {
							t.Fatal("extended datatype conflict did not produce false proof")
						}
					}
					astTestMust(t, s.PopScopes(1))
					if s.Inconsistent() {
						t.Fatal("scoped datatype conflict survived pop")
					}
				}
			})
		}
	}
}

func TestSMTDatatypeDisjointConstructorsConflictDuringMerge(t *testing.T) {
	f := newSMTDatatypeConflictFixture(t, false)
	xLeft, err := f.m.MkEq(f.x, f.left)
	astTestMust(t, err)
	yRight, err := f.m.MkEq(f.y, f.right)
	astTestMust(t, err)
	f.assert(t, xLeft)
	f.assert(t, yRight)
	s := f.k.internalizer
	if s.Inconsistent() {
		t.Fatal("separate constructor classes already conflict")
	}
	astTestMust(t, s.PushScope())
	equal, err := f.m.MkEq(f.x, f.y)
	astTestMust(t, err)
	f.assert(t, equal)
	conflict, found := s.Conflict()
	if !found {
		t.Fatal("disjoint constructor merge was deferred")
	}
	owner, _ := conflict.Justification()
	extended, ok := owner.(*SMTExtTheoryConflictJustification)
	if !ok || len(extended.Literals) != 0 || len(extended.Equalities) != 1 {
		t.Fatal("disjoint conflict did not retain its constructor equality", owner)
	}
	pair := extended.Equalities[0]
	if pair.First.root != pair.Second.root || pair.First.owner != f.left && pair.First.owner != f.right || pair.Second.owner != f.left && pair.Second.owner != f.right {
		t.Fatal("disjoint conflict used unrelated constructor evidence")
	}
	astTestMust(t, s.PopScopes(1))
	if s.Inconsistent() {
		t.Fatal("disjoint constructor conflict did not pop")
	}
}

func TestSMTDatatypeKnownConstructorPositiveRecognizerNeedsNoReconstruction(t *testing.T) {
	f := newSMTDatatypeConflictFixture(t, false)
	f.assert(t, f.equality)
	s := f.k.internalizer
	expression, err := s.BoolExpression(f.recognizer.Var())
	astTestMust(t, err)
	values, reconstructions := len(f.dt.data), f.dt.statistics.constructorAxioms
	f.assert(t, expression)
	if s.Inconsistent() || len(f.dt.data) != values || f.dt.statistics.constructorAxioms != reconstructions {
		t.Fatal("known constructor was reconstructed again")
	}
	assignment, err := s.Assignment(f.recognizer)
	astTestMust(t, err)
	if assignment != lbool.True {
		t.Fatal("positive recognizer was not assigned")
	}
}
