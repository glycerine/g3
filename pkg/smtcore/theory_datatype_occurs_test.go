package smtcore

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTDatatypeOccursEvidenceOrderAndScopeRecovery(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, unknownFirst := range []bool{false, true} {
			for _, length := range []int{1, 3} {
				t.Run(fmt.Sprint(proofs, "/", unknownFirst, "/", length), func(t *testing.T) {
					m, cfg := smtModelGeneratorFixture(t)
					if proofs {
						astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
					}
					solver, err := NewSMTSolver(m, nil, "QF_DT", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, solver.Close()) })
					k := solver.kernel
					k.preprocess = false
					k.smtParams.datatype.LazySplits = 2
					state := k.internalizer
					family, err := m.DatatypeFamilyID()
					astTestMust(t, err)
					dt := state.theories[family].(*SMTDatatypeTheory)
					var trace bytes.Buffer
					tags := diagnostic.NewTraceTagsWithWriter(&trace)
					tags.Enable("datatype")
					dt.trace = tags
					sort := smtDatatypePolicySort(t, m, true)
					ctors, err := m.DatatypeConstructors(sort)
					astTestMust(t, err)
					unknown, err := m.MkConst(astTestSymbol(t, m, "occurs-unknown"), sort)
					astTestMust(t, err)
					astTestPin(t, m, unknown)
					attach := func(h ast.ASTHandle) {
						t.Helper()
						astTestMust(t, state.Internalize(h, false))
						astTestMust(t, dt.ensureVariable(state.context.enodes.get(h)))
					}
					if unknownFirst {
						attach(unknown)
					}
					values := make([]ast.ASTHandle, length)
					constructors := make([]ast.ASTHandle, length)
					for i := range values {
						values[i], err = m.MkConst(astTestSymbol(t, m, fmt.Sprint("occurs-x", i)), sort)
						astTestMust(t, err)
						astTestPin(t, m, values[i])
						attach(values[i])
					}
					for i := range values {
						constructors[i], err = m.MkApp(ctors[1], []ast.ASTHandle{values[(i+1)%length]})
						astTestMust(t, err)
						astTestPin(t, m, constructors[i])
						attach(constructors[i])
					}
					if !unknownFirst {
						attach(unknown)
					}
					count := dt.NumVariables()
					for attempt := 0; attempt < 2; attempt++ {
						astTestMust(t, state.PushScope())
						for i := range values {
							eq, err := m.MkEq(values[i], constructors[i])
							astTestMust(t, err)
							smtDatatypeConflictFixture{m: m, k: k}.assert(t, eq)
						}
						if state.Inconsistent() {
							t.Fatal("cycle conflicted before final check")
						}
						splits := dt.statistics.splits
						trace.Reset()
						status, err := dt.FinalCheck()
						astTestMust(t, err)
						if status != SMTFinalCheckContinue || !state.Inconsistent() {
							t.Fatal("cycle escaped final check")
						}
						wantSplits := uint32(0)
						if unknownFirst {
							wantSplits = 1
						}
						if dt.statistics.splits-splits != wantSplits {
							t.Fatal("final check did not preserve root/split order")
						}
						conflict, _ := state.Conflict()
						owner, _ := conflict.Justification()
						ext, ok := owner.(*SMTExtTheoryConflictJustification)
						// occurs_check_explain pushes one equality per constructor
						// link it walks (length-1), plus the closing (app, root)
						// pair when the DFS closes the cycle away from its root.
						// Every pair must be a same-class equality.
						if !ok || len(ext.Literals) != 0 || len(ext.Equalities) < length-1 || len(ext.Equalities) > length {
							t.Fatal("cycle did not retain equality-only source evidence", owner)
						}
						for _, pr := range ext.Equalities {
							if pr.First.root != pr.Second.root {
								t.Fatal("cycle evidence pair is not a same-class equality")
							}
						}
						if tags.ShouldTrace("datatype") {
							last := -1
							for _, site := range []string{"[datatype] occurs_check src/smt/theory_datatype.cpp:524", "[datatype] occurs_check src/smt/theory_datatype.cpp:538", "[datatype] occurs_check_explain src/smt/theory_datatype.cpp:461", "[datatype] occurs_check src/smt/theory_datatype.cpp:556"} {
								i := strings.Index(trace.String(), site)
								if i <= last {
									t.Fatal("missing or reordered cycle diagnostic", site, trace.String())
								}
								last = i
							}
							if strings.Count(trace.String()[last:], "eq: #") != len(ext.Equalities) {
								t.Fatal("cycle diagnostic lost conflict equality evidence", trace.String()[last:])
							}
						} else if trace.Len() != 0 {
							t.Fatal("native cycle diagnostics were not erased")
						}
						for _, pair := range ext.Equalities {
							if pair.First == pair.Second || pair.First.root != pair.Second.root {
								t.Fatal("invalid cycle equality")
							}
						}
						statusSAT, err := k.resolveCheckConflict()
						astTestMust(t, err)
						if statusSAT != lbool.False {
							t.Fatal("cycle conflict did not resolve to false")
						}
						if proofs {
							proof, err := k.Proof()
							astTestMust(t, err)
							fact, err := m.GetProofFact(proof)
							astTestMust(t, err)
							if !m.IsFalse(fact) {
								t.Fatal("cycle proof did not conclude false")
							}
						}
						astTestMust(t, state.PopScopes(1))
						if state.Inconsistent() || dt.NumVariables() != count || dt.find.GetNumVars() != uint32(count) {
							t.Fatal("cycle scope did not restore variable lifecycle")
						}
						for v := uint32(0); v < dt.find.GetNumVars(); v++ {
							if !dt.find.IsRoot(v) {
								t.Fatal("datatype union survived pop")
							}
						}
					}
					// A completed traversal must not retain cycle marks from either
					// conflicting scope, and every constructor-free root gets a split.
					splits := dt.statistics.splits
					status, err := dt.FinalCheck()
					astTestMust(t, err)
					if status != SMTFinalCheckContinue || state.Inconsistent() || dt.statistics.splits-splits != uint32(length+1) {
						t.Fatal("final-check cleanup lost roots or retained a cycle")
					}
				})
			}
		}
	}
}

func TestSMTDatatypeAccessorVariableContract(t *testing.T) {
	m, _, solver := smtDatatypeSolver(t)
	state := solver.kernel.internalizer
	sort := smtDatatypePolicySort(t, m, true)
	ctors, err := m.DatatypeConstructors(sort)
	astTestMust(t, err)
	fields, err := m.DatatypeConstructorAccessors(ctors[1])
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "accessor-contract-x"), sort)
	astTestMust(t, err)
	selected, err := m.MkApp(fields[0], []ast.ASTHandle{x})
	astTestMust(t, err)
	astTestPin(t, m, selected)
	astTestMust(t, state.Internalize(selected, false))
	// Reinternalizing an existing handled accessor also permits an enode
	// without a theory variable, as smt_internalizer.cpp requires.
	astTestMust(t, state.Internalize(selected, false))
	family, err := m.DatatypeFamilyID()
	astTestMust(t, err)
	dt := state.theories[family].(*SMTDatatypeTheory)
	if !dt.IsAttachedToVariable(state.context.enodes.get(x)) || dt.IsAttachedToVariable(state.context.enodes.get(selected)) {
		t.Fatal("accessor attached a spurious result variable")
	}
	// Source context::has_quantifiers refers to sticky asserted-formula
	// state, independently of the internalized quantifier registry.
	astTestMust(t, solver.Push())
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	bound, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean},
		Names: []symbol.Symbol{astTestSymbol(t, m, "accessor-contract-bound")}, Body: bound,
	})
	astTestMust(t, err)
	astTestMust(t, solver.Assert(quantifier))
	if !solver.kernel.assertedHasQuantifiers {
		t.Fatal("asserted quantifier did not set context quantifier state")
	}
	astTestMust(t, dt.ApplySortConstraint(state.context.enodes.get(selected), sort))
	if !dt.IsAttachedToVariable(state.context.enodes.get(selected)) {
		t.Fatal("quantified sort constraint omitted infinite datatype variable")
	}
	astTestMust(t, solver.Pop(1))
	if dt.IsAttachedToVariable(state.context.enodes.get(selected)) {
		t.Fatal("scoped sort constraint retained its variable")
	}
	astTestMust(t, dt.ApplySortConstraint(state.context.enodes.get(selected), sort))
	if !dt.IsAttachedToVariable(state.context.enodes.get(selected)) {
		t.Fatal("pop incorrectly cleared sticky asserted quantifier state")
	}
}

func TestSMTDatatypeAccessorResultSortConstraints(t *testing.T) {
	for _, finite := range []bool{false, true} {
		t.Run(fmt.Sprint(finite), func(t *testing.T) {
			m, dt, _, finiteSort, _ := smtDatatypeSplitFixture(t)
			state, err := dt.State()
			astTestMust(t, err)
			rangeSort := finiteSort
			if !finite {
				rangeSort, err = m.MkUninterpretedSort(astTestSymbol(t, m, "accessor-range-U"), nil)
				astTestMust(t, err)
			}
			sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{
				Name: astTestSymbol(t, m, "accessor-range-container"),
				Constructors: []ast.DatatypeConstructorDefinition{
					{Name: astTestSymbol(t, m, "accessor-range-empty"), Recognizer: astTestSymbol(t, m, "accessor-range-is-empty")},
					{Name: astTestSymbol(t, m, "accessor-range-box"), Recognizer: astTestSymbol(t, m, "accessor-range-is-box"), Accessors: []ast.DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "accessor-range-field"), Range: rangeSort}}},
				},
			}})
			astTestMust(t, err)
			ctors, err := m.DatatypeConstructors(sorts[0])
			astTestMust(t, err)
			fields, err := m.DatatypeConstructorAccessors(ctors[1])
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "accessor-range-x"), sorts[0])
			astTestMust(t, err)
			selected, err := m.MkApp(fields[0], []ast.ASTHandle{x})
			astTestMust(t, err)
			astTestPin(t, m, selected)
			astTestMust(t, state.Internalize(selected, false))
			if !dt.IsAttachedToVariable(state.context.enodes.get(x)) {
				t.Fatal("accessor argument has no datatype variable")
			}
			if dt.IsAttachedToVariable(state.context.enodes.get(selected)) != finite {
				t.Fatal("accessor result did not follow its sort constraint")
			}
		})
	}
}
