package fp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogExpressionExplanationsCarryNamedProofAndSurviveScope(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	must(e.AddFact(f.declarations["p"], nil))
	step, err := m.MkImplies(f.applications["p"], f.applications["q"])
	must(err)
	must(e.addRuleEager(step, "explained-step"))
	status, err := e.QueryExpression(f.applications["q"])
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	root, err := m.RefGet(answer)
	must(err)
	var walk func(ast.ASTHandle)
	names := map[string]bool{}
	walk = func(h ast.ASTHandle) {
		kind, err := m.GetKind(h)
		must(err)
		if kind != ast.ASTApp {
			return
		}
		d, err := m.GetDecl(h)
		must(err)
		sym, err := m.GetASTName(d)
		must(err)
		name, _, err := m.Symbols.BareString(sym)
		must(err)
		names[name] = true
		n, err := m.GetNumArgs(h)
		must(err)
		for i := uint32(0); i < n; i++ {
			arg, err := m.GetArg(h, i)
			must(err)
			walk(arg)
		}
	}
	walk(root)
	if !names["explained-step"] || !names["fact"] {
		t.Fatal("answer lacks source proof constructors", names)
	}
	// The transformed product has already been scoped away; the owning answer
	// must also outlive destruction of the expression-query clone.
	status, err = e.QueryPredicate(f.declarations["q"])
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	held, err := m.RefGet(answer)
	must(err)
	if held != root {
		t.Fatal("answer owner changed after query cleanup")
	}
	if e.datalogExplanationAnswer != (ast.ASTRef{}) {
		t.Fatal("direct relation query enabled expression explanations")
	}
}

func TestDatalogExpressionExplanationModesAndRecursiveSparseSeeds(t *testing.T) {
	for _, mode := range []string{"enabled", "magic", "local-disabled", "relation-level", "checked", "relation-checked", "relation-checked-sparse", "relation-checked-lazy", "relation-checked-explicit", "checked-default", "checked-default-reverse", "checked-default-lazy", "checked-default-sparse-lazy", "checked-default-lazy-sparse", "relation-checked-default", "relation-checked-default-reverse", "relation-checked-default-lazy", "empty"} {
		t.Run(mode, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
			if mode == "magic" {
				must(e.globals.SourceSetName("fixedpoint.datalog.magic_sets_for_queries", "true"))
			}
			if mode == "checked" {
				must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "sparse"))
			}
			if mode == "relation-checked" || mode == "relation-checked-explicit" {
				must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "hashtable"))
			}
			if mode == "relation-checked-sparse" {
				must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "sparse"))
			}
			if mode == "relation-checked-lazy" {
				must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "lazy_sparse"))
			}
			if mode == "checked-default" || mode == "relation-checked-default" {
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "hashtable"))
			}
			if mode == "checked-default-reverse" || mode == "relation-checked-default-reverse" {
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "hashtable"))
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "sparse"))
			}
			if mode == "checked-default-lazy" || mode == "relation-checked-default-lazy" {
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "lazy_sparse"))
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "hashtable"))
			}
			if mode == "checked-default-sparse-lazy" || mode == "checked-default-lazy-sparse" {
				primary, checker := "sparse", "lazy_sparse"
				if mode == "checked-default-lazy-sparse" {
					primary, checker = checker, primary
				}
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table", primary))
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", checker))
			}
			if mode == "relation-level" || mode == "relation-checked" || mode == "relation-checked-sparse" || mode == "relation-checked-lazy" || mode == "relation-checked-explicit" || mode == "relation-checked-default" || mode == "relation-checked-default-reverse" || mode == "relation-checked-default-lazy" {
				must(e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
			}
			if mode == "local-disabled" {
				params := e.params.EmptyValue()
				key := "datalog.generate_explanations"
				must(e.params.SetBoolName(&params, &key, false))
				must(e.SetQueryParameters(&params))
				must(e.params.Del(&params))
			}
			b, err := m.MkBoolSort()
			must(err)
			sym, err := m.Symbols.Intern("explanation-sparse-P")
			must(err)
			d, err := m.MkFuncDecl(sym, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(d))
			yes, err := m.MkTrue()
			must(err)
			if mode != "empty" {
				must(e.AddFact(d, []ast.ASTHandle{yes}))
			}
			x, err := m.MkVar(0, b)
			must(err)
			px, err := m.MkApp(d, []ast.ASTHandle{x})
			must(err)
			rsym, err := m.Symbols.Intern("explanation-sparse-R")
			must(err)
			rd, err := m.MkFuncDecl(rsym, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(rd))
			if mode == "relation-checked-explicit" {
				must(e.SetPredicateRepresentation(rd, []string{"tr_sparse"}))
			}
			rx, err := m.MkApp(rd, []ast.ASTHandle{x})
			must(err)
			forward, err := m.MkImplies(px, rx)
			must(err)
			must(e.addRuleEager(forward, "forward"))
			backward, err := m.MkImplies(rx, px)
			must(err)
			must(e.addRuleEager(backward, "backward"))
			query, err := m.MkApp(rd, []ast.ASTHandle{yes})
			must(err)
			queryRef, err := m.NewRef(query)
			must(err)
			defer func() { must(m.DelRef(&queryRef)) }()
			status, err := e.QueryExpression(query)
			if mode == "checked" {
				if !errors.Is(err, rel.ErrCompilerUnsupported) {
					t.Fatal("unsupported explanation combination guard", status, err)
				}
				if _, err = e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("failed query published answer", err)
				}
				return
			}
			must(err)
			want := lbool.True
			if mode == "empty" {
				want = lbool.False
			}
			if status != want {
				t.Fatal(status, want)
			}
			answer, err := e.ExpressionAnswer()
			must(err)
			defer func() { must(m.DelRef(&answer)) }()
			if mode == "local-disabled" {
				if e.queryClone.datalogExplanationQuery {
					t.Fatal("module setting overrode local disable")
				}
			} else if e.queryClone.datalogExplanationAnswer == (ast.ASTRef{}) {
				t.Fatal("missing explanation answer")
			}
			if mode == "checked-default-lazy" || mode == "checked-default-lazy-sparse" || mode == "relation-checked-default-lazy" {
				if _, ok := e.predicates[d].table.(*rel.LazySparseTable); !ok {
					t.Fatalf("explanation query lost configured lazy seed storage: %T", e.predicates[d].table)
				}
			} else if mode == "checked-default-reverse" || mode == "relation-checked-default-reverse" {
				if _, ok := e.predicates[d].table.(*rel.HashTable); !ok {
					t.Fatalf("explanation query lost configured hash seed storage: %T", e.predicates[d].table)
				}
			} else if _, ok := e.predicates[d].table.(*rel.SparseTable); !ok {
				t.Fatalf("explanation query converted sparse seed storage: %T", e.predicates[d].table)
			}
		})
	}
}

func TestDatalogExpressionExplanationOpenQueryKeepsFiniteAndProofColumns(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	b, err := m.MkBoolSort()
	must(err)
	sym, err := m.Symbols.Intern("open-explanation-P")
	must(err)
	d, err := m.MkFuncDecl(sym, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(d))
	yes, err := m.MkTrue()
	must(err)
	must(e.AddFact(d, []ast.ASTHandle{yes}))
	x, err := m.MkVar(0, b)
	must(err)
	query, err := m.MkApp(d, []ast.ASTHandle{x})
	must(err)
	ref, err := m.NewRef(query)
	must(err)
	defer func() { must(m.DelRef(&ref)) }()
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	root, err := m.RefGet(answer)
	must(err)
	variables := map[uint32]ast.ASTHandle{}
	var visit func(ast.ASTHandle)
	visit = func(h ast.ASTHandle) {
		kind, err := m.GetKind(h)
		must(err)
		if kind == ast.ASTVar {
			index, err := m.GetVarIndex(h)
			must(err)
			sort, err := m.GetSort(h)
			must(err)
			variables[index] = sort
			return
		}
		if kind != ast.ASTApp {
			return
		}
		n, err := m.GetNumArgs(h)
		must(err)
		for i := uint32(0); i < n; i++ {
			arg, err := m.GetArg(h, i)
			must(err)
			visit(arg)
		}
	}
	visit(root)
	proofSort, err := m.MkDatalogRuleSort()
	must(err)
	if variables[0] != b || variables[1] != proofSort || len(variables) != 2 {
		t.Fatal("answer lost finite key or shifted proof column", variables)
	}
}

func TestDatalogRelationExplanationFormulaCheckConstantHeadWithoutPositiveTail(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "hashtable"))
	boolean, err := m.MkBoolSort()
	must(err)
	bits, err := m.BitVectorSort(2)
	must(err)
	name, err := m.Symbols.Intern("relation-formula-constant-head")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, bits}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(predicate))
	three, err := m.BitVectorNumeralUnsigned(3, 2)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	head, err := m.MkApp(predicate, []ast.ASTHandle{truth, three})
	must(err)
	headRef, err := m.NewRef(head)
	must(err)
	defer func() { must(m.DelRef(&headRef)) }()
	must(e.addRuleEager(head, "constant-head-no-tail"))
	status, err := e.QueryExpression(head)
	must(err)
	if status != lbool.True {
		t.Fatal("constant-head formula-checked explanation", status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	root, err := m.RefGet(answer)
	must(err)
	if !datalogExplanationFormulaNames(t, m, root)["constant-head-no-tail"] {
		t.Fatal("constant-head answer lacks named proof")
	}
}
