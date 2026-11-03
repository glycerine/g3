package fp

// Source: context::mk_fresh_head_predicate and rel_context::inherit_predicate_kind
// copy explicit requests from a filtered tail or a split's selected parent.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogGeneratedFiltersAndJoinsInheritFiniteKinds(t *testing.T) {
	for _, shape := range []string{"filter", "nullary-filter", "split", "shared-split"} {
		for _, kind := range []string{"", "tr_hashtable", "tr_sparse", "tr_lazy_sparse"} {
			name := kind
			if name == "" {
				name = "unrequested"
			}
			t.Run(shape+"/"+name, func(t *testing.T) {
				m, globals := newPDRLifecycleServices(t)
				must := func(err error) { t.Helper(); fragmentMust(t, err) }
				must(globals.SourceSetName("fixedpoint.datalog.default_table", "hashtable"))
				for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
					must(globals.SourceSetName(key, "false"))
				}
				e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
				must(err)
				t.Cleanup(func() { must(e.Close()) })
				must(e.SelectEngine("datalog"))
				boolean, err := m.MkBoolSort()
				must(err)
				declare := func(name string, arity int) ast.ASTHandle {
					symbol, err := m.Symbols.Intern("generated-kind-" + name)
					must(err)
					domain := make([]ast.ASTHandle, arity)
					for i := range domain {
						domain[i] = boolean
					}
					d, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
					must(err)
					must(e.RegisterPredicate(d))
					return d
				}
				x, err := m.MkVar(0, boolean)
				must(err)
				truth, err := m.MkTrue()
				must(err)
				app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
					a, err := m.MkApp(d, args)
					must(err)
					return a
				}
				add := func(head ast.ASTHandle, tails ...ast.ASTHandle) {
					body, err := m.MkAndArray(tails)
					must(err)
					rule, err := m.MkImplies(body, head)
					must(err)
					must(e.AddRule(rule, ""))
				}
				var origin, seed ast.ASTHandle
				var outputs []ast.ASTHandle
				ground := shape == "nullary-filter"
				if shape == "filter" || ground {
					seed = declare("P", 2)
					headArity := 1
					if ground {
						headArity = 0
					}
					head := declare("H", headArity)
					outputs, origin = []ast.ASTHandle{head}, seed
					must(e.AddTableFact(seed, []uint32{0, 1}))
					must(e.AddTableFact(seed, []uint32{1, 0}))
					if ground {
						add(app(head), app(seed, truth, truth))
					} else {
						add(app(head, x), app(seed, x, truth))
					}
				} else {
					a, b, c := declare("A", 1), declare("B", 1), declare("C", 1)
					for _, d := range []ast.ASTHandle{a, b, c} {
						must(e.AddTableFact(d, []uint32{0}))
						must(e.AddTableFact(d, []uint32{1}))
					}
					h := declare("H", 1)
					outputs, origin = []ast.ASTHandle{h}, h
					add(app(h, x), app(a, x), app(b, x), app(c, x))
					if shape == "shared-split" {
						other, short := declare("Other", 1), declare("Short", 1)
						add(app(other, x), app(a, x), app(b, x), app(c, x))
						// AddFormula normalizes conjunction order. Share the
						// first actual pair selected on equal source costs.
						parent, err := e.set.Rule(0)
						must(err)
						first, _, err := parent.Tail(0)
						must(err)
						second, _, err := parent.Tail(1)
						must(err)
						add(app(short, x), first, second)
						must(parent.Close())
						outputs = append(outputs, other, short)
						origin = short
						// The short last consumer selects the kind, not this
						// first long consumer that actually requires a split.
						otherKind := "tr_sparse"
						if kind == otherKind {
							otherKind = "tr_hashtable"
						}
						must(e.SetPredicateRepresentation(h, []string{otherKind}))
					}
				}
				if kind != "" {
					must(e.SetPredicateRepresentation(origin, []string{kind}))
				}
				must(e.applyDatalogDefaultFiniteTable())
				if kind == "" {
					// An incidental concrete owner is not a requested kind.
					must(e.selectFiniteRepresentation(origin, "tr_sparse"))
				}
				originalCount, err := e.set.Len()
				must(err)
				originalPredicates := len(e.predicates)
				func() {
					scope, err := e.beginDatalogQueryScope(outputs)
					must(err)
					original := e.set
					e.set = scope.set
					defer func() { e.set = original; must(scope.Close()) }()
					valid, err := e.set.Finalize()
					must(err)
					if !valid {
						t.Fatal("kind fixture is not stratified")
					}
					must(e.normalizeSimpleJoins())
					if len(e.auxiliary) != 1 {
						t.Fatal("expected exactly one shared filter/split", len(e.auxiliary))
					}
					for d := range e.auxiliary {
						state := e.predicates[d]
						generatedName, err := m.GetASTName(d)
						must(err)
						generatedText, err := m.Symbols.String(generatedName)
						must(err)
						if kind == "" {
							if len(state.requestedRepresentations) != 0 || state.explicitFiniteRepresentation {
								t.Fatal("incidental backing became an explicit request")
							}
						} else if len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != kind || state.explicitFiniteRepresentation != (!ground || kind == "tr_hashtable") {
							t.Fatal("generated kind metadata", generatedText, state.requestedRepresentations, state.explicitFiniteRepresentation)
						}
						switch {
						case kind == "" || kind == "tr_hashtable" || ground:
							if _, ok := state.table.(*rel.HashTable); !ok {
								t.Fatalf("expected default/nullary Hash, got %T", state.table)
							}
						case kind == "tr_sparse":
							if _, ok := state.table.(*rel.SparseTable); !ok {
								t.Fatalf("expected inherited Sparse, got %T", state.table)
							}
						case kind == "tr_lazy_sparse":
							if _, ok := state.table.(*rel.LazySparseTable); !ok || e.lazyFinite == nil || !e.lazyFinite.Owns(state.table) {
								t.Fatalf("expected owned inherited LazySparse, got %T", state.table)
							}
						}
					}
				}()
				for query := 0; query < 2; query++ {
					if query == 1 && !seed.IsNull() {
						must(e.AddTableFact(seed, []uint32{1, 1}))
					}
					status, err := e.QueryPredicates(outputs)
					must(err)
					wantStatus := lbool.True
					if ground && query == 0 {
						wantStatus = lbool.False
					}
					if status != wantStatus || !e.compiled {
						t.Fatal("generated backing query", status, wantStatus, e.compiled)
					}
					for _, d := range outputs {
						rows, err := e.predicates[d].table.Facts()
						must(err)
						want := 2
						if shape == "filter" {
							want = 1 + query
						} else if ground {
							want = query
						}
						if len(rows) != want {
							t.Fatal("generated kind changed output tuples", rows, want)
						}
						for _, row := range rows {
							if ground && len(row) != 0 || !ground && (len(row) != 1 || row[0] > 1 || shape == "filter" && query == 0 && row[0] != 0) {
								t.Fatal("unexpected generated-kind tuple", row)
							}
						}
					}
					count, err := e.set.Len()
					must(err)
					if count != originalCount || len(e.predicates) != originalPredicates || len(e.auxiliary) != 0 {
						t.Fatal("kind inheritance escaped query scope")
					}
				}
			})
		}
	}
}
