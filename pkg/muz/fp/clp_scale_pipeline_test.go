package fp

// Source: src/muz/clp/clp_context.cpp query and get_answer, and
// src/muz/base/dl_engine_base.h's default empty model.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCLPScaleQueryAnswerAndDefaultModel(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			for _, key := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
			}
			if enabled {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform.scale", "true"))
			}
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, e.Close()) }()
			fragmentMust(t, e.SelectEngine("clp"))
			var pins []ast.ASTRef
			defer func() {
				for i := len(pins) - 1; i >= 0; i-- {
					fragmentMust(t, m.DelRef(&pins[i]))
				}
			}()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				fragmentMust(t, err)
				ref, err := m.NewRef(h)
				fragmentMust(t, err)
				pins = append(pins, ref)
				return h
			}
			one := own(m.ArithmeticRealNumeralText("1"))
			realSort, err := m.GetSort(one)
			fragmentMust(t, err)
			boolean := own(m.MkBoolSort())
			decl := func(name string) ast.ASTHandle {
				symbol, err := m.Symbols.Intern(name)
				fragmentMust(t, err)
				d := own(m.MkFuncDecl(symbol, []ast.ASTHandle{realSort}, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(d))
				return d
			}
			p := decl("scale-clp-reachable")
			q := decl("scale-clp-unreachable")
			fragmentMust(t, e.addRuleEager(own(m.MkApp(p, []ast.ASTHandle{one})), "scale-clp-fact"))
			for _, query := range []struct {
				decl ast.ASTHandle
				want lbool.LBool
			}{{p, lbool.True}, {q, lbool.True}, {p, lbool.True}} {
				// clp::imp::query transforms the context's rules: the second
				// query's output predicates are the first query!N (whose
				// rule derives it) and its own, and get_output_predicate()
				// takes the hash table's first entry, as in z3 4.7.1.
				status, err := e.QueryPredicate(query.decl)
				fragmentMust(t, err)
				if status != query.want {
					t.Fatalf("query status = %v, want %v", status, query.want)
				}
				answer, err := e.Answer(query.decl)
				fragmentMust(t, err)
				value, err := m.RefGet(answer)
				fragmentMust(t, err)
				if !m.IsTrue(value) {
					t.Fatal("CLP answer is not true")
				}
				fragmentMust(t, m.DelRef(&answer))
				var certificate bytes.Buffer
				found, err := e.WriteCertificate(&certificate, query.decl, nil)
				fragmentMust(t, err)
				if !found || certificate.String() != "true\n" {
					t.Fatalf("CLP certificate = %q, found %v", certificate.String(), found)
				}
				first, err := e.Model(query.decl)
				fragmentMust(t, err)
				second, err := e.Model(query.decl)
				fragmentMust(t, err)
				checkEmptyEngineDefaultModel(t, first)
				checkEmptyEngineDefaultModel(t, second)
				fragmentMust(t, first.Close())
				checkEmptyEngineDefaultModel(t, second)
				fragmentMust(t, second.Close())
			}
			// clp::imp::query leaves the transformed program as the context's
			// rules; with scale on, its predicates gained the scale column.
			predicates, err := e.Predicates()
			fragmentMust(t, err)
			scaled := 0
			for _, predicate := range predicates {
				arity, err := m.GetArity(predicate)
				fragmentMust(t, err)
				if arity == 2 {
					scaled++
				}
			}
			if enabled != (scaled != 0) {
				t.Fatalf("scale enabled %v, scaled predicates %d", enabled, scaled)
			}
		})
	}
}
