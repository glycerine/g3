package fp

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExpressionBitBlastModelPreservesOriginalPredicates(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			f := newQueryVariablesOwnershipFixture(t)
			m, e := f.m, f.e
			fragmentMust(t, e.SelectEngine(backend))
			fragmentMust(t, e.globals.SourceSetName("fixedpoint.xform.bit_blast", "true"))
			for _, option := range []string{"slice", "inline_eager", "inline_linear", "subsumption_checker"} {
				fragmentMust(t, e.globals.SourceSetName("fixedpoint.xform."+option, "false"))
			}
			zero := f.own(m.BitVectorNumeralUnsigned(0, 2))
			one := f.own(m.BitVectorNumeralUnsigned(1, 2))
			two := f.own(m.BitVectorNumeralUnsigned(2, 2))
			predicate, fact := f.predicate("public-blast-model-P", zero)
			second := f.own(m.MkApp(predicate, []ast.ASTHandle{two}))
			query := f.own(m.MkApp(predicate, []ast.ASTHandle{one}))
			fragmentMust(t, e.addRuleEager(fact, "zero"))
			fragmentMust(t, e.addRuleEager(second, "two"))
			status, err := e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("bit-blasted query was not unreachable", status)
			}
			conversions := e.queryClone.pdrConversions
			if backend == "spacer" {
				conversions = e.queryClone.spacerConversions
			}
			var derived ast.ASTHandle
			for _, conversion := range conversions {
				if conversion.kind == hornConvertBitBlast {
					derived = conversion.predicates[predicate]
				}
			}
			if derived.IsNull() || derived == predicate {
				t.Fatal("public expression query did not reach bit-blast model conversion")
			}
			md, err := e.ExpressionModel()
			fragmentMust(t, err)
			t.Cleanup(func() { fragmentMust(t, md.Close()) })
			if present, err := md.Core().HasInterpretation(derived); err != nil || present {
				t.Fatal("model exposed its bit-blasted predicate", present, err)
			}
			status, err = e.QueryExpression(fact)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("replacement query lost its original fact", status)
			}
			params, globals := e.params, e.globals
			fragmentMust(t, e.Close())
			cooperation, err := cooperate.NewCooperation(m.Budget())
			fragmentMust(t, err)
			t.Cleanup(func() { fragmentMust(t, cooperation.Close()) })
			evaluator, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: params, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation})
			fragmentMust(t, err)
			t.Cleanup(func() { fragmentMust(t, evaluator.Close()) })
			fragmentMust(t, evaluator.SetModelCompletion(false))
			solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: params, Globals: globals})
			fragmentMust(t, err)
			t.Cleanup(func() { fragmentMust(t, solver.Close()) })
			for _, check := range []struct {
				expression ast.ASTHandle
				argument   ast.ASTHandle
				word       uint32
				truth      bool
			}{{fact, zero, 0, true}, {second, two, 2, true}, {query, one, 1, false}} {
				value, err := evaluator.EvaluateValue(check.expression)
				fragmentMust(t, err)
				body, err := m.RefGet(value)
				fragmentMust(t, err)
				// dl_mk_bit_blast.cpp:92-112 exports internal OP_BIT2BOOL
				// projections. Source bv_rewriter leaves them unreduced, and
				// theory_bv.cpp:589-600 warns against using them as ordinary
				// axioms. Supply each ground bit's independent arithmetic value
				// before asking SMT to refute the model obligation; the internal
				// operator itself is outside this solver oracle's supported path.
				substitution, err := m.NewExprSubstitution()
				fragmentMust(t, err)
				for bit := uint32(0); bit < 2; bit++ {
					projection := f.own(m.MkBitVectorBit2Bool(bit, check.argument))
					truth := f.own(m.MkFalse())
					if (check.word>>bit)&1 != 0 {
						truth = f.own(m.MkTrue())
					}
					fragmentMust(t, m.ExprSubstitutionInsert(substitution, projection, truth, ast.ASTHandle{}, ast.ASTDependency{}))
				}
				replacer, err := rewriter.NewDefaultExprReplacer(m)
				fragmentMust(t, err)
				fragmentMust(t, replacer.SetSubstitution(&substitution))
				normalized, err := m.NewRef(ast.ASTHandle{})
				fragmentMust(t, err)
				fragmentMust(t, replacer.Rewrite(body, normalized))
				fragmentMust(t, replacer.Close())
				fragmentMust(t, m.DelExprSubstitution(&substitution))
				obligation, err := m.RefGet(normalized)
				fragmentMust(t, err)
				if check.truth {
					obligation = f.own(m.MkNot(obligation))
				}
				fragmentMust(t, solver.Push())
				fragmentMust(t, solver.Assert(obligation))
				checked, err := solver.CheckSat(nil)
				fragmentMust(t, err)
				fragmentMust(t, solver.Pop(1))
				if checked != lbool.False {
					var dump bytes.Buffer
					fragmentMust(t, md.Core().WriteSMT2(&dump, params, globals, 0))
					t.Fatal("owning model changed an original fact or query after replacement/close", check.truth, checked, dump.String())
				}
				fragmentMust(t, m.DelRef(&normalized))
				fragmentMust(t, m.DelRef(&value))
			}
		})
	}
}
