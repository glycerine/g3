package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTDatatypeEqualityAxiomSourceBranchesAndRollback(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, guard := range []string{"null", "true", "false", "undefined", "constant-true", "constant-false"} {
			t.Run(fmt.Sprint(proofs, "/", guard), func(t *testing.T) {
				m, dt, x, sort, _ := smtDatatypeSplitFixture(t)
				if proofs {
					astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				s, err := dt.State()
				astTestMust(t, err)
				y, err := m.MkConst(astTestSymbol(t, m, "datatype-equality-y"), sort)
				astTestMust(t, err)
				astTestPin(t, m, y)
				astTestMust(t, s.Internalize(y, false))
				boolean, err := m.MkBoolSort()
				astTestMust(t, err)
				p, err := m.MkConst(astTestSymbol(t, m, "datatype-equality-guard"), boolean)
				astTestMust(t, err)
				astTestPin(t, m, p)
				astTestMust(t, s.Internalize(p, true))
				pLiteral, err := s.Literal(p)
				astTestMust(t, err)
				left, right := s.context.enodes.get(x), s.context.enodes.get(y)
				for attempt := 0; attempt < 2; attempt++ {
					astTestMust(t, s.PushScope())
					antecedent := pLiteral
					switch guard {
					case "null":
						antecedent = SMTNullLiteral
					case "true":
						astTestMust(t, s.AssignLiteral(pLiteral, NewSMTBooleanAxiom()))
					case "false":
						astTestMust(t, s.AssignLiteral(pLiteral.Negated(), NewSMTBooleanAxiom()))
					case "constant-true":
						antecedent = SMTTrueLiteral
					case "constant-false":
						antecedent = SMTFalseLiteral
					}
					clauses, bools, owners := len(s.clauses), len(s.boolExpr), len(s.ownedJustifications)
					astTestMust(t, dt.equalityAxiom(x, y, antecedent))
					direct := !proofs && (guard == "null" || guard == "true" || guard == "constant-true")
					if direct {
						if left.root == right.root || len(s.clauses) != clauses || len(s.boolExpr) != bools {
							t.Fatal("queued datatype equality merged eagerly or created SAT encoding")
						}
						if guard != "null" {
							if len(s.ownedJustifications) != owners+1 {
								t.Fatal("missing scoped equality owner")
							}
							j, ok := s.ownedJustifications[owners].(*SMTExtTheoryEqualityPropagationJustification)
							if !ok || j.FromTheory() != SMTNullTheoryID || j.theory != dt.FamilyID() || len(j.Literals) != 1 || j.Literals[0] != antecedent || len(j.Equalities) != 0 || j.Left != left || j.Right != right {
								t.Fatal("datatype equality lost its source origin or evidence")
							}
						} else if len(s.ownedJustifications) != owners {
							t.Fatal("unconditional equality allocated a conditional owner")
						}
						astTestMust(t, s.PropagateTheoryAssignments())
						if left.root != right.root {
							t.Fatal("source equality phase did not merge queued datatype equality")
						}
						events := s.PendingTheoryEqualities()
						found := false
						for _, event := range events {
							if event.Theory == dt.FamilyID() {
								found = true
							}
						}
						if !found {
							t.Fatal("datatype equality was not returned to its originating theory")
						}
					} else {
						eq, err := s.mkEqualityAtom(x, y)
						astTestMust(t, err)
						if _, found := s.boolVar.lookup(eq); !found || !s.isRelevant(eq) {
							t.Fatal("clause equality was not internalized and relevant")
						}
						if guard == "undefined" && !proofs && !s.isRelevant(p) {
							t.Fatal("nontrue antecedent was not relevant")
						}
						if proofs && guard == "undefined" && s.isRelevant(p) {
							t.Fatal("proof branch unnecessarily marked its antecedent")
						}
					}
					_, err = s.PropagateCore()
					astTestMust(t, err)
					merged := guard == "null" || guard == "true" || guard == "constant-true"
					if (left.root == right.root) != merged || s.Inconsistent() {
						t.Fatal("wrong guarded equality result", guard)
					}
					if guard == "undefined" {
						astTestMust(t, s.AssignLiteral(pLiteral, NewSMTBooleanAxiom()))
						_, err = s.PropagateCore()
						astTestMust(t, err)
						if left.root != right.root {
							t.Fatal("guard activation did not merge the equality")
						}
					}
					astTestMust(t, s.PopScopes(1))
					if left.root == right.root || len(s.ownedJustifications) != owners {
						t.Fatal("scoped equality owner or merge survived pop")
					}
				}
			})
		}
	}
}

func TestSMTDatatypeConstructorAccessorAxiomsAndCounters(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			f := newSMTDatatypeConflictFixture(t, proofs)
			s := f.k.internalizer
			// The fixture internalizes two one-field constructors. Neither
			// constructor creates a synthetic recognizer or reconstruction.
			if f.dt.statistics.accessorAxioms != 2 || f.dt.statistics.constructorAxioms != 0 {
				t.Fatal("constructor publication changed source counters")
			}
			for _, expression := range s.boolExpr {
				if expression.IsNull() {
					continue
				}
				op, own, err := f.dt.operation(expression)
				astTestMust(t, err)
				if !own || op != ast.ASTDatatypeIs && op != ast.ASTDatatypeRecognizer {
					continue
				}
				arg, err := f.m.GetArg(expression, 0)
				astTestMust(t, err)
				if arg == f.left || arg == f.right {
					t.Fatal("constructor synthesized a recognizer")
				}
			}
			ctor, _, err := f.m.DatatypeConstructorApp(f.left)
			astTestMust(t, err)
			fields, err := f.m.DatatypeConstructorAccessors(ctor)
			astTestMust(t, err)
			selected, err := f.m.MkApp(fields[0], []ast.ASTHandle{f.left})
			astTestMust(t, err)
			argument, err := f.m.GetArg(f.left, 0)
			astTestMust(t, err)
			if !proofs && s.context.enodes.get(selected).root == s.context.enodes.get(argument).root {
				t.Fatal("accessor axiom merged before the source equality phase")
			}
			_, err = s.PropagateCore()
			astTestMust(t, err)
			if s.context.enodes.get(selected).root != s.context.enodes.get(argument).root {
				t.Fatal("accessor axiom was not propagated")
			}
			astTestMust(t, s.PushScope())
			proof := ast.ASTHandle{}
			if proofs {
				proof, err = f.m.MkProofAsserted(f.negative)
				astTestMust(t, err)
			}
			astTestMust(t, s.Assert(f.negative, proof, 0))
			_, err = s.PropagateCore()
			astTestMust(t, err)
			if value, err := s.Assignment(f.recognizer); err != nil || value != lbool.False {
				t.Fatal("recognizer assignment")
			}
			astTestMust(t, s.PopScopes(1))
		})
	}
}

func TestSMTDatatypeAccessorCounterCountsConstructorCalls(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			if proofs {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			solver, err := NewSMTSolver(m, nil, "QF_DT", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			s := solver.kernel.internalizer
			solver.kernel.smtParams.datatype.LazySplits = 2
			uSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "counter-U"), nil)
			astTestMust(t, err)
			sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{
				Name: astTestSymbol(t, m, "counter-choice"),
				Constructors: []ast.DatatypeConstructorDefinition{
					{Name: astTestSymbol(t, m, "counter-empty"), Recognizer: astTestSymbol(t, m, "counter-is-empty")},
					{Name: astTestSymbol(t, m, "counter-pair"), Recognizer: astTestSymbol(t, m, "counter-is-pair"), Accessors: []ast.DatatypeAccessorDefinition{
						{Name: astTestSymbol(t, m, "counter-first"), Range: uSort},
						{Name: astTestSymbol(t, m, "counter-second"), Range: uSort},
					}},
				},
			}})
			astTestMust(t, err)
			ctors, err := m.DatatypeConstructors(sorts[0])
			astTestMust(t, err)
			u, err := m.MkConst(astTestSymbol(t, m, "counter-u"), uSort)
			astTestMust(t, err)
			v, err := m.MkConst(astTestSymbol(t, m, "counter-v"), uSort)
			astTestMust(t, err)
			pair, err := m.MkApp(ctors[1], []ast.ASTHandle{u, v})
			astTestMust(t, err)
			empty, err := m.MkApp(ctors[0], nil)
			astTestMust(t, err)
			astTestPin(t, m, pair)
			astTestPin(t, m, empty)
			family, err := m.DatatypeFamilyID()
			astTestMust(t, err)
			dt := s.theories[family].(*SMTDatatypeTheory)
			bools := len(s.boolExpr)
			astTestMust(t, s.Internalize(pair, false))
			if dt.statistics.accessorAxioms != 1 {
				t.Fatal("accessor counter counted fields instead of constructor calls")
			}
			astTestMust(t, s.Internalize(empty, false))
			if dt.statistics.accessorAxioms != 2 || dt.statistics.constructorAxioms != 0 {
				t.Fatal("nullary constructor lost its accessor call or added reconstruction")
			}
			want := 0
			if proofs {
				want = 2
			}
			if len(s.boolExpr)-bools != want {
				t.Fatal("constructor created extra recognizer/equality atoms")
			}
		})
	}
}
