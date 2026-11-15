package arith

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func pbTacticFixture(t *testing.T) *PB2BVTactic {
	t.Helper()
	base, m := fixDLFixture(t, false)
	r, err := NewPB2BVTactic(m, base.context, base.globals, nil, base.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	return r
}

func TestPB2BVEncodingTruthTables(t *testing.T) {
	for _, mode := range []string{"clauses", "cardinality", "bv", "split"} {
		for _, weights := range [][]int64{{1, 1, 1, 1}, {2, -3, 5, 1}, {3, 3, 2, 1}, {2, -2, 1, -1}} {
			for _, op := range []int32{2, 3, -1} {
				for bound := int64(-1); bound <= 7; bound++ {
					t.Run(fmt.Sprint(mode, "/", weights, "/", op, "/", bound), func(t *testing.T) {
						r := pbTacticFixture(t)
						m := r.manager
						if mode != "clauses" && mode != "split" {
							r.allClausesLimit = 0
						}
						if mode == "bv" {
							r.cardinalityLimit = 0
						}
						var vars, terms []ast.ASTHandle
						positions := make(map[ast.ASTHandle]int)
						for i, w := range weights {
							x := fixDLConst(t, m, true, fmt.Sprint("x", i))
							vars = append(vars, x)
							decl, e := m.GetDecl(x)
							equationMust(t, e)
							positions[decl] = i
							term := x
							if w != 1 {
								term = fixDLApp(t, m, 9, fixDLNumber(t, m, true, fmt.Sprint(w)), x)
							}
							terms = append(terms, term)
						}
						sum := fixDLApp(t, m, 6, terms...)
						rhs := fixDLNumber(t, m, true, fmt.Sprint(bound))
						var atom ast.ASTHandle
						if op < 0 {
							var e error
							atom, e = m.MkEq(sum, rhs)
							equationMust(t, e)
							fixDLPin(t, m, atom)
						} else {
							atom = fixDLApp(t, m, op, sum, rhs)
						}
						for _, positive := range []bool{false, true} {
							out, e := m.NewRef(ast.ASTHandle{})
							equationMust(t, e)
							equationMust(t, r.convert(atom, out, positive, mode == "split"))
							encoded, e := m.RefGet(out)
							equationMust(t, e)
							for mask := 0; mask < 1<<len(vars); mask++ {
								var total int64
								for i, w := range weights {
									if mask&(1<<i) != 0 {
										total += w
									}
								}
								want := total == bound
								if op == 2 {
									want = total <= bound
								} else if op == 3 {
									want = total >= bound
								}
								if !positive {
									want = !want
								}
								accepted := false
								for auxiliary := 0; auxiliary < 1<<len(r.temporaryInts); auxiliary++ {
									md, e := model.NewModel(m)
									equationMust(t, e)
									for _, entry := range r.entries {
										value := false
										if i, ok := positions[entry.declaration]; ok {
											value = mask&(1<<i) != 0
										} else {
											for i, v := range r.temporaryInts {
												d, e := m.GetDecl(v)
												equationMust(t, e)
												if d == entry.declaration {
													value = auxiliary&(1<<i) != 0
													break
												}
											}
										}
										h, e := m.MkFalse()
										if value {
											h, e = m.MkTrue()
										}
										equationMust(t, e)
										d, e := m.GetDecl(entry.positive)
										equationMust(t, e)
										equationMust(t, md.CoreField.RegisterConstant(d, h))
									}
									ev, e := model.NewModelEvaluator(&md.CoreField, r.services)
									equationMust(t, e)
									value, e := ev.EvaluateValue(encoded)
									equationMust(t, e)
									h, e := m.RefGet(value)
									equationMust(t, e)
									if !m.IsTrue(h) && !m.IsFalse(h) {
										t.Fatal("nonground Boolean encoding", h)
									}
									accepted = accepted || m.IsTrue(h)
									equationMust(t, m.DelRef(&value))
									equationMust(t, ev.Close())
									equationMust(t, md.Close())
								}
								if accepted != want {
									t.Fatal("encoding truth", mask, total, bound, positive, accepted, want)
								}
							}
							equationMust(t, m.DelRef(&out))
						}
					})
				}
			}
		}
	}
}

// A numeral on the left is mirrored onto the right before encoding: z3 4.7.1
// turned (= 3 x) into the unit x and (<= 3 x) into x (AUDIT.md
// tactic_arith_03-F1).
func TestPB2BVNumeralOnLeft(t *testing.T) {
	for _, tc := range []struct {
		op    int32
		bound string
		want  bool
	}{{-1, "3", false}, {2, "3", false}, {3, "2", true}, {2, "-1", true}} {
		for _, positive := range []bool{true, false} {
			r := pbTacticFixture(t)
			m := r.manager
			x := fixDLConst(t, m, true, "x")
			c := fixDLNumber(t, m, true, tc.bound)
			var atom ast.ASTHandle
			if tc.op < 0 {
				var e error
				atom, e = m.MkEq(c, x)
				equationMust(t, e)
				fixDLPin(t, m, atom)
			} else {
				atom = fixDLApp(t, m, tc.op, c, x)
			}
			out, e := m.NewRef(ast.ASTHandle{})
			equationMust(t, e)
			equationMust(t, r.convert(atom, out, positive, false))
			h, e := m.RefGet(out)
			equationMust(t, e)
			want := tc.want == positive
			if (want && !m.IsTrue(h)) || (!want && !m.IsFalse(h)) {
				t.Fatal("numeral-left encoding", tc.op, tc.bound, positive, h, want)
			}
			equationMust(t, m.DelRef(&out))
		}
	}
}

func TestPB2BVCoefficientNormalization(t *testing.T) {
	r := pbTacticFixture(t)
	m := r.manager
	x := fixDLConst(t, m, true, "x")
	y := fixDLConst(t, m, true, "y")
	p := pbPolynomial{{weight: *big.NewRat(12, 1), literal: pbLiteral{x, false}}, {weight: *big.NewRat(6, 1), literal: pbLiteral{y, false}}}
	c := big.NewRat(10, 1)
	scope := &pbEncodingScope{owner: r}
	_, err := r.mkPBC(p, c, false, scope)
	equationMust(t, err)
	if p[0].weight.RatString() != "5" || p[1].weight.RatString() != "3" || c.RatString() != "5" {
		t.Fatal("trim before GCD and ceil", p, c)
	}
	equationMust(t, scope.close())
}

func TestPB2BVTacticAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "clauses", "cardinality", "bv", "split", "cleanup"} {
		success := false
		for stop := int64(0); stop < 1024; stop++ {
			t.Run(fmt.Sprint(phase, "/", stop), func(t *testing.T) {
				r := pbTacticFixture(t)
				m := r.manager
				if phase == "constructor" {
					m.Budget().FailAfter(stop)
					fresh, err := NewPB2BVTactic(m, r.context, r.globals, nil, r.services)
					m.Budget().FailAfter(-1)
					if fresh != nil {
						equationMust(t, fresh.Close())
					}
					if err == nil {
						success = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal(err)
					}
					return
				}
				if phase == "cardinality" || phase == "bv" {
					r.allClausesLimit = 0
				}
				if phase == "bv" {
					r.cardinalityLimit = 0
				}
				weights := []string{"3", "3", "2", "1"}
				bound := "3"
				if phase == "cardinality" {
					weights = []string{"1", "1", "1", "1"}
					bound = "2"
				}
				var forms, terms []ast.ASTHandle
				for i, w := range weights {
					x := fixDLConst(t, m, true, fmt.Sprint("x", i))
					forms = append(forms, diffNeqTestBounds(t, m, x, "1")...)
					term := x
					if w != "1" {
						term = fixDLApp(t, m, 9, fixDLNumber(t, m, true, w), x)
					}
					terms = append(terms, term)
				}
				forms = append(forms, fixDLApp(t, m, 3, fixDLApp(t, m, 6, terms...), fixDLNumber(t, m, true, bound)))
				goal := fixDLGoal(t, m, false, true, true, forms...)
				boolean, e := m.MkBoolSort()
				equationMust(t, e)
				name, e := m.Symbols.Intern("original-bound")
				equationMust(t, e)
				label, e := m.MkConst(name, boolean)
				equationMust(t, e)
				fixDLPin(t, m, label)
				dep, e := m.DependencyLeaf(label)
				equationMust(t, e)
				equationMust(t, goal.Update(0, forms[0], ast.ASTHandle{}, dep))
				var result tactic.TacticResult
				t.Cleanup(func() { equationMust(t, result.Close()) })
				if phase == "cleanup" {
					equationMust(t, r.Apply(goal, &result))
				}
				m.Budget().FailAfter(stop)
				var err error
				if phase == "cleanup" {
					err = r.Cleanup()
				} else {
					err = r.Apply(goal, &result)
				}
				m.Budget().FailAfter(-1)
				if err == nil {
					success = true
				} else if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal(err)
				}
			})
			if success {
				break
			}
		}
		if !success {
			t.Fatal("no successful allocation frontier", phase)
		}
	}
}

func TestPB2BVProbeRefusalsParametersAndMemory(t *testing.T) {
	r := pbTacticFixture(t)
	m := r.manager
	x := fixDLConst(t, m, true, "x")
	y := fixDLConst(t, m, true, "y")
	forms := append(diffNeqTestBounds(t, m, x, "1"), diffNeqTestBounds(t, m, y, "1")...)
	goal := fixDLGoal(t, m, false, false, false, forms...)
	yes, err := IsPseudoBooleanGoal(goal, r.context, r.globals)
	equationMust(t, err)
	if !yes {
		t.Fatal("bounded 0/1 goal refused")
	}
	probe, err := NewIsPBProbe(m.Budget(), r.context, r.globals)
	equationMust(t, err)
	// Conditional and composed probes retain the concrete arithmetic probe
	// through the source virtual interface and release it on their own teardown.
	not, err := tactic.NewNotProbe(m.Budget(), probe)
	equationMust(t, err)
	equationMust(t, not.IncRef())
	t.Cleanup(func() { equationMust(t, not.DecRef()) })
	value, err := not.Eval(goal)
	equationMust(t, err)
	if value != 0 {
		t.Fatal("composed is-pb probe", value)
	}
	selected, err := tactic.NewCondTactic(probe, tactic.NewSkipTactic(), tactic.NewFailTactic())
	equationMust(t, err)
	equationMust(t, selected.IncRef())
	t.Cleanup(func() { equationMust(t, selected.DecRef()) })
	var routed tactic.TacticResult
	t.Cleanup(func() { equationMust(t, routed.Close()) })
	equationMust(t, selected.Apply(goal, &routed))
	bad := fixDLGoal(t, m, false, false, false, fixDLApp(t, m, 2, x, fixDLNumber(t, m, true, "2")))
	yes, err = IsPseudoBooleanGoal(bad, r.context, r.globals)
	equationMust(t, err)
	if yes {
		t.Fatal("unbounded integer accepted by is-pb")
	}
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	if err = r.Apply(bad, &result); err == nil || err.Error() != "goal is in a fragment unsupported by pb2bv. Offending expression: x" {
		t.Fatal("unsupported diagnostic", err)
	}
	p, err := r.context.NewValue()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.context.Del(&p)) })
	key := "pb2bv_all_clauses_limit"
	equationMust(t, r.context.SetUIntName(&p, &key, 1))
	equationMust(t, r.UpdateParams(&p))
	flat, err := r.boolean.Flat()
	equationMust(t, err)
	if !flat {
		t.Fatal("live source update must restore default flat")
	}
	equationMust(t, r.Cleanup())
	flat, err = r.boolean.Flat()
	equationMust(t, err)
	if flat || r.allClausesLimit != 1 {
		t.Fatal("constructor flags and updated snapshot after cleanup", flat, r.allClausesLimit)
	}
	block, err := r.services.Memory.Allocate(16)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.services.Memory.Deallocate(&block)) })
	r.maxMemory = 0
	if err = r.checkpoint(); err == nil || err.Error() != tactic.TacticMaxMemoryMessage {
		t.Fatal("source memory checkpoint", err)
	}
	equationMust(t, r.Cleanup())
	equationMust(t, r.Apply(goal, &result))
	copy := *r
	if err = copy.Reset(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied tactic", err)
	}
}

func TestPB2BVRootTraceAndProofGuard(t *testing.T) {
	r := pbTacticFixture(t)
	m := r.manager
	x := fixDLConst(t, m, true, "x")
	goal := fixDLGoal(t, m, false, false, false, diffNeqTestBounds(t, m, x, "1")...)
	var before, boundText, after, trace bytes.Buffer
	equationMust(t, goal.Display(&before))
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("pb2bv")
	equationMust(t, r.BindTrace(tags))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, r.Apply(goal, &result))
	if tags.ShouldTrace("pb2bv") {
		equationMust(t, r.bounds.Display(&boundText))
		equationMust(t, goal.Display(&after))
		record := func(function string, line int, body string) string {
			return fmt.Sprintf("-------- [pb2bv] %s src/tactic/arith/pb2bv_tactic.cpp:%d ---------\n%s------------------------------------------------\n", function, line, body)
		}
		want := record("operator()", 893, before.String()) + record("operator()", 909, boundText.String()) + record("convert", 704, "discarding:\n(>= x 0)\n") + record("convert", 691, "discarding:\n(<= x 1)\n") + record("operator()", 967, after.String())
		if trace.String() != want {
			t.Fatal("root source trace", trace.String(), want)
		}
	} else if trace.Len() != 0 {
		t.Fatal("erased trace emitted output")
	}
	equationMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	proofGoal := fixDLGoal(t, m, true, false, false)
	if err := r.Apply(proofGoal, &result); err == nil {
		t.Fatal("proof guard ignored")
	}
}

func TestPB2BVCPPEncodingShapes(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	for _, mode := range []string{"clauses-split", "cardinality", "bv"} {
		t.Run(mode, func(t *testing.T) {
			r := pbTacticFixture(t)
			m := r.manager
			p, err := r.context.NewValue()
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, r.context.Del(&p)) })
			all, card := uint32(8), ^uint32(0)
			weights := []int64{3, 3, 2, 1}
			bound := int64(3)
			if mode == "cardinality" {
				all = 0
				weights = []int64{1, 1, 1, 1}
				bound = 2
			}
			if mode == "bv" {
				all, card = 0, 0
				weights = []int64{7, 5, 3, 2}
				bound = 17
			}
			key := "pb2bv_all_clauses_limit"
			equationMust(t, r.context.SetUIntName(&p, &key, all))
			key = "pb2bv_cardinality_limit"
			equationMust(t, r.context.SetUIntName(&p, &key, card))
			equationMust(t, r.UpdateParams(&p))
			var source strings.Builder
			source.WriteString("(set-logic ALL)\n")
			var forms, terms []ast.ASTHandle
			for i, w := range weights {
				name := fmt.Sprint("x", i)
				x := fixDLConst(t, m, true, name)
				fmt.Fprintf(&source, "(declare-const %s Int)(assert (>= %s 0))(assert (<= %s 1))\n", name, name, name)
				forms = append(forms, diffNeqTestBounds(t, m, x, "1")...)
				term := x
				if w != 1 {
					term = fixDLApp(t, m, 9, fixDLNumber(t, m, true, fmt.Sprint(w)), x)
				}
				terms = append(terms, term)
			}
			atom := fixDLApp(t, m, 3, fixDLApp(t, m, 6, terms...), fixDLNumber(t, m, true, fmt.Sprint(bound)))
			forms = append(forms, atom)
			source.WriteString("(assert ")
			equationMust(t, r.bounds.writeExpr(&source, atom))
			source.WriteString(")\n")
			fmt.Fprintf(&source, "(apply (using-params pb2bv :pb2bv_all_clauses_limit %d :pb2bv_cardinality_limit %d))", all, card)
			goal := fixDLGoal(t, m, false, false, false, forms...)
			var result tactic.TacticResult
			t.Cleanup(func() { equationMust(t, result.Close()) })
			equationMust(t, r.Apply(goal, &result))
			var actual bytes.Buffer
			actual.WriteString("(goals (goal ")
			size, e := goal.Size()
			equationMust(t, e)
			for i := uint32(0); i < size; i++ {
				form, e := goal.Form(i)
				equationMust(t, e)
				equationMust(t, r.bounds.writeExpr(&actual, form))
				actual.WriteByte(' ')
			}
			depth, e := goal.Depth()
			equationMust(t, e)
			fmt.Fprintf(&actual, ":precision precise :depth %d))", depth)
			cmd := exec.Command(reference, "-in")
			cmd.Stdin = strings.NewReader(source.String())
			native, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatal(e, string(native))
			}
			canonical := func(s string) string {
				seen := make(map[string]string)
				s = regexp.MustCompile(`k![0-9]+`).ReplaceAllStringFunc(s, func(name string) string {
					if existing, ok := seen[name]; ok {
						return existing
					}
					next := fmt.Sprint("helper", len(seen))
					seen[name] = next
					return next
				})
				return strings.Join(strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(s)), " ")
			}
			if canonical(actual.String()) != canonical(string(native)) {
				t.Fatalf("encoding shape mismatch\nGo: %s\nC++: %s", actual.String(), native)
			}
		})
	}
}
