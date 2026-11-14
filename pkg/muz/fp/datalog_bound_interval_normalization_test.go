package fp

// Source: dl_mk_filter_rules.cpp and dl_mk_simple_joins.cpp normalize repeated
// variables/constants and long positive bodies before relational compilation.

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogBoundIntervalNormalizesThreePositiveTails(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, mode := range []struct {
			name       string
			widen, all bool
		}{{"ordinary", false, false}, {"widen", true, false}, {"widen-all", true, true}} {
			for _, shape := range []string{"match", "empty-join", "empty-repetition", "empty-constant"} {
				t.Run(kinds[0]+"/"+mode.name+"/"+shape, func(t *testing.T) {
					empty := shape != "match"
					f := newBoundIntervalPublicFixture(t, kinds)
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", strconv.FormatBool(mode.widen)))
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", strconv.FormatBool(mode.all)))
					p, r, s, q := f.declare("normalize-P", 2), f.declare("normalize-R", 2), f.declare("normalize-S", 2), f.declare("normalize-Q", 2)
					if shape == "empty-repetition" {
						f.fact(p, 0, 99)
						f.fact(p, 2, 100)
					} else {
						f.fact(p, 0, 0)
						f.fact(p, 2, 2)
					}
					if shape == "empty-join" {
						f.fact(r, 99, 10)
						f.fact(r, 99, 20)
					} else {
						f.fact(r, 0, 10)
						f.fact(r, 2, 20)
					}
					if shape == "empty-constant" {
						f.fact(s, 15, 8)
					} else {
						f.fact(s, 15, 7)
					}
					x, y := f.hold(f.m.MkVar(0, f.integer)), f.hold(f.m.MkVar(1, f.integer))
					body := f.hold(f.m.MkAnd3(f.app(p, x, x), f.app(r, x, y), f.app(s, y, f.numeral(7))))
					head := f.app(q, x, y)
					fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(body, head)), "normalize-three-positive"))

					originalSet := f.e.set
					originalRule, err := originalSet.Rule(0)
					fragmentMust(t, err)
					defer func() { fragmentMust(t, originalRule.Close()) }()
					originalText, err := originalRule.FormulaString()
					fragmentMust(t, err)
					owner := f.e.boundIntervalDatalog
					entries := make(map[ast.ASTHandle]*boundIntervalDatalogEntry, 4)
					for declaration, entry := range owner.entries {
						entries[declaration] = entry
					}
					checkOriginal := func() {
						t.Helper()
						if f.e.set != originalSet || f.e.boundIntervalDatalog != owner || owner.rules != originalSet || len(f.e.predicates) != 4 || len(owner.entries) != 4 || len(owner.order) != 4 {
							t.Fatal("temporary normalization changed the persistent program or predicate owners")
						}
						for declaration, entry := range entries {
							if owner.entries[declaration] != entry {
								t.Fatal("normalization replaced a persistent seed owner")
							}
						}
						count, cause := originalSet.Len()
						fragmentMust(t, cause)
						rule, cause := originalSet.Rule(0)
						fragmentMust(t, cause)
						defer func() { fragmentMust(t, rule.Close()) }()
						positive, cause := rule.PositiveTailSize()
						fragmentMust(t, cause)
						name, cause := rule.Name()
						fragmentMust(t, cause)
						text, cause := rule.FormulaString()
						fragmentMust(t, cause)
						if count != 1 || positive != 3 || name != "normalize-three-positive" || text != originalText {
							t.Fatal("query rewrote or renamed its original three-tail rule", count, positive, name)
						}
					}
					checkOriginal()
					var answer ast.ASTRef
					for run := 0; run < 2; run++ {
						status, cause := f.e.QueryPredicate(q)
						fragmentMust(t, cause)
						// C++ projection can turn an empty intermediate full, after
						// which saturation keeps the imprecise relation.
						want := lbool.Undef
						if status != want {
							t.Fatal("normalized three-tail query", status, want)
						}
						answer = f.own(f.e.Answer(q))
						// Ordinary union of an empty projected intermediate keeps the
						// reset payload. Widening retains the C++ endpoint state from
						// the active source relation.
						for _, values := range [][]int{{-1, 15}, {0, 15}, {1, 15}, {2, 15}, {3, 15}, {1, 14}, {1, 16}} {
							want := true
							if mode.widen {
								want = false
								switch shape {
								case "match":
									want = values[0] >= 0 && values[0] <= 2 && values[1] == 15
								case "empty-join":
									want = values[0] == 99 && values[1] >= 10 && values[1] <= 20
								case "empty-repetition", "empty-constant":
									want = values[0] >= 0 && values[0] <= 2 && values[1] >= 10 && values[1] <= 20
								}
							}
							f.checkFormula(answer, values, want)
						}
						checkOriginal()
					}
					if kinds[0] != "bound_relation" || mode.widen || empty {
						return
					}
					// A warmed repeated query must release every temporary rule
					// manager, helper declaration and product owner before return.
					refs := f.m.Stats().LiveReferences
					_, err = f.e.QueryPredicate(q)
					fragmentMust(t, err)
					if f.m.Stats().LiveReferences != refs {
						t.Fatal("repeated normalization left temporary AST owners")
					}
					status, err := f.e.QueryExpression(f.app(q, x, f.numeral(15)))
					fragmentMust(t, err)
					if status != lbool.Undef || f.e.queryClone == nil || f.e.queryClone.boundIntervalDatalog == nil || f.e.queryClone.boundIntervalDatalog == owner || f.e.queryClone.boundIntervalDatalog.entries[p].seed == owner.entries[p].seed {
						t.Fatal("normalized expression query did not clone its product seeds", status)
					}
					expression := f.own(f.e.ExpressionAnswer())
					f.checkFormula(expression, []int{0}, true)
					f.checkFormula(expression, []int{2}, true)
					f.checkFormula(expression, []int{3}, true)
					checkOriginal()
					guard := f.hold(f.m.MkArithmeticLE(x, f.numeral(1)))
					stronger := f.hold(f.m.MkAnd(body, guard))
					fragmentMust(t, f.e.UpdateRule(f.hold(f.m.MkImplies(stronger, head)), "normalize-three-positive"))
					if _, err := f.e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
						t.Fatal("named update exposed a stale expression answer", err)
					}
					updated := boundIntervalWideningAnswer(t, f, q)
					f.checkFormula(updated, []int{1, 15}, true)
					f.checkFormula(updated, []int{2, 15}, false)
					f.checkFormula(answer, []int{2, 15}, true)
					f.checkFormula(expression, []int{2}, true)
					if f.e.boundIntervalDatalog != owner || len(owner.entries) != 4 {
						t.Fatal("named update replaced the persistent product owners")
					}
					f.fact(s, 17, 7)
					if _, cause := f.e.Answer(q); !errors.Is(cause, ErrPropositionalEngine) {
						t.Fatal("fact update exposed the previous normalized result", cause)
					}
					factUpdated := boundIntervalWideningAnswer(t, f, q)
					f.checkFormula(factUpdated, []int{1, 17}, true)
					f.checkFormula(factUpdated, []int{2, 17}, false)
					f.checkFormula(updated, []int{1, 17}, true)
					f.checkFormula(answer, []int{1, 17}, true)
					f.m.Limit.Cancel()
					_, err = f.e.QueryPredicate(q)
					f.m.Limit.ResetCancel()
					if err == nil {
						t.Fatal("normalized query ignored cancellation")
					}
					if _, cause := f.e.Answer(q); !errors.Is(cause, ErrPropositionalEngine) {
						t.Fatal("canceled normalization exposed an answer", cause)
					}
					retried := boundIntervalWideningAnswer(t, f, q)
					f.checkFormula(retried, []int{1, 17}, true)
					f.checkFormula(retried, []int{2, 17}, false)
					fragmentMust(t, f.e.Close())
					for _, owned := range []ast.ASTRef{answer, expression, updated, factUpdated} {
						if _, cause := f.m.RefGet(owned); cause != nil {
							t.Fatal("normalization or engine cleanup invalidated a caller-owned answer", cause)
						}
					}
				})
			}
		}
	}
}

func TestDatalogBoundIntervalNormalizesHelpersWithOriginKinds(t *testing.T) {
	f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
	p, r, s, q := f.declare("kind-P", 2), f.declare("kind-R", 2), f.declare("kind-S", 2), f.declare("kind-Q", 2)
	for _, declaration := range []ast.ASTHandle{p, s} {
		fragmentMust(t, f.e.SetPredicateRepresentation(declaration, []string{"interval_relation", "bound_relation"}))
	}
	f.fact(p, 0, 0)
	f.fact(r, 0, 15)
	f.fact(s, 15, 7)
	x, y := f.hold(f.m.MkVar(0, f.integer)), f.hold(f.m.MkVar(1, f.integer))
	body := f.hold(f.m.MkAnd3(f.app(p, x, x), f.app(r, x, y), f.app(s, y, f.numeral(7))))
	fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(body, f.app(q, x, y))), "mixed-kind-origins"))
	source := f.e.boundIntervalDatalog
	scope, err := prepareBoundIntervalNormalization(source, []ast.ASTHandle{q})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, scope.Close()) }()
	if scope.rules == f.e.rules || scope.activation == source || len(source.entries) != 4 {
		t.Fatal("normalization did not isolate its rules and product owners")
	}
	for declaration, original := range source.entries {
		copy := scope.activation.entries[declaration]
		if copy == nil || copy == original || copy.seed == original.seed || !boundIntervalKindsEqual(copy.kinds, original.kinds) {
			t.Fatal("normalized activation did not copy each original seed and ordered kinds")
		}
	}
	count, err := scope.set.Len()
	fragmentMust(t, err)
	filters, joins := 0, 0
	var helpers []ast.ASTHandle
	for i := 0; i < count; i++ {
		rule, cause := scope.set.Rule(i)
		fragmentMust(t, cause)
		head, cause := rule.HeadDeclaration()
		fragmentMust(t, cause)
		if source.entries[head] != nil {
			fragmentMust(t, rule.Close())
			continue
		}
		positive, cause := rule.PositiveTailSize()
		fragmentMust(t, cause)
		var origin ast.ASTHandle
		switch positive {
		case 1:
			// Source filter heads inherit the original filtered tail's kind.
			origin, cause = rule.TailDeclaration(0)
			fragmentMust(t, cause)
			if origin != p && origin != s {
				t.Fatal("unexpected source filter origin")
			}
			filters++
		case 2:
			// The sole split helper belongs to the original Q rule, whose
			// requested order deliberately differs from both filter heads.
			origin = q
			joins++
		default:
			t.Fatal("unexpected normalized helper body", positive)
		}
		fragmentMust(t, rule.Close())
		entry := scope.activation.entries[head]
		if entry == nil || !boundIntervalKindsEqual(entry.kinds, source.entries[origin].kinds) {
			t.Fatal("generated helper did not inherit its source origin's ordered kinds")
		}
		registered, cause := f.e.rules.IsRegisteredPredicate(head)
		fragmentMust(t, cause)
		if registered {
			t.Fatal("temporary helper was registered in the persistent rule manager")
		}
		helpers = append(helpers, head)
	}
	if filters != 2 || joins != 1 {
		t.Fatal("normalization did not exercise both kind inheritance paths", filters, joins)
	}
	fragmentMust(t, scope.Close())
	for _, declaration := range helpers {
		if f.m.IsLiveHandle(declaration) {
			t.Fatal("closed normalization still owns a helper declaration")
		}
	}
	if len(source.entries) != 4 || source.closed {
		t.Fatal("temporary cleanup changed the persistent activation")
	}
}

func TestDatalogBoundIntervalNormalizesRecursiveThreePositiveTails(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, all := range []bool{false, true} {
			t.Run(kinds[0]+"/all="+strconv.FormatBool(all), func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, kinds)
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", strconv.FormatBool(all)))
				h, p, q := f.declare("normalize-recursive-H", 1), f.declare("normalize-recursive-P", 1), f.declare("normalize-recursive-Q", 1)
				for _, declaration := range []ast.ASTHandle{h, p, q} {
					f.fact(declaration, 7)
				}
				x := f.hold(f.m.MkVar(0, f.integer))
				body := f.hold(f.m.MkAnd3(f.app(h, x), f.app(p, x), f.app(q, x)))
				fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(body, f.app(h, x))), "normalized-recursive-identity"))
				for run := 0; run < 2; run++ {
					answer := boundIntervalWideningAnswer(t, f, h)
					f.checkFormula(answer, []int{7}, true)
					// C++ all-or-nothing delta propagation carries the full
					// projection payload into the recursive result.
					f.checkFormula(answer, []int{6}, all)
					f.checkFormula(answer, []int{8}, all)
				}
				if count, err := f.e.set.Len(); err != nil || count != 1 || len(f.e.boundIntervalDatalog.entries) != 3 {
					t.Fatal("recursive normalization published temporary helpers", count, err)
				}
			})
		}
	}
}

func TestDatalogBoundIntervalNormalizesOnlyPositiveLongBodies(t *testing.T) {
	f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
	p, q, r := f.declare("negative-P", 1), f.declare("negative-Q", 1), f.declare("negative-R", 1)
	n, h := f.declare("negative-N", 1), f.declare("negative-H", 1)
	for _, declaration := range []ast.ASTHandle{p, q, r, n} {
		f.fact(declaration, 7)
	}
	x := f.hold(f.m.MkVar(0, f.integer))
	negative := f.hold(f.m.MkNot(f.app(n, x)))
	body := f.hold(f.m.MkAndArgs([]ast.ASTHandle{f.app(p, x), f.app(q, x), f.app(r, x), negative}))
	fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(body, f.app(h, x))), "normalized-negative-boundary"))
	source, owner := f.e.set, f.e.boundIntervalDatalog
	entries := make(map[ast.ASTHandle]*boundIntervalDatalogEntry, len(owner.entries))
	for declaration, entry := range owner.entries {
		entries[declaration] = entry
	}
	normalize, err := owner.needsJoinNormalization()
	fragmentMust(t, err)
	if !normalize {
		t.Fatal("negative boundary fixture bypasses positive-tail normalization")
	}
	if _, err = f.e.QueryPredicate(h); !errors.Is(err, ErrBoundIntervalDatalogFragment) {
		t.Fatal("long-body normalization accepted unsupported populated negation", err)
	}
	if _, err = f.e.Answer(h); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("failed normalized query exposed an answer", err)
	}
	if f.e.set != source || f.e.boundIntervalDatalog != owner || len(owner.entries) != 5 || len(f.e.predicates) != 5 {
		t.Fatal("refused normalization changed persistent program or predicate owners")
	}
	for declaration, entry := range entries {
		if owner.entries[declaration] != entry {
			t.Fatal("refused normalization replaced a persistent seed owner")
		}
	}
	count, err := source.Len()
	fragmentMust(t, err)
	rule, err := source.Rule(0)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, rule.Close()) }()
	positive, err := rule.PositiveTailSize()
	fragmentMust(t, err)
	uninterpreted, err := rule.UninterpretedTailSize()
	fragmentMust(t, err)
	name, err := rule.Name()
	fragmentMust(t, err)
	if count != 1 || positive != 3 || uninterpreted != 4 || name != "normalized-negative-boundary" {
		t.Fatal("refused normalization changed the original rule", count, positive, uninterpreted, name)
	}
}
