package pdr

// Source pdr_prop_solver.cpp:145-159 expands each bit-vector equality into
// owning per-bit literals through fresh extract, equality, and negation nodes.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPropSolverBitVectorExpansionReleasesNodesAfterAllocationFailure(t *testing.T) {
	for _, stage := range []string{"extract", "equality", "negation"} {
		t.Run(stage, func(t *testing.T) {
			m, context, _ := classifierContextFixture(t)
			var pins []ast.ASTRef
			t.Cleanup(func() { closureMust(t, closeASTRefs(m, pins)) })
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				closureMust(t, err)
				ref, err := m.NewRef(h)
				closureMust(t, err)
				pins = append(pins, ref)
				return h
			}
			truth := own(m.MkTrue())
			solver, err := NewPropSolver(m, context.owner.contexts, truth, "extract-allocation")
			closureMust(t, err)
			t.Cleanup(func() { closureMust(t, solver.Close()) })
			// Thirty-two bits require enough generated literal references to reach
			// another reference-page allocation before solver preparation begins.
			value := own(m.BitVectorNumeralUint64(0, 32))
			sort := own(m.GetSort(value))
			term := own(m.MkFreshConst("extract-allocation-x", sort))
			one := own(m.BitVectorNumeralUint64(1, 1))
			equality := own(m.MkEq(term, value))
			// Warm the expansion's shared declarations and slots, then release every
			// generated application so the targeted node must be constructed anew.
			_, expanded, _, err := solver.expandEqualities([]ast.ASTHandle{equality})
			closureMust(t, err)
			closureMust(t, closeASTRefs(m, expanded))
			extract, err := m.MkBitVectorExtract(0, 0, term)
			closureMust(t, err)
			ref, err := m.NewRef(extract)
			closureMust(t, err)
			own(m.GetDecl(extract))
			if stage != "extract" {
				own(extract, nil)
			}
			closureMust(t, m.DelRef(&ref))
			var bitEquality ast.ASTHandle
			var suffix string
			spareCount := 1
			switch stage {
			case "extract":
				termID, err := m.GetASTID(term)
				closureMust(t, err)
				suffix = fmt.Sprintf(" extract #%d\n", termID)
			case "equality":
				oneID, err := m.GetASTID(one)
				closureMust(t, err)
				extractID, err := m.GetASTID(extract)
				closureMust(t, err)
				suffix = fmt.Sprintf(" = #%d #%d\n", oneID, extractID)
				spareCount = 2
			case "negation":
				bitEquality = own(m.MkEq(one, extract))
				bitID, err := m.GetASTID(bitEquality)
				closureMust(t, err)
				suffix = fmt.Sprintf(" not #%d\n", bitID)
				spareCount = 3
			}
			writer := &projectedCubeAllocationWriter{manager: m, suffix: suffix}
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget(), BorrowedWriter: writer})
			closureMust(t, err)
			t.Cleanup(func() { closureMust(t, stream.Close()) })
			spares := make([]ast.ASTRef, spareCount)
			for i := range spares {
				spares[i], err = m.NewRef(truth)
				closureMust(t, err)
			}
			var fill []ast.ASTRef
			t.Cleanup(func() { closureMust(t, closeASTRefs(m, fill)) })
			for {
				m.Budget().FailAfter(0)
				ref, err := m.NewRef(truth)
				m.Budget().FailAfter(-1)
				if errors.Is(err, arena.ErrInjectedFailure) {
					break
				}
				closureMust(t, err)
				fill = append(fill, ref)
			}
			// Leave reusable slots for the numeral and earlier nodes in this bit.
			// A direct allocation profile may have no fill slots; the source event
			// still selects the failure without a fixed allocation ordinal.
			closureMust(t, closeASTRefs(m, spares))
			var result PropCheckResult
			func() {
				m.TraceStream = stream
				defer func() {
					m.TraceStream = nil
					m.Budget().FailAfter(-1)
				}()
				result, err = solver.CheckAssumptions([]ast.ASTHandle{equality}, false, false)
			}()
			if !writer.fired || result.Status != lbool.Undef || !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("missed public bit-vector expansion allocation failure", writer.fired, result.Status, err)
			}
			if len(solver.proxies) != 0 {
				t.Fatal("allocation failure escaped expansion into solver preparation", len(solver.proxies))
			}
			counts := func() [4]uint32 {
				stats := m.Stats()
				return [4]uint32{stats.LiveNodes, stats.LiveInfos, stats.LiveParameters, stats.LiveReferences}
			}
			before := counts()
			var recreated ast.ASTHandle
			switch stage {
			case "extract":
				recreated, err = m.MkBitVectorExtract(0, 0, term)
			case "equality":
				recreated, err = m.MkEq(one, extract)
			case "negation":
				recreated, err = m.MkNot(bitEquality)
			}
			closureMust(t, err)
			after := counts()
			ref, err = m.NewRef(recreated)
			closureMust(t, err)
			closureMust(t, m.DelRef(&ref))
			want := before
			want[0]++
			if after != want || counts() != before {
				t.Fatal("failed expansion left its fresh node unowned", before, after, counts())
			}
			result, err = solver.CheckAssumptions([]ast.ASTHandle{equality}, false, false)
			closureMust(t, err)
			if result.Status != lbool.True {
				t.Fatal("same-solver retry lost the original bit-vector equality", result.Status)
			}
			// The recovered solver must still expand the equality into its clear-bit
			// core, and the contradictory temporary formula must not survive the call.
			extract = own(m.MkBitVectorExtract(0, 0, term))
			setBit := own(m.MkEq(one, extract))
			clearBit := own(m.MkNot(setBit))
			result, err = solver.CheckAssumptionsAndFormula([]ast.ASTHandle{equality}, setBit, true, false)
			closureMust(t, err)
			if result.Status != lbool.False || len(result.Core) != 1 || result.Core[0] != clearBit {
				t.Fatal("recovered expansion lost its contradictory bit core", result.Status, result.Core)
			}
			result, err = solver.CheckAssumptions([]ast.ASTHandle{equality}, false, false)
			closureMust(t, err)
			if result.Status != lbool.True {
				t.Fatal("temporary contradictory bit survived scope rollback", result.Status)
			}
			// The fixture closes the solver, source owners and manager to zero budget;
			// the exact live-count check above detects an orphan before manager teardown.
		})
	}
}
