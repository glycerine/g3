package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// runExplanationsMode is the C API reference program for
// datalog.generate_explanations (Z3_fixedpoint_query, rel_context::query
// over an expression, which applies mk_explanations): edges e over a
// finite domain of size 4, path as its transitive closure, node and bad
// for negation and interpreted tails. Its output is the program's: the
// result of each query and the answer (or the reason unknown), errors as
// the error handler prints them.
// newAPIProgramFixture is the C API reference programs' context:
// Z3_mk_context (no logic, no solver until one is used). The ASTs the
// program obtains stay in its trail (api::context::save_ast_trail of a
// context without user reference counts) until the context is deleted,
// released first to last.
func newAPIProgramFixture(t *testing.T) *smtAPIFixture {
	t.Helper()
	c, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	m, err := c.Manager()
	astTestMust(t, err)
	trail := new([]ast.ASTRef)
	t.Cleanup(func() {
		for i := range *trail {
			astTestMust(t, m.DelRef(&(*trail)[i]))
		}
	})
	return &smtAPIFixture{t: t, c: c, m: m, trail: trail}
}

// programDecl is the reference programs' Z3_mk_func_decl of a relation
// (a Bool range), kept in the trail.
func (f *smtAPIFixture) programDecl(name string, domain []ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	b, err := f.m.MkBoolSort()
	astTestMust(f.t, err)
	sym, err := f.m.Symbols.Intern(name)
	astTestMust(f.t, err)
	return f.pin(f.m.MkFuncDecl(sym, domain, b, ast.ASTFuncDeclInfo{}))
}

func runExplanationsMode(t *testing.T, mode int) string {
	sf := newAPIProgramFixture(t)
	fixedpoint, err := sf.c.NewFixedpoint()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, fixedpoint.Close()) })
	f := &apiFixedpointFixture{smtAPIFixture: sf, fp: fixedpoint}
	var out strings.Builder
	params, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetSymbol("engine", "datalog"))
	astTestMust(t, params.SetBool("datalog.generate_explanations", true))
	if mode == 1 || mode == 5 {
		astTestMust(t, params.SetBool("datalog.explanations_on_relation_level", true))
	}
	if mode == 2 {
		astTestMust(t, params.SetBool("datalog.magic_sets_for_queries", true))
	}
	if mode == 6 {
		astTestMust(t, params.SetSymbol("datalog.default_table", "hashtable"))
	}
	if mode == 8 {
		astTestMust(t, params.SetSymbol("datalog.default_relation", "doc"))
	}
	if mode == 9 {
		astTestMust(t, params.SetSymbol("datalog.check_relation", "sparse"))
	}
	if mode == 11 || mode == 12 {
		astTestMust(t, params.SetBool("datalog.explanations_on_relation_level", true))
	}
	if mode == 12 {
		astTestMust(t, params.SetBool("datalog.magic_sets_for_queries", true))
	}
	if mode == 14 {
		astTestMust(t, params.SetSymbol("datalog.default_table", "bitvector"))
	}
	if mode == 15 {
		astTestMust(t, params.SetSymbol("datalog.default_table", "lazy_sparse"))
	}
	if mode == 16 {
		astTestMust(t, params.SetBool("xform.inline_eager", false))
	}
	astTestMust(t, f.fp.SetParameters(params))
	astTestMust(t, params.Close())
	s := f.pin(f.c.FiniteDomainSort("S", 4))
	ss := []ast.ASTHandle{s, s}
	e := f.programDecl("e", ss)
	path := f.programDecl("path", ss)
	node := f.programDecl("node", ss[:1])
	bad := f.programDecl("bad", ss[:1])
	for _, d := range []ast.ASTHandle{e, path, node, bad} {
		astTestMust(t, f.fp.RegisterRelation(d))
	}
	astTestMust(t, f.fp.AddTableFact(e, []uint32{0, 1}))
	astTestMust(t, f.fp.AddTableFact(e, []uint32{1, 2}))
	if mode != 4 {
		astTestMust(t, f.fp.AddTableFact(e, []uint32{2, 3}))
	}
	astTestMust(t, f.fp.AddTableFact(bad, []uint32{2}))
	x := f.pin(f.c.BoundVariable(0, s))
	y := f.pin(f.c.BoundVariable(1, s))
	z := f.pin(f.c.BoundVariable(2, s))
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		return f.pin(f.m.MkApp(d, args))
	}
	and := func(args ...ast.ASTHandle) ast.ASTHandle { return f.pin(f.m.MkAndArray(args)) }
	implies := func(a, b ast.ASTHandle) ast.ASTHandle { return f.pin(f.m.MkImplies(a, b)) }
	rule := func(body ast.ASTHandle, name string, nv int) {
		sorts := []ast.ASTHandle{s, s, s}[:nv]
		names := []string{"v0", "v1", "v2"}[:nv]
		r := f.pin(f.c.Forall(sorts, names, body))
		if err := f.fp.AddRule(r, name); err != nil {
			fmt.Fprintf(&out, "error: %v\n", err)
		}
	}
	rule(implies(app(e, x, y), app(path, x, y)), "base", 2)
	stepName := "step"
	if mode == 3 {
		stepName = ""
	}
	rule(implies(and(app(path, x, y), app(e, y, z)), app(path, x, z)), stepName, 3)
	if mode == 3 || mode == 7 || mode == 11 {
		rule(implies(and(app(e, x, y), f.pin(f.m.MkNot(app(bad, x)))), app(node, x)), "", 2)
	}
	if mode == 4 {
		rule(implies(and(app(path, x, y), f.pin(f.m.MkNot(f.pin(f.m.MkEq(x, y))))), app(node, y)), "diff", 2)
	}
	c0 := f.pin(f.c.Numeral("0", s))
	c3 := f.pin(f.c.Numeral("3", s))
	var q ast.ASTHandle
	switch mode {
	case 3, 4, 7, 11:
		q = f.pin(f.c.Exists(ss[:1], []string{"a"}, app(node, x)))
	case 13:
		q = f.pin(f.c.Exists(ss[:1], []string{"a"}, app(path, c0, x)))
	case 5:
		q = app(path, c0, c3)
	default:
		q = f.pin(f.c.Exists(ss, []string{"a", "b"}, app(path, x, y)))
	}
	query := func(q ast.ASTHandle) { explanationQueryOutput(t, f, &out, q) }
	query(q)
	if mode == 0 {
		query(app(path, c0, c3))
	}
	return out.String()
}

// TestAPIFixedpointExplanations: AUDIT.md FUZZ-F5 (relation plugins).
// datalog.generate_explanations runs mk_explanations with the explanation
// relation plugin and finite_product_relation through the compiled
// relation pipeline. The expected outputs are the fixed fork's
// (test_vectors/explanations/expl.c, mode_N.txt); modes 4 and 8 are the
// errors of FORK_BUGFIXES.md's fixes (tst_z3_4_7_1_fpr_filter_interpreted,
// tst_z3_4_7_1_explanations_doc_union), where z3 4.7.1 crashed.
func TestAPIFixedpointExplanations(t *testing.T) {
	for _, mode := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 13, 14, 15, 16} {
		t.Run(strconv.Itoa(mode), func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("test_vectors", "explanations", "mode_"+strconv.Itoa(mode)+".txt"))
			astTestMust(t, err)
			if got := runExplanationsMode(t, mode); got != string(want) {
				t.Fatalf("mode %d:\n%s\nwant\n%s", mode, got, want)
			}
		})
	}
}

// runExplanationScenario is the C API reference program for explanation
// configurations (relation level, check_relation, checked default tables,
// explicit representations, magic sets, negation, doc relations).
func runExplanationScenario(t *testing.T, s int) string {
	sf := newAPIProgramFixture(t)
	fixedpoint, err := sf.c.NewFixedpoint()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, fixedpoint.Close()) })
	f := &apiFixedpointFixture{smtAPIFixture: sf, fp: fixedpoint}
	var out strings.Builder
	params, err := f.c.NewParameterSet()
	astTestMust(t, err)
	setb := func(k string, v bool) { astTestMust(t, params.SetBool(k, v)) }
	sets := func(k, v string) { astTestMust(t, params.SetSymbol(k, v)) }
	setp := func() { astTestMust(t, f.fp.SetParameters(params)) }
	defer func() { astTestMust(t, params.Close()) }()
	b := f.pin(f.c.BoolSort())
	decl := func(name string, domain ...ast.ASTHandle) ast.ASTHandle {
		d := f.programDecl(name, domain)
		astTestMust(t, f.fp.RegisterRelation(d))
		return d
	}
	repr := func(d ast.ASTHandle, k string) { astTestMust(t, f.fp.SetPredicateRepresentation(d, k)) }
	fact := func(d ast.ASTHandle, v ...uint32) { astTestMust(t, f.fp.AddTableFact(d, v)) }
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle { return f.pin(f.m.MkApp(d, args)) }
	and := func(args ...ast.ASTHandle) ast.ASTHandle { return f.pin(f.m.MkAndArray(args)) }
	not := func(a ast.ASTHandle) ast.ASTHandle { return f.pin(f.m.MkNot(a)) }
	implies := func(a, c ast.ASTHandle) ast.ASTHandle { return f.pin(f.m.MkImplies(a, c)) }
	forall := func(s ast.ASTHandle, n int, body ast.ASTHandle) ast.ASTHandle {
		return f.pin(f.c.Forall([]ast.ASTHandle{s, s}[:n], []string{"x", "y"}[:n], body))
	}
	rule := func(body ast.ASTHandle, name string) {
		if err := f.fp.AddRule(body, name); err != nil {
			fmt.Fprintf(&out, "error: %v\n", err)
		}
	}
	tt := f.pin(f.m.MkTrue())
	ff := f.pin(f.m.MkFalse())
	x := f.pin(f.c.BoundVariable(0, b))
	y := f.pin(f.c.BoundVariable(1, b))
	query := func(q ast.ASTHandle) { explanationQueryOutput(t, f, &out, q) }
	sets("engine", "datalog")
	setb("datalog.generate_explanations", true)
	fmt.Fprintf(&out, "== scenario %d\n", s)
	switch {
	case s >= 1 && s <= 3:
		setb("datalog.explanations_on_relation_level", true)
		sets("datalog.check_relation", "hashtable")
		setb("datalog.unbound_compressor", false)
		setp()
		p, q := decl("p", b), decl("q", b)
		var n ast.ASTHandle
		if s == 3 {
			n = decl("n")
		} else {
			n = decl("n", b)
		}
		if s == 2 {
			repr(n, "tr_sparse")
		} else {
			repr(n, "tr_hashtable")
		}
		fact(p, 1)
		fact(p, 0)
		if s != 3 {
			fact(n, 1)
		}
		var na ast.ASTHandle
		if s == 3 {
			na = app(n)
		} else {
			na = app(n, x)
		}
		rule(forall(b, 1, implies(and(app(p, x), not(na)), app(q, x))), "negproof")
		if s == 3 {
			query(app(q, tt))
		} else {
			query(app(q, ff))
		}
	case s == 4:
		setb("datalog.magic_sets_for_queries", true)
		sets("datalog.default_table", "hashtable")
		setp()
		seed, pred := decl("seed", b), decl("pred", b)
		repr(pred, "tr_sparse")
		fact(seed, 1)
		rule(forall(b, 1, implies(app(seed, x), app(pred, x))), "step")
		query(app(pred, tt))
	case s >= 5 && s <= 8:
		magic, derived := s == 6 || s == 8, s >= 7
		setb("datalog.magic_sets_for_queries", magic)
		setp()
		p, q, u := decl("p"), decl("q"), decl("u")
		fact(p)
		if derived {
			rule(implies(app(p), app(u)), "producer")
		}
		rule(implies(and(app(p), not(app(u))), app(q)), "consumer")
		query(app(q))
		if !derived {
			fact(u)
			query(app(q))
		}
	case s == 9 || s == 10:
		setb("datalog.explanations_on_relation_level", true)
		setb("datalog.magic_sets_for_queries", s == 9)
		sets("datalog.check_relation", "check")
		if s == 9 {
			sets("datalog.default_table", "hashtable")
			setb("datalog.default_table_checked", true)
			sets("datalog.default_table_checker", "sparse")
			setb("datalog.unbound_compressor", false)
		}
		setp()
		if s == 9 {
			seed, edge, reach, goal := decl("seed", b), decl("edge", b, b), decl("reach", b), decl("goal", b)
			repr(seed, "tr_sparse")
			repr(edge, "tr_hashtable")
			fact(seed, 0)
			fact(edge, 0, 1)
			rule(forall(b, 1, implies(app(seed, x), app(reach, x))), "base")
			rule(forall(b, 2, implies(and(app(reach, x), app(edge, x, y)), app(reach, y))), "recursive")
			rule(forall(b, 1, implies(app(reach, x), app(goal, x))), "goalrule")
			query(app(goal, tt))
		} else {
			p := decl("p")
			fact(p)
			query(app(p))
		}
	case s >= 11 && s <= 22:
		k := s - 11
		modes := []string{"enabled", "magic", "relation-level", "checked", "relation-checked", "relation-checked-sparse",
			"relation-checked-explicit", "checked-default", "checked-default-reverse", "relation-checked-default", "checked-default-lazy", "empty"}
		fmt.Fprintf(&out, "mode %s\n", modes[k])
		if k == 1 {
			setb("datalog.magic_sets_for_queries", true)
		}
		if k == 3 || k == 5 {
			sets("datalog.check_relation", "sparse")
		}
		if k == 4 || k == 6 {
			sets("datalog.check_relation", "hashtable")
		}
		if k == 2 || k == 4 || k == 5 || k == 6 || k == 9 {
			setb("datalog.explanations_on_relation_level", true)
		}
		if k == 7 || k == 9 {
			setb("datalog.default_table_checked", true)
			sets("datalog.default_table_checker", "hashtable")
		}
		if k == 8 {
			sets("datalog.default_table", "hashtable")
			setb("datalog.default_table_checked", true)
			sets("datalog.default_table_checker", "sparse")
		}
		if k == 10 {
			sets("datalog.default_table", "lazy_sparse")
			setb("datalog.default_table_checked", true)
			sets("datalog.default_table_checker", "hashtable")
		}
		setp()
		P, R := decl("P", b), decl("R", b)
		if k == 6 {
			repr(R, "tr_sparse")
		}
		if k != 11 {
			fact(P, 1)
		}
		rule(forall(b, 1, implies(app(P, x), app(R, x))), "forward")
		rule(forall(b, 1, implies(app(R, x), app(P, x))), "backward")
		query(app(R, tt))
	case s >= 25 && s <= 28:
		def := []string{"sparse", "lazy_sparse", "sparse", "lazy_sparse"}[s-25]
		chk := []string{"sparse", "sparse", "lazy_sparse", "lazy_sparse"}[s-25]
		fmt.Fprintf(&out, "mode checked-%s-%s\n", def, chk)
		sets("datalog.default_table", def)
		setb("datalog.default_table_checked", true)
		sets("datalog.default_table_checker", chk)
		setp()
		P, R := decl("P", b), decl("R", b)
		fact(P, 1)
		rule(forall(b, 1, implies(app(P, x), app(R, x))), "forward")
		rule(forall(b, 1, implies(app(R, x), app(P, x))), "backward")
		query(app(R, tt))
	case s == 23 || s == 24:
		setb("datalog.explanations_on_relation_level", true)
		if s == 24 {
			sets("datalog.check_relation", "doc")
		} else {
			sets("datalog.default_relation", "doc")
		}
		setp()
		bv := f.pin(f.c.BitVectorSort(3))
		p, q, r, bs := decl("pl", bv), decl("qd", bv), decl("rr", bv), decl("bs", b)
		v := f.pin(f.c.Numeral("5", bv))
		xb := f.pin(f.c.BoundVariable(0, bv))
		fact(p, 5)
		fact(r, 5)
		fact(bs, 1)
		rule(forall(bv, 1, implies(and(app(p, xb), app(r, xb), app(bs, tt), f.pin(f.m.MkEq(xb, v))), app(q, xb))), "joinproject")
		query(app(q, v))
	}
	return out.String()
}

// explanationQueryOutput is the reference programs' query(): the result,
// then the answer or the reason unknown; errors as the error handler
// prints them (Z3_EXCEPTION).
func explanationQueryOutput(t *testing.T, f *apiFixedpointFixture, out *strings.Builder, q ast.ASTHandle) {
	status, err := f.fp.QueryExpression(q)
	if err != nil {
		msg := err.Error()
		var exception z3exception.Z3Exception
		if errors.As(err, &exception) {
			msg = exception.Message()
		}
		fmt.Fprintf(out, "error 12: %s\n", msg)
	}
	r := 0
	switch status {
	case lbool.True:
		r = 1
	case lbool.False:
		r = -1
	}
	fmt.Fprintf(out, "result %d\n", r)
	if status == lbool.Undef {
		reason, err := f.fp.ReasonUnknown()
		astTestMust(t, err)
		fmt.Fprintf(out, "reason %s\n", reason)
		return
	}
	answer, err := f.fp.LastAnswer()
	astTestMust(t, err)
	h, err := f.m.RefGet(answer)
	astTestMust(t, err)
	text, err := f.c.ASTString(h, APIASTPrintSMTFull)
	astTestMust(t, err)
	if f.trail != nil {
		// Z3_fixedpoint_get_answer's result stays in the context's trail.
		*f.trail = append(*f.trail, answer)
	} else {
		astTestMust(t, f.m.DelRef(&answer))
	}
	fmt.Fprintf(out, "%s\n", text)
}

// TestAPIFixedpointExplanationScenarios: the configurations of the
// explanation queries (relation level, check_relation, checked default
// tables, explicit and inherited table representations, magic sets,
// negation, doc relations) against the fixed fork
// (test_vectors/explanations/scen.c, scenario_N.txt). Scenarios 26-28
// make lazy_sparse tables of the check tables of a finite product
// relation, whose join-project keeps functional columns.
func TestAPIFixedpointExplanationScenarios(t *testing.T) {
	for s := 1; s <= 28; s++ {
		t.Run(strconv.Itoa(s), func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("test_vectors", "explanations", "scenario_"+strconv.Itoa(s)+".txt"))
			astTestMust(t, err)
			if got := runExplanationScenario(t, s); got != string(want) {
				t.Fatalf("scenario %d:\n%s\nwant\n%s", s, got, want)
			}
		})
	}
}
