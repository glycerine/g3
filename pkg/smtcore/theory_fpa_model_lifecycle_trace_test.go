package smtcore

// Source: src/smt/theory_fpa.cpp:723,730-799.

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func smtFPAModelLifecycleTraceFixture(t *testing.T, writer io.Writer) (*ast.ASTManager, *SMTFloatingPointTheory, *SMTModelGenerator, ast.FamilyID, ast.ASTHandle, *diagnostic.TraceTags) {
	t.Helper()
	owner, config := smtModelGeneratorFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("t_fpa")
	tags.Enable("t_fpa_detail")
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
	config.Trace = tags
	solver, err := NewSMTSolver(m, nil, "QF_FPBV", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	theory := solver.kernel.internalizer.theories[family].(*SMTFloatingPointTheory)
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	astTestMust(t, generator.SetContext(solver.kernel.internalizer.context))
	astTestMust(t, generator.InitializeModel(config))
	t.Cleanup(func() {
		astTestMust(t, generator.Close())
		astTestMust(t, solver.Close())
		astTestMust(t, m.Close())
	})
	return m, theory, generator, family, sort, tags
}

func TestSMTFPAModelLifecycleAndDependencySourceTrace(t *testing.T) {
	var output bytes.Buffer
	m, theory, generator, family, sort, tags := smtFPAModelLifecycleTraceFixture(t, &output)
	state, err := theory.State()
	astTestMust(t, err)
	fields := make([]ast.ASTHandle, 3)
	for i, width := range []uint32{1, 2, 1} {
		fieldSort, sortErr := m.BitVectorSort(width)
		astTestMust(t, sortErr)
		fields[i], err = m.MkConst(astTestSymbol(t, m, []string{"fpa-model-sign", "fpa-model-exponent", "fpa-model-significand"}[i]), fieldSort)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(fields[i], false))
	}
	term, err := m.MkFamilyApp(family, ast.FPAOpFP, nil, fields)
	astTestMust(t, err)
	node, err := state.MkEnode(term, false, false, true)
	astTestMust(t, err)
	astTestMust(t, theory.ApplySortConstraint(node, sort))
	constant, err := m.MkConst(astTestSymbol(t, m, "fpa-model-trace-x"), sort)
	astTestMust(t, err)
	constantNode, err := state.MkEnode(constant, false, false, true)
	astTestMust(t, err)
	astTestMust(t, theory.ApplySortConstraint(constantNode, sort))
	astTestMust(t, theory.InitializeModel(generator))
	procedure, err := theory.ModelValue(node, generator)
	astTestMust(t, err)
	if len(procedure.Dependencies()) != 3 {
		t.Fatal("direct FP model dependency shape")
	}
	fallback, err := theory.ModelValue(constantNode, generator)
	astTestMust(t, err)
	if len(fallback.Dependencies()) != 0 {
		t.Fatal("uninternalized wrapper did not select fallback")
	}
	frame, err := theory.frame()
	astTestMust(t, err)
	var frameErr error
	defer func() { frame.release(&frameErr); astTestMust(t, frameErr) }()
	wrapper, err := theory.wrap(frame, constant)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(wrapper, false))
	wrapped, err := theory.ModelValue(constantNode, generator)
	astTestMust(t, err)
	if len(wrapped.Dependencies()) != 1 {
		t.Fatal("internalized wrapper did not become the model dependency")
	}
	roundingSort, err := m.RoundingModeSort()
	astTestMust(t, err)
	bitsSort, err := m.BitVectorSort(3)
	astTestMust(t, err)
	bits, err := m.MkConst(astTestSymbol(t, m, "fpa-model-rounding-bits"), bitsSort)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(bits, false))
	rounding, err := m.MkFamilyApp(family, ast.FPAOpBV2RM, nil, []ast.ASTHandle{bits})
	astTestMust(t, err)
	roundingNode, err := state.MkEnode(rounding, false, false, true)
	astTestMust(t, err)
	astTestMust(t, theory.ApplySortConstraint(roundingNode, roundingSort))
	roundingProcedure, err := theory.ModelValue(roundingNode, generator)
	astTestMust(t, err)
	if len(roundingProcedure.Dependencies()) != 1 {
		t.Fatal("rounding conversion dependency shape")
	}
	if tags.ShouldTrace("t_fpa") {
		trace := output.String()
		for _, want := range []string{
			"-------- [t_fpa] init_model src/smt/theory_fpa.cpp:723 ---------\ninitializing model\n",
			"-------- [t_fpa] mk_value src/smt/theory_fpa.cpp:730 ---------\nmk_value for: (fp fpa-model-sign fpa-model-exponent fpa-model-significand) (sort (_ FloatingPoint 2 2))\n",
			"-------- [t_fpa_detail] mk_value src/smt/theory_fpa.cpp:770 ---------\nDepends on: fpa-model-sign eq. cls. #",
			"mk_value for: fpa-model-trace-x (sort (_ FloatingPoint 2 2))\n",
			"-------- [t_fpa_detail] mk_value src/smt/theory_fpa.cpp:752 ---------\nModel dependency not internalized: ",
			"(owner is internalized)\n",
			"-------- [t_fpa_detail] mk_value src/smt/theory_fpa.cpp:798 ---------\nDepends on: ",
			"-------- [t_fpa_detail] mk_value src/smt/theory_fpa.cpp:782 ---------\nDepends on: fpa-model-rounding-bits eq. cls. #",
		} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing FPA model lifecycle trace %q in %q", want, trace)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("compiled-out FPA model lifecycle trace executed", output.String())
	}
}

func TestSMTFPAModelLifecycleTraceStreamRefusalDoesNotStop(t *testing.T) {
	_, theory, generator, _, _, _ := smtFPAModelLifecycleTraceFixture(t, smtFPAInternalizeRefusingWriter{})
	astTestMust(t, theory.InitializeModel(generator))
}
