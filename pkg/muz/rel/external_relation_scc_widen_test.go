package rel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestExternalRuleSCCSchedulingAndWideningCallbacks(t *testing.T) {
	for _, widen := range []bool{false, true} {
		t.Run(fmt.Sprintf("widen=%v", widen), func(t *testing.T) {
			f := newTableRelationFixture(t)
			type event struct {
				op, outputs int
				target      ast.ASTHandle
			}
			var events []event
			rows := make(map[ast.ASTHandle][][]ast.ASTHandle)
			equal := func(left, right []ast.ASTHandle) bool {
				if len(left) != len(right) {
					return false
				}
				for i := range left {
					if left[i] != right[i] {
						return false
					}
				}
				return true
			}
			insert := func(target ast.ASTHandle, tuple []ast.ASTHandle) bool {
				for _, old := range rows[target] {
					if equal(old, tuple) {
						return false
					}
				}
				rows[target] = append(rows[target], append([]ast.ASTHandle(nil), tuple...))
				return true
			}
			plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
				ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
					kind, cause := f.ast.GetDeclKind(declaration)
					if cause != nil {
						return cause
					}
					switch kind {
					case ast.DatalogOpEmpty:
						rows[outputs[0]] = nil
					case ast.DatalogOpStore:
						insert(outputs[0], arguments[1:])
					case ast.DatalogOpClone:
						rows[outputs[0]] = nil
						for _, tuple := range rows[arguments[0]] {
							insert(outputs[0], tuple)
						}
					case ast.DatalogOpUnion, ast.DatalogOpWiden:
						events = append(events, event{int(kind), len(outputs), outputs[0]})
						if len(outputs) == 2 {
							rows[outputs[1]] = nil
						}
						for _, tuple := range rows[arguments[1]] {
							if insert(outputs[0], tuple) && len(outputs) == 2 {
								insert(outputs[1], tuple)
							}
						}
					default:
						t.Fatalf("unexpected SCC assignment operation %d", kind)
					}
					return nil
				},
				Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
					kind, cause := f.ast.GetDeclKind(declaration)
					if cause != nil {
						return ast.ASTHandle{}, cause
					}
					if kind == ast.DatalogOpIsEmpty {
						return f.ast.MkBoolVal(len(rows[arguments[0]]) == 0)
					}
					sort, cause := f.ast.GetRange(declaration)
					if cause != nil {
						return ast.ASTHandle{}, cause
					}
					result, cause := f.ast.MkFreshConst("external-scc-matrix", sort)
					if cause != nil {
						return ast.ASTHandle{}, cause
					}
					parameters, cause := f.ast.GetDeclParametersCopy(declaration)
					if cause != nil {
						return ast.ASTHandle{}, cause
					}
					indices := make([]int32, len(parameters))
					for i := range parameters {
						indices[i], cause = f.ast.ParameterInt(parameters[i])
						cause = errors.Join(cause, f.ast.DelParameter(&parameters[i]))
						if cause != nil {
							return ast.ASTHandle{}, cause
						}
					}
					switch kind {
					case ast.DatalogOpJoin:
						for _, left := range rows[arguments[0]] {
							for _, right := range rows[arguments[1]] {
								matched := true
								for i := 0; i < len(indices); i += 2 {
									matched = matched && left[indices[i]] == right[indices[i+1]]
								}
								if matched {
									insert(result, append(append([]ast.ASTHandle(nil), left...), right...))
								}
							}
						}
					case ast.DatalogOpProject:
						removed := make(map[int32]bool, len(indices))
						for _, index := range indices {
							removed[index] = true
						}
						for _, tuple := range rows[arguments[0]] {
							projected := make([]ast.ASTHandle, 0, len(tuple)-len(removed))
							for i, value := range tuple {
								if !removed[int32(i)] {
									projected = append(projected, value)
								}
							}
							insert(result, projected)
						}
					default:
						t.Fatalf("unexpected SCC reduction operation %d", kind)
					}
					return result, nil
				},
			})
			astTestMust(t, err)
			defer func() { astTestMust(t, plugin.Close()) }()
			rules, err := base.NewRuleManager(f.ast)
			astTestMust(t, err)
			defer func() { astTestMust(t, rules.Close()) }()
			serial := 0
			declare := func(label string) ast.ASTHandle {
				serial++
				name, cause := f.ast.Symbols.Intern(fmt.Sprintf("scc-%s-%t-%d", label, widen, serial))
				astTestMust(t, cause)
				declaration, cause := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, cause)
				astTestPin(t, f.ast, declaration)
				astTestMust(t, rules.RegisterPredicate(declaration))
				return declaration
			}
			app := func(declaration ast.ASTHandle, argument ast.ASTHandle) ast.ASTHandle {
				value, cause := f.ast.MkApp(declaration, []ast.ASTHandle{argument})
				astTestMust(t, cause)
				return value
			}
			x, err := f.ast.MkVar(0, f.boolean)
			astTestMust(t, err)
			astTestPin(t, f.ast, x)
			run := func(specs []struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
				name  string
			}, requested ...ExternalRuleExecutionOptions) {
				t.Helper()
				set, cause := base.NewRuleSet(rules)
				astTestMust(t, cause)
				defer func() { astTestMust(t, set.Close()) }()
				for _, spec := range specs {
					rule, addErr := rules.NewRule(spec.head, spec.tails, nil, spec.name)
					astTestMust(t, addErr)
					astTestMust(t, set.Add(rule))
					astTestMust(t, rule.Close())
				}
				options := ExternalRuleExecutionOptions{CompileWithWidening: widen}
				if len(requested) != 0 {
					options = requested[0]
				}
				astTestMust(t, ExecuteExternalFactRulesWithOptions(plugin, set, options))
			}
			handle := func(predicate ast.ASTHandle) ast.ASTHandle {
				relation, cause := plugin.PredicateRelation(predicate)
				astTestMust(t, cause)
				value, cause := relation.Relation()
				astTestMust(t, cause)
				return value
			}
			targetEvents := func(predicate ast.ASTHandle, start int) []event {
				target := handle(predicate)
				var result []event
				for _, current := range events[start:] {
					if current.target == target {
						result = append(result, current)
					}
				}
				return result
			}
			loopOp := int32(ast.DatalogOpUnion)
			if widen {
				loopOp = ast.DatalogOpWiden
			}

			// Reversed source order must not disturb dependency-first one-shot
			// scheduling. Facts stay Store operations and q receives one UNION
			// output without a recursive delta.
			p, q := declare("chain-p"), declare("chain-q")
			start := len(events)
			run([]struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
				name  string
			}{{app(q, x), []ast.ASTHandle{app(p, x)}, "q-before-p"}, {app(p, f.truth), nil, "p-fact"}})
			chain := targetEvents(q, start)
			if len(chain) != 1 || chain[0].op != int(ast.DatalogOpUnion) || chain[0].outputs != 1 {
				t.Fatal("nonrecursive component did not use one-shot UNION/1", chain)
			}

			// The lower-only seed initializes p with UNION/1. It is not rerun in
			// the recursive loop; only p<-p receives a two-output update.
			seed, recursive := declare("self-seed"), declare("self-p")
			start = len(events)
			run([]struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
				name  string
			}{{app(recursive, x), []ast.ASTHandle{app(recursive, x)}, "self-first"}, {app(recursive, x), []ast.ASTHandle{app(seed, x)}, "lower-seed"}, {app(seed, f.truth), nil, "seed-fact"}})
			self := targetEvents(recursive, start)
			if len(self) != 2 || self[0].op != int(ast.DatalogOpUnion) || self[0].outputs != 1 || self[1].op != int(loopOp) || self[1].outputs != 2 {
				t.Fatal("recursive seed/loop callback schedule", self)
			}

			// A recursive component with no seed clones an empty head delta and
			// performs no loop update.
			unseeded := declare("unseeded")
			start = len(events)
			run([]struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
				name  string
			}{{app(unseeded, x), []ast.ASTHandle{app(unseeded, x)}, "unseeded-self"}})
			if got := targetEvents(unseeded, start); len(got) != 0 {
				t.Fatal("unseeded recursion performed a loop update", got)
			}

			// Each internal tail occurrence schedules one semi-naive evaluation.
			double := declare("double-tail")
			start = len(events)
			run([]struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
				name  string
			}{{app(double, x), []ast.ASTHandle{app(double, x), app(double, x)}, "two-internal-tails"}, {app(double, f.truth), nil, "double-fact"}})
			doubled := targetEvents(double, start)
			if len(doubled) != 2 {
				t.Fatal("two internal tails did not schedule two updates", doubled)
			}
			for _, current := range doubled {
				if current.op != int(loopOp) || current.outputs != 2 {
					t.Fatal("internal-tail loop update profile", doubled)
				}
			}

			// All-or-nothing mode evaluates every nonground component rule once
			// with full inputs while any component delta is active. Thus the
			// lower-only seed rule joins the recursive rule in the single loop
			// round, instead of being initialization-only as above.
			allSeed, allRecursive := declare("all-seed"), declare("all-p")
			start = len(events)
			run([]struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
				name  string
			}{{app(allRecursive, x), []ast.ASTHandle{app(allRecursive, x)}, "all-self"}, {app(allRecursive, x), []ast.ASTHandle{app(allSeed, x)}, "all-lower-seed"}, {app(allSeed, f.truth), nil, "all-fact"}}, ExternalRuleExecutionOptions{CompileWithWidening: widen, AllOrNothingDeltas: true})
			all := targetEvents(allRecursive, start)
			if len(all) != 3 || all[0].op != int(ast.DatalogOpUnion) || all[0].outputs != 1 {
				t.Fatal("all-or-nothing initialization profile", all)
			}
			for _, current := range all[1:] {
				if current.op != int(loopOp) || current.outputs != 2 {
					t.Fatal("all-or-nothing full-input loop profile", all)
				}
			}

			// Mutual recursion is one SCC. The lower seed is initialized first;
			// every later update is a two-output loop operation for p or q.
			mutualSeed, mutualP, mutualQ := declare("mutual-seed"), declare("mutual-p"), declare("mutual-q")
			start = len(events)
			run([]struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
				name  string
			}{{app(mutualP, x), []ast.ASTHandle{app(mutualQ, x)}, "p-before-q"}, {app(mutualQ, x), []ast.ASTHandle{app(mutualP, x)}, "q-from-p"}, {app(mutualP, x), []ast.ASTHandle{app(mutualSeed, x)}, "mutual-lower-seed"}, {app(mutualSeed, f.truth), nil, "mutual-fact"}})
			mutual := append(targetEvents(mutualP, start), targetEvents(mutualQ, start)...)
			oneOutput, loopUpdates := 0, 0
			for _, current := range mutual {
				if current.op == int(ast.DatalogOpUnion) && current.outputs == 1 {
					oneOutput++
					continue
				}
				if current.op == int(loopOp) && current.outputs == 2 {
					loopUpdates++
					continue
				}
				t.Fatal("mutual SCC callback profile", mutual)
			}
			if oneOutput != 1 || loopUpdates == 0 {
				t.Fatal("mutual SCC missed initialization or loop updates", mutual)
			}
		})
	}
}
