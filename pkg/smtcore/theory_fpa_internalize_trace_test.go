package smtcore

// Source: src/smt/theory_fpa.cpp:426,452.

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type smtFPAInternalizeRefusingWriter struct{}

func (smtFPAInternalizeRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func smtFPAInternalizeTraceFixture(t *testing.T, writer io.Writer) (*ast.ASTManager, *SMTFloatingPointTheory, ast.FamilyID, ast.ASTHandle, *diagnostic.TraceTags) {
	t.Helper()
	owner, config := smtModelGeneratorFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("t_fpa_internalize")
	m, err := ast.NewASTManager(ast.ASTManagerConfig{
		Symbols: owner.Symbols, Numerals: owner.NumeralService(), Budget: owner.Budget(), TraceTags: tags,
	})
	astTestMust(t, err)
	astTestMust(t, m.RegisterDeclarationPlugins())
	_, err = m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	family, err := m.RegisterFloatingPointPlugin()
	astTestMust(t, err)
	sort, err := m.FloatingPointSort(2, 2)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_FPBV", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	theory := solver.kernel.internalizer.theories[family].(*SMTFloatingPointTheory)
	t.Cleanup(func() {
		astTestMust(t, solver.Close())
		astTestMust(t, m.Close())
	})
	return m, theory, family, sort, tags
}

func TestSMTFPAInternalizeAtomAndTermSourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, theory, family, sort, tags := smtFPAInternalizeTraceFixture(t, &output)
	zero, err := m.MkFPAZeroOfSort(sort, false)
	astTestMust(t, err)
	atom, err := m.MkFamilyApp(family, ast.FPAOpEq, nil, []ast.ASTHandle{zero, zero})
	astTestMust(t, err)
	matched, err := theory.InternalizeAtom(atom, false)
	astTestMust(t, err)
	if !matched {
		t.Fatal("floating-point atom was not internalized")
	}
	fields := make([]ast.ASTHandle, 3)
	for i, width := range []uint32{1, 2, 1} {
		fields[i], err = m.BitVectorNumeralUint64(0, width)
		astTestMust(t, err)
	}
	term, err := m.MkFamilyApp(family, ast.FPAOpFP, nil, fields)
	astTestMust(t, err)
	matched, err = theory.InternalizeTerm(term)
	astTestMust(t, err)
	if !matched {
		t.Fatal("floating-point term was not internalized")
	}
	if tags.ShouldTrace("t_fpa_internalize") {
		trace := output.String()
		for _, want := range []string{
			"-------- [t_fpa_internalize] internalize_atom src/smt/theory_fpa.cpp:426 ---------\n",
			"internalizing atom: (fp.eq (_ +zero 2 2) (_ +zero 2 2))\n",
			"-------- [t_fpa_internalize] internalize_term src/smt/theory_fpa.cpp:452 ---------\n",
			"internalizing term: (fp #b0 #b00 #b0)\n",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing source internalization trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out FPA internalization trace executed", output.String())
	}
}

func TestSMTFPAInternalizeTraceStreamRefusalDoesNotStop(t *testing.T) {
	m, theory, family, sort, _ := smtFPAInternalizeTraceFixture(t, smtFPAInternalizeRefusingWriter{})
	zero, err := m.MkFPAZeroOfSort(sort, false)
	astTestMust(t, err)
	atom, err := m.MkFamilyApp(family, ast.FPAOpEq, nil, []ast.ASTHandle{zero, zero})
	astTestMust(t, err)
	matched, err := theory.InternalizeAtom(atom, false)
	astTestMust(t, err)
	if !matched {
		t.Fatal("source ostream badbit stopped FPA atom internalization")
	}
}

func TestSMTFPAConversionCacheSourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, theory, family, _, tags := smtFPAInternalizeTraceFixture(t, &output)
	tags.Enable("t_fpa")
	tags.Enable("t_fpa_detail")
	fields := make([]ast.ASTHandle, 3)
	for i, width := range []uint32{1, 2, 1} {
		var err error
		fields[i], err = m.BitVectorNumeralUint64(0, width)
		astTestMust(t, err)
	}
	term, err := m.MkFamilyApp(family, ast.FPAOpFP, nil, fields)
	astTestMust(t, err)
	first, err := theory.convert(term)
	astTestMust(t, err)
	second, err := theory.convert(term)
	astTestMust(t, err)
	if first != term || second != term {
		t.Fatal("source FP constructor conversion fixed point", first, second, term)
	}
	if tags.ShouldTrace("t_fpa") {
		trace := output.String()
		if strings.Count(trace, "-------- [t_fpa] convert src/smt/theory_fpa.cpp:354 ---------\n") != 2 {
			t.Fatal("conversion entry trace count", trace)
		}
		for _, want := range []string{
			"-------- [t_fpa_detail] convert src/smt/theory_fpa.cpp:372 ---------\nconverted; caching:\n",
			"-------- [t_fpa_detail] convert src/smt/theory_fpa.cpp:358 ---------\ncached:\n",
			"(fp #b0 #b00 #b0)\n -> \n(fp #b0 #b00 #b0)\n",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing conversion-cache trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out FPA conversion trace executed", output.String())
	}
}

func TestSMTFPAConversionAndSideConditionDetailSourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, theory, family, sort, tags := smtFPAInternalizeTraceFixture(t, &output)
	tags.Enable("t_fpa")
	tags.Enable("t_fpa_detail")
	zero, err := m.MkFPAZeroOfSort(sort, false)
	astTestMust(t, err)
	atom, err := m.MkFamilyApp(family, ast.FPAOpEq, nil, []ast.ASTHandle{zero, zero})
	astTestMust(t, err)
	_, err = theory.convert(atom)
	astTestMust(t, err)
	_, err = theory.convert(zero)
	astTestMust(t, err)

	// The standalone converter path records the same symbolic-RM range side
	// condition consumed by theory_fpa::mk_side_conditions.
	theory.converter.SetOverrides(astfpa.FPA2BVOverrides{})
	roundingSort, err := m.RoundingModeSort()
	astTestMust(t, err)
	rounding, err := m.MkConst(astTestSymbol(t, m, "fpa-trace-rounding"), roundingSort)
	astTestMust(t, err)
	_, err = theory.convert(rounding)
	astTestMust(t, err)
	if extra, e := theory.converter.ExtraAssertions(); e != nil || len(extra) != 1 {
		t.Fatal("symbolic rounding conversion did not record its source side condition")
	}
	astTestMust(t, theory.flushSideConstraints())
	if tags.ShouldTrace("t_fpa_detail") {
		trace := output.String()
		for _, want := range []string{
			"-------- [t_fpa_detail] convert_atom src/smt/theory_fpa.cpp:297 ---------\nconverting atom: ",
			"-------- [t_fpa_detail] convert_term src/smt/theory_fpa.cpp:316 ---------\nterm: ",
			"\nconverted term: ",
			"-------- [t_fpa] mk_side_conditions src/smt/theory_fpa.cpp:404 ---------\nside condition: ",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing FPA conversion detail trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out FPA conversion detail trace executed", output.String())
	}
}

func TestSMTFPAVariableConstraintRelevantAndScopeSourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, theory, _, sort, tags := smtFPAInternalizeTraceFixture(t, &output)
	tags.Enable("t_fpa")
	tags.Enable("t_fpa_detail")
	constant, err := m.MkConst(astTestSymbol(t, m, "fpa-trace-variable"), sort)
	astTestMust(t, err)
	state, err := theory.State()
	astTestMust(t, err)
	node, err := state.MkEnode(constant, false, false, true)
	astTestMust(t, err)
	astTestMust(t, theory.ApplySortConstraint(node, sort))
	astTestMust(t, theory.Relevant(constant))
	wrapper, err := theory.wrap(constant)
	astTestMust(t, err)
	if state.context.enodes.get(wrapper) == nil {
		t.Fatal("relevant floating constant did not install its source wrapper relation")
	}
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, theory.assertConstraint(falsehood))
	astTestMust(t, theory.PushScope())
	astTestMust(t, theory.PopScopes(1))
	if tags.ShouldTrace("t_fpa") {
		trace := output.String()
		for _, want := range []string{
			"-------- [t_fpa] apply_sort_cnstr src/smt/theory_fpa.cpp:494 ---------\napply sort cnstr for: fpa-trace-variable\n",
			"-------- [t_fpa] attach_new_th_var src/smt/theory_fpa.cpp:422 ---------\nnew theory var: fpa-trace-variable := 0\n",
			"-------- [t_fpa] relevant_eh src/smt/theory_fpa.cpp:647 ---------\nrelevant_eh for: fpa-trace-variable\n",
			"-------- [t_fpa] relevant_eh src/smt/theory_fpa.cpp:679 ---------\nw/u eq: \n",
			"-------- [t_fpa_detail] assert_cnstr src/smt/theory_fpa.cpp:410 ---------\nasserting false\n",
			"-------- [t_fpa] pop_scope_eh src/smt/theory_fpa.cpp:625 ---------\npop 1; now 0\n",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing FPA lifecycle trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out FPA lifecycle trace executed", output.String())
	}
}

func TestSMTFPACallbackSourceTraceUsesEagerDefinitions(t *testing.T) {
	var output bytes.Buffer
	m, theory, family, sort, tags := smtFPAInternalizeTraceFixture(t, &output)
	tags.Enable("t_fpa")
	tags.Enable("t_fpa_detail")
	state, err := theory.State()
	astTestMust(t, err)
	constants := make([]ast.ASTHandle, 2)
	nodes := make([]*SMTEnode, 2)
	for i, name := range []string{"fpa-callback-left", "fpa-callback-right"} {
		constants[i], err = m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		nodes[i], err = state.MkEnode(constants[i], false, false, true)
		astTestMust(t, err)
		astTestMust(t, theory.ApplySortConstraint(nodes[i], sort))
	}
	left := nodes[0].TheoryVar(family)
	right := nodes[1].TheoryVar(family)
	astTestMust(t, theory.NewEquality(left, right))
	astTestMust(t, theory.NewDisequality(left, right))

	structural, err := m.MkEq(constants[0], constants[1])
	astTestMust(t, err)
	astTestMust(t, state.Internalize(structural, false))
	atom, err := m.MkFamilyApp(family, ast.FPAOpEq, nil, constants)
	astTestMust(t, err)
	matched, err := theory.InternalizeAtom(atom, false)
	astTestMust(t, err)
	if !matched {
		t.Fatal("floating-point callback atom was not internalized")
	}
	variable, found := state.BoolVar(atom)
	if !found {
		t.Fatal("floating-point callback atom has no Boolean variable")
	}
	astTestMust(t, theory.Assign(variable, true))

	if tags.ShouldTrace("t_fpa") {
		trace := output.String()
		for _, want := range []string{
			"-------- [t_fpa] new_eq_eh src/smt/theory_fpa.cpp:528 ---------\nnew eq: 0 = 1\n",
			"-------- [t_fpa_detail] new_eq_eh src/smt/theory_fpa.cpp:545 ---------\nxc = ",
			"\nyc = ",
			"-------- [t_fpa] new_diseq_eh src/smt/theory_fpa.cpp:572 ---------\nnew diseq: 0 != 1\n",
			"-------- [t_fpa] assign_eh src/smt/theory_fpa.cpp:634 ---------\nassign_eh for: ",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing FPA callback trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out FPA callback trace executed", output.String())
	}
}
