package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func fpaDisplayFixture(t *testing.T) (*ast.ASTManager, *SMTInternalizerState, *SMTFloatingPointTheory, []ast.ASTHandle, string) {
	t.Helper()

	m, cfg := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	family, err := m.RegisterFloatingPointPlugin()
	astTestMust(t, err)
	sort, err := m.FloatingPointSort(2, 2)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	s := solver.kernel.internalizer
	fp := s.theories[family].(*SMTFloatingPointTheory)
	a, err := m.MkConst(astTestSymbol(t, m, "first fpa"), sort)
	astTestMust(t, err)
	b, err := m.MkConst(astTestSymbol(t, m, "second-fpa"), sort)
	astTestMust(t, err)
	an, err := s.MkEnode(a, false, false, true)
	astTestMust(t, err)
	bn, err := s.MkEnode(b, false, false, true)
	astTestMust(t, err)
	// Use real sort-constraint callbacks, deliberately attaching in the reverse
	// order from AST creation and enode registration.
	astTestMust(t, fp.ApplySortConstraint(bn, sort))
	astTestMust(t, fp.ApplySortConstraint(an, sort))
	integerFamily, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	integerSort, err := m.MkFamilySort(integerFamily, 1, nil)
	astTestMust(t, err)
	integer, err := m.MkConst(astTestSymbol(t, m, "integer-var"), integerSort)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(integer, false))
	bvSort, err := m.BitVectorSort(3)
	astTestMust(t, err)
	bv, err := m.MkConst(astTestSymbol(t, m, "bits-var"), bvSort)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(bv, false))
	_, foreign := smtUFConstants(t, m, "foreign-", 1)
	astTestMust(t, s.Internalize(foreign[0], false))
	terms := []ast.ASTHandle{a, b, integer, bv, foreign[0]}
	// theory_arith::apply_sort_cnstr does nothing: a bare integer constant
	// has no arithmetic variable until an arithmetic term mentions it.
	want := "fpa theory variables:\n1 -> |first fpa|\n0 -> second-fpa\nbv theory variables:\n0 -> bits-var\narith theory variables:\nequivalence classes:\n1 --> true\n2 --> false\n"
	for i, name := range []string{"|first fpa|", "second-fpa", "integer-var", "bits-var", "foreign-a"} {
		id, err := m.GetASTID(terms[i])
		astTestMust(t, err)
		want += fmt.Sprintf("%d --> %s\n", id, name)
	}
	return m, s, fp, terms, want
}

func TestSMTFPADisplayUsesContextOrderAndActualFamilyVariables(t *testing.T) {
	m, s, fp, terms, want := fpaDisplayFixture(t)
	var out bytes.Buffer
	astTestMust(t, fp.Display(&out))
	if out.String() != want {
		t.Fatal("FPA source display", out.String(), want)
	}
	out.Reset()
	astTestMust(t, s.DisplayTheories(&out))
	contextWant := fpaFixtureBVDisplayExpected(t, m, terms[3]) + want
	if out.String() != contextWant {
		t.Fatal("context theory composition", out.String(), contextWant)
	}
}

// This fixture has one independent three-bit BV variable, registered before
// the FPA owner. Source display_theories invokes both concrete printers.
func fpaFixtureBVDisplayExpected(t *testing.T, m *ast.ASTManager, word ast.ASTHandle) string {
	t.Helper()
	id, err := m.GetASTID(word)
	astTestMust(t, err)
	var bits [3]uint32
	for i := range bits {
		bit, err := m.MkBitVectorBit2Bool(uint32(i), word)
		astTestMust(t, err)
		bits[i], err = m.GetASTID(bit)
		astTestMust(t, err)
	}
	return fmt.Sprintf("Theory bv:\nv%-4d #%-4d -> #%-4d, bits: #%d #%d #%d\natoms:\n#%d -> #%d[0]\n#%d -> #%d[1]\n#%d -> #%d[2]\n", 0, id, id, bits[0], bits[1], bits[2], bits[0], id, bits[1], id, bits[2], id)
}

type theoryDisplayFlushWriter struct {
	output  bytes.Buffer
	flushes int
	refuse  bool
}

func (w *theoryDisplayFlushWriter) Write(p []byte) (int, error) { return w.output.Write(p) }
func (w *theoryDisplayFlushWriter) Flush() error {
	w.flushes++
	if w.refuse {
		return io.ErrClosedPipe
	}
	return nil
}

func TestSMTFPADisplayWriterPrefixesFlushAndOwnerStability(t *testing.T) {
	m, s, fp, _, want := fpaDisplayFixture(t)
	w := &theoryDisplayFlushWriter{}
	astTestMust(t, fp.Display(w))
	// Three section headings, three variable lines and seven equivalence lines
	// (with context::init's true and false) use endl. The equivalence heading
	// has an ordinary newline.
	if w.output.String() != want || w.flushes != 13 {
		t.Fatal(w.output.String(), w.flushes)
	}
	before := m.Stats()
	rng := s.random
	for cut := 0; cut < len(want); cut++ {
		out := &smtLRAOutputPrefix{remaining: cut}
		err := fp.Display(out)
		if !errors.Is(err, io.ErrClosedPipe) || out.text.String() != want[:cut] {
			t.Fatal("FPA writer prefix", cut, err, out.text.String(), want[:cut])
		}
	}
	if err := fp.Display(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	refused := &theoryDisplayFlushWriter{refuse: true}
	if err := fp.Display(refused); !errors.Is(err, io.ErrClosedPipe) || refused.output.String() != "fpa theory variables:\n" || refused.flushes != 1 {
		t.Fatal("endl refusal", err, refused.output.String(), refused.flushes)
	}
	if before != m.Stats() || s.random != rng {
		t.Fatal("display changed owners or RNG", before, m.Stats())
	}
}

func TestSMTFPADisplayMergeRootAndScopeRollback(t *testing.T) {
	m, s, fp, terms, base := fpaDisplayFixture(t)
	contextBase := fpaFixtureBVDisplayExpected(t, m, terms[3]) + base
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(s.context.enodes.get(terms[0]), s.context.enodes.get(terms[1]), NewSMTEqualityAxiom()))
	extra, err := m.MkConst(astTestSymbol(t, m, "scoped-fpa"), mustFPASort(t, m, terms[0]))
	astTestMust(t, err)
	astTestMust(t, s.Internalize(extra, false))
	var out bytes.Buffer
	astTestMust(t, fp.Display(&out))
	root, err := s.context.enodes.get(terms[0]).Root().OwnerID()
	astTestMust(t, err)
	firstID, err := m.GetASTID(terms[0])
	astTestMust(t, err)
	secondID, err := m.GetASTID(terms[1])
	astTestMust(t, err)
	want := strings.Replace(base, fmt.Sprintf("%d --> |first fpa|\n", firstID), fmt.Sprintf("%d --> |first fpa|\n", root), 1)
	want = strings.Replace(want, fmt.Sprintf("%d --> second-fpa\n", secondID), fmt.Sprintf("%d --> second-fpa\n", root), 1)
	want = strings.Replace(want, "bv theory variables:\n", "2 -> scoped-fpa\nbv theory variables:\n", 1)
	id, err := m.GetASTID(extra)
	astTestMust(t, err)
	want += fmt.Sprintf("%d --> scoped-fpa\n", id)
	if out.String() != want {
		t.Fatal("context root/late scope order", out.String(), want)
	}
	astTestMust(t, s.PopScopes(1))
	out.Reset()
	astTestMust(t, s.DisplayTheories(&out))
	if out.String() != contextBase {
		t.Fatal("FPA display rollback", out.String(), contextBase)
	}
}

func mustFPASort(t *testing.T, m *ast.ASTManager, value ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	return sort
}

func TestSMTFPADisplayEmptyGateAndGuards(t *testing.T) {
	m, family, _, solver := smtFloatingPointSolver(t)
	fp := solver.kernel.internalizer.theories[family].(*SMTFloatingPointTheory)
	bvSort, err := m.BitVectorSort(3)
	astTestMust(t, err)
	foreign, err := m.MkConst(astTestSymbol(t, m, "unused-fpa-bits"), bvSort)
	astTestMust(t, err)
	astTestMust(t, solver.kernel.internalizer.Internalize(foreign, false))
	var out bytes.Buffer
	before := m.Stats()
	m.Budget().FailAfter(0)
	err = fp.Display(&out)
	m.Budget().FailAfter(-1)
	astTestMust(t, err)
	if out.Len() != 0 || before != m.Stats() {
		t.Fatal("unused FPA owner printed or allocated")
	}
	if err = fp.Display(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if err = (*SMTFloatingPointTheory)(nil).Display(&out); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	uninitialized, err := NewSMTFloatingPointTheory(family)
	astTestMust(t, err)
	if err = uninitialized.Display(&out); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
}

func TestSMTFPADisplayColdAllocationRefusalAndRetry(t *testing.T) {
	m, _, fp, _, want := fpaDisplayFixture(t)
	var out bytes.Buffer
	m.Budget().FailAfter(0)
	err := fp.Display(&out)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("cold format allocation did not refuse", err)
	}
	// The section and numeric prefix are inserted before the source pp operand.
	if out.String() != "fpa theory variables:\n1 -> " {
		t.Fatal("format refusal prefix", out.String())
	}
	out.Reset()
	astTestMust(t, fp.Display(&out))
	if out.String() != want {
		t.Fatal("format refusal destroyed borrowed input", out.String(), want)
	}
}
