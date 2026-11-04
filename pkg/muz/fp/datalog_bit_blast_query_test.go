package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogBitBlastFiniteQueriesPreserveOriginalDomain(t *testing.T) {
	for _, tc := range []struct {
		name, kind             string
		module, local, enabled bool
		checked                bool
	}{
		{name: "default"},
		{name: "module", module: true, enabled: true},
		{name: "local", local: true, enabled: true},
		{name: "local-disables-module", module: true, local: true},
		{name: "sparse", module: true, enabled: true, kind: "tr_sparse"},
		{name: "lazy", module: true, enabled: true, kind: "tr_lazy_sparse"},
		{name: "checked", module: true, enabled: true, checked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(globals.SourceSetName("fixedpoint.datalog.default_table", "hashtable"))
			for _, name := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
				must(globals.SourceSetName(name, "false"))
			}
			if tc.module {
				must(globals.SourceSetName("fixedpoint.xform.bit_blast", "true"))
			}
			if tc.checked {
				must(globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
				must(globals.SourceSetName("fixedpoint.datalog.default_table_checker", "sparse"))
			}
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			must(err)
			t.Cleanup(func() { must(e.Close()) })
			must(e.SelectEngine("datalog"))
			checkKind := func(state *predicateState) {
				t.Helper()
				switch tc.kind {
				case "":
					if _, ok := state.table.(*rel.HashTable); !ok {
						t.Fatalf("bit-blast default table %T", state.table)
					}
				case "tr_sparse":
					if _, ok := state.table.(*rel.SparseTable); !ok {
						t.Fatalf("bit-blast sparse table %T", state.table)
					}
				case "tr_lazy_sparse":
					if _, ok := state.table.(*rel.LazySparseTable); !ok || e.lazyFinite == nil || !e.lazyFinite.Owns(state.table) {
						t.Fatalf("bit-blast lazy sparse owner %T", state.table)
					}
				}
				if tc.kind != "" && (len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != tc.kind || !state.explicitFiniteRepresentation) {
					t.Fatal("bit-blast requested kind metadata", state.requestedRepresentations, state.explicitFiniteRepresentation)
				}
			}
			if tc.local {
				local := globals.Context.EmptyValue()
				key := "xform.bit_blast"
				must(globals.Context.SetBoolName(&local, &key, tc.enabled))
				must(e.SetQueryParameters(&local))
				must(globals.Context.Del(&local))
			}
			boolean, err := m.MkBoolSort()
			must(err)
			bv, err := m.BitVectorSort(2)
			must(err)
			declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				symbol, err := m.Symbols.Intern("finite-blast-" + name)
				must(err)
				d, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(d))
				return d
			}
			p, q, goal := declare("P", []ast.ASTHandle{bv}), declare("Q", []ast.ASTHandle{bv}), declare("Goal", nil)
			if tc.kind != "" {
				must(e.SetPredicateRepresentation(p, []string{tc.kind}))
				must(e.SetPredicateRepresentation(q, []string{tc.kind}))
			}
			must(e.AddTableFact(p, []uint32{1}))
			must(e.AddTableFact(p, []uint32{2}))
			x, err := m.MkVar(0, bv)
			must(err)
			app := func(d, value ast.ASTHandle) ast.ASTHandle {
				a, err := m.MkApp(d, []ast.ASTHandle{value})
				must(err)
				return a
			}
			pa, qa := app(p, x), app(q, x)
			one, err := m.BitVectorNumeralUint64(1, 2)
			must(err)
			lowHalf, err := m.MkBitVectorULE(x, one)
			must(err)
			body, err := m.MkAnd(pa, lowHalf)
			must(err)
			rule, err := m.MkImplies(body, qa)
			must(err)
			must(e.AddRule(rule, "finite-bit-filter"))
			three, err := m.BitVectorNumeralUint64(3, 2)
			must(err)
			ga, err := m.MkApp(goal, nil)
			must(err)
			rule, err = m.MkImplies(app(q, three), ga)
			must(err)
			must(e.AddRule(rule, "finite-bit-unreachable"))

			// Exercise actual source expansion and stored-row import while the
			// generated relation owners are still visible in their query scope.
			func() {
				scope, err := e.beginDatalogQueryScope([]ast.ASTHandle{q})
				must(err)
				original := e.set
				e.set = scope.set
				defer func() { e.set = original; must(scope.Close()) }()
				valid, err := e.set.Finalize()
				must(err)
				if !valid {
					t.Fatal("bit-blast fixture stratification")
				}
				blast := &datalogBitBlastQuery{engine: e}
				must(e.normalizeSimpleJoins(blast))
				if (len(blast.columns) == 2) != tc.enabled || !tc.enabled && len(blast.columns) != 0 {
					t.Fatal("configured bit-blast mapping", len(blast.columns), tc.enabled)
				}
				for _, columns := range blast.columns {
					checkKind(e.predicates[columns.derived])
					mapped, err := e.set.DerivedPredicate(columns.original)
					must(err)
					if mapped != columns.derived {
						t.Fatal("source predicate mapping lost")
					}
					if columns.original == p {
						for _, bits := range []rel.Fact{{1, 0}, {0, 1}} {
							found, err := e.predicates[mapped].table.Contains(bits)
							must(err)
							if !found {
								t.Fatal("stored BV tuple did not use low-bit-first columns", bits)
							}
						}
					}
				}
			}()

			if tc.name == "module" {
				// Refuse work across normalization and execution boundaries,
				// including a failed query after the mapping was installed.
				// Keep the original formulas alive so every published clause can be
				// checked against its source, including during a partial flush.
				originalRaw := append([]rawRuleFormula(nil), e.rawRules...)
				if len(originalRaw) != 2 {
					t.Fatal("fixture raw rule count", len(originalRaw))
				}
				names := []string{"finite-bit-filter", "finite-bit-unreachable"}
				originalFormulas := make([]ast.ASTRef, len(names))
				for i := range names {
					r, err := e.set.Rule(i)
					must(err)
					name, err := r.Name()
					must(err)
					if name != names[i] {
						t.Fatal("fixture rule name", i, name)
					}
					originalFormulas[i], err = r.ToFormula()
					must(err)
					must(r.Close())
				}
				defer func() {
					for i := range originalFormulas {
						must(m.DelRef(&originalFormulas[i]))
					}
				}()
				interrupted, completed := false, false
				emptyFlush, partialFlush := false, false
				for allowance := uint32(1); allowance <= 512; allowance++ {
					revision := e.revision
					must(m.Limit.Push(allowance))
					status, cause := e.QueryPredicate(q)
					m.Limit.Pop()
					if len(e.predicates) != 3 || len(e.auxiliary) != 0 {
						t.Fatal("failed bit-blast query left generated predicates", allowance, cause)
					}
					if len(e.rawRules) != len(originalRaw) {
						t.Fatal("query changed raw rule count", allowance, len(e.rawRules))
					}
					for i, raw := range e.rawRules {
						if raw.formula != originalRaw[i].formula || raw.name != originalRaw[i].name || raw.bound != originalRaw[i].bound {
							t.Fatal("query changed a raw source rule", allowance, i)
						}
					}
					count, err := e.set.Len()
					must(err)
					seen := make([]int, len(names))
					previous := -1
					for i := 0; i < count; i++ {
						r, err := e.set.Rule(i)
						must(err)
						name, err := r.Name()
						must(err)
						index := -1
						for j := range names {
							if name == names[j] {
								index = j
								break
							}
						}
						if index < 0 || index < previous {
							t.Fatal("query changed published rule identity or order", allowance, i, name)
						}
						formula, err := r.ToFormula()
						must(err)
						got, err := m.RefGet(formula)
						must(err)
						want, err := m.RefGet(originalFormulas[index])
						must(err)
						if got != want {
							t.Fatal("query changed published rule formula", allowance, i, name)
						}
						must(m.DelRef(&formula))
						must(r.Close())
						seen[index]++
						previous = index
					}
					for i, raw := range e.rawRules {
						if !raw.pending && seen[i] == 0 {
							t.Fatal("flushed source rule is missing", allowance, names[i])
						}
					}
					if cause != nil {
						emptyFlush = emptyFlush || count == 0 && e.rawRules[0].pending && e.rawRules[1].pending
						partialFlush = partialFlush || count == 1 && seen[0] == 1 && seen[1] == 0 && !e.rawRules[0].pending && e.rawRules[1].pending
					}
					if cause == nil && (seen[0] == 0 || seen[1] == 0) {
						t.Fatal("successful retry lost source rules", allowance, seen)
					}
					for _, row := range []rel.Fact{{1}, {2}} {
						found, err := e.predicates[p].table.Contains(row)
						must(err)
						if !found {
							t.Fatal("failed bit-blast query lost stored row", row, allowance)
						}
					}
					if cause != nil {
						if status != lbool.Undef {
							t.Fatal("failed query published a status", status, cause)
						}
						if answer, err := e.Answer(q); err == nil || answer != (ast.ASTRef{}) {
							t.Fatal("failed bit-blast query published an answer", answer, err)
						}
						interrupted = interrupted || e.revision > revision
						continue
					}
					if status != lbool.True {
						t.Fatal("retry lost the reachable tuple", status, allowance, revision, e.revision)
					}
					completed = true
					t.Logf("resource campaign completed at allowance %d after a transformed-query interruption", allowance)
					break
				}
				if !interrupted || !completed || !emptyFlush || !partialFlush {
					t.Fatal("resource campaign missed an interruption, completion, or flush state", interrupted, completed, emptyFlush, partialFlush)
				}
			}

			for attempt := 0; attempt < 2; attempt++ {
				if attempt == 1 {
					must(e.AddTableFact(p, []uint32{0}))
				}
				status, err := e.QueryPredicate(q)
				must(err)
				if status != lbool.True || !e.compiled {
					t.Fatal("finite bit-blast query", status, e.compiled)
				}
				rows, err := e.predicates[q].table.Facts()
				must(err)
				checkKind(e.predicates[q])
				if len(rows) != 1+attempt {
					t.Fatal("original-domain rows", rows)
				}
				answer, err := e.Answer(q)
				must(err)
				formula, err := m.RefGet(answer)
				must(err)
				substitution, err := astrewriter.NewVarSubstitution(m, false)
				must(err)
				for value := uint64(0); value < 4; value++ {
					constant, err := m.BitVectorNumeralUint64(value, 2)
					must(err)
					ground, err := m.NewRef(ast.ASTHandle{})
					must(err)
					must(substitution.Substitute(formula, []ast.ASTHandle{constant}, ground))
					expression, err := m.RefGet(ground)
					must(err)
					reduced, err := m.NewRef(ast.ASTHandle{})
					must(err)
					must(e.guard.RewriteWithBindings(expression, nil, reduced))
					truth, err := m.RefGet(reduced)
					must(err)
					want := value == 1 || attempt == 1 && value == 0
					if m.IsTrue(truth) != want || !m.IsTrue(truth) && !m.IsFalse(truth) {
						t.Fatal("original BV answer coordinates", value, want)
					}
					must(m.DelRef(&reduced))
					must(m.DelRef(&ground))
				}
				must(substitution.Close())
				must(m.DelRef(&answer))
				if len(e.predicates) != 3 || len(e.auxiliary) != 0 {
					t.Fatal("bit-blasted predicates escaped scope")
				}
			}
			status, err := e.QueryPredicate(goal)
			must(err)
			if status != lbool.False {
				t.Fatal("unreachable original goal", status)
			}
			inductive, err := e.Model(goal)
			must(err)
			defer func() { must(inductive.Close()) }()
			cooperation, err := cooperate.NewCooperation(m.Budget())
			must(err)
			defer func() { must(cooperation.Close()) }()
			evaluator, err := model.NewModelEvaluator(inductive.Core(), model.ModelEvaluatorConfig{Context: globals.Context, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation})
			must(err)
			defer func() { must(evaluator.Close()) }()
			for _, d := range []ast.ASTHandle{p, q} {
				for value := uint64(0); value < 4; value++ {
					constant, err := m.BitVectorNumeralUint64(value, 2)
					must(err)
					result, err := evaluator.EvaluateValue(app(d, constant))
					must(err)
					truth, err := m.RefGet(result)
					must(err)
					want := value < 2 || d == p && value == 2
					if m.IsTrue(truth) != want || !m.IsTrue(truth) && !m.IsFalse(truth) {
						t.Fatal("original BV model interpretation", d, value, want)
					}
					must(m.DelRef(&result))
				}
			}
		})
	}
}
