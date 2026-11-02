package fp

// Source: horn_tactic.cpp:317-356; dl_transforms.cpp default transforms and
// dl_mk_slice.cpp optional column projection (no relational join pipeline).

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	smtcore "github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornSimplifyDefaultPipelineAndOptionalSlice(t *testing.T) {
	for _, mode := range []string{"inline", "plain", "slice", "source-slice-order"} {
		for _, reachable := range []bool{false, true} {
			name := mode + map[bool]string{false: "/sat", true: "/unsat"}[reachable]
			t.Run(name, func(t *testing.T) {
				m, g := newPDRLifecycleServices(t)
				for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.slice", "fixedpoint.xform.subsumption_checker"} {
					fragmentMust(t, g.SourceSetName(key, "false"))
				}
				if mode == "inline" {
					for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
						fragmentMust(t, g.SourceSetName(key, "true"))
					}
				}
				if mode == "slice" || mode == "source-slice-order" {
					fragmentMust(t, g.SourceSetName("fixedpoint.xform.slice", "true"))
				}
				// These are engine-only stages, absent from horn_tactic::simplify.
				fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "2"))
				fragmentMust(t, g.SourceSetName("fixedpoint.xform.coalesce_rules", "true"))
				var refs []ast.ASTRef
				defer func() {
					for i := len(refs) - 1; i >= 0; i-- {
						fragmentMust(t, m.DelRef(&refs[i]))
					}
				}()
				own := func(h ast.ASTHandle, err error) ast.ASTHandle {
					fragmentMust(t, err)
					r, e := m.NewRef(h)
					fragmentMust(t, e)
					refs = append(refs, r)
					return h
				}
				boolean := own(m.MkBoolSort())
				declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
					s, e := m.Symbols.Intern("simplify-pipeline-" + name)
					fragmentMust(t, e)
					return own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
				}
				p, q := declare("p", []ast.ASTHandle{boolean, boolean}), declare("q", []ast.ASTHandle{boolean})
				a, b, unused := declare("a", nil), declare("b", nil), declare("unused", nil)
				aa, bb, uu := own(m.MkApp(a, nil)), own(m.MkApp(b, nil)), own(m.MkApp(unused, nil))
				x, y := own(m.MkVar(0, boolean)), own(m.MkVar(1, boolean))
				falsity, truth := own(m.MkFalse()), own(m.MkTrue())
				quantify := func(body ast.ASTHandle, n int) ast.ASTHandle {
					sorts := make([]ast.ASTHandle, n)
					names := make([]symbol.Symbol, n)
					for i := range sorts {
						sorts[i] = boolean
						names[i] = symbol.Numerical(uint32(i))
					}
					return own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body}))
				}
				fact := quantify(own(m.MkApp(p, []ast.ASTHandle{falsity, x})), 1)
				body := own(m.MkAndArray([]ast.ASTHandle{own(m.MkApp(p, []ast.ASTHandle{x, y})), aa, bb}))
				if mode != "source-slice-order" {
					body = own(m.MkAndArray([]ast.ASTHandle{body, own(m.MkEq(x, falsity))}))
				}
				rule := quantify(own(m.MkImplies(body, own(m.MkApp(q, []ast.ASTHandle{x})))), 2)
				value := truth
				if reachable {
					value = falsity
				}
				query := own(m.MkNot(own(m.MkApp(q, []ast.ASTHandle{value}))))
				inputs := []ast.ASTHandle{fact, rule, aa, bb, uu, query}
				goal, e := tactic.NewGoal(m, false, true, false)
				fragmentMust(t, e)
				defer func() { fragmentMust(t, goal.Close()) }()
				for _, input := range inputs {
					fragmentMust(t, goal.Assert(input))
				}
				horn, e := NewHornSimplifyTactic(m, g.Context, g)
				fragmentMust(t, e)
				fragmentMust(t, horn.IncRef())
				out := &tactic.TacticResult{}
				defer func() { fragmentMust(t, out.Close()) }()
				fragmentMust(t, horn.Apply(goal, out))
				fragmentMust(t, horn.DecRef()) // Returned goals/filter must outlive the tactic.
				if len(out.Goals) != 1 || out.ModelConverter == nil {
					t.Fatal("simplify exports", len(out.Goals), out.ModelConverter)
				}
				seedModel, e := model.NewModel(m)
				fragmentMust(t, e)
				seed := seedModel.Core()
				fragmentMust(t, seed.RegisterConstant(unused, truth))
				var converted tactic.ModelRef
				fragmentMust(t, converted.Set(seed))
				defer func() { fragmentMust(t, converted.Close()) }()
				fragmentMust(t, out.ModelConverter.ApplyModel(&converted, 0))
				filtered, e := converted.Get()
				fragmentMust(t, e)
				kept, e := filtered.ConstantInterpretation(unused)
				fragmentMust(t, e)
				if !m.IsTrue(kept) {
					t.Fatal("query filter removed an original predicate")
				}
				size, e := out.Goals[0].Size()
				fragmentMust(t, e)
				sawP := false
				seen := make(map[ast.ASTHandle]bool)
				var visit func(ast.ASTHandle)
				visit = func(h ast.ASTHandle) {
					if seen[h] {
						return
					}
					seen[h] = true
					k, e := m.GetKind(h)
					fragmentMust(t, e)
					if k == ast.ASTQuantifier {
						b, e := m.GetQuantifierBody(h)
						fragmentMust(t, e)
						visit(b)
						return
					}
					if k != ast.ASTApp {
						return
					}
					d, e := m.GetDecl(h)
					fragmentMust(t, e)
					s, e := m.GetASTName(d)
					fragmentMust(t, e)
					label, e := m.Symbols.String(s)
					fragmentMust(t, e)
					if strings.Contains(label, "!filter") || strings.Contains(label, "!join") || d == unused {
						t.Fatal("non-source or irrelevant predicate survived", label)
					}
					if strings.HasPrefix(label, "simplify-pipeline-p") {
						sawP = true
						arity, e := m.GetArity(d)
						fragmentMust(t, e)
						want := uint32(2)
						if mode == "slice" {
							want = 1
						}
						if mode == "source-slice-order" {
							want = 0
						}
						if arity != want {
							t.Fatal("predicate slice arity", arity, want)
						}
					}
					n, e := m.GetNumArgs(h)
					fragmentMust(t, e)
					for i := uint32(0); i < n; i++ {
						a, e := m.GetArg(h, i)
						fragmentMust(t, e)
						visit(a)
					}
				}
				solver, e := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
				fragmentMust(t, e)
				defer func() { fragmentMust(t, solver.Close()) }()
				fragmentMust(t, solver.Push())
				for _, input := range inputs {
					fragmentMust(t, solver.AssertFormula(input))
				}
				original, e := solver.CheckSat(nil)
				fragmentMust(t, e)
				fragmentMust(t, solver.Pop(1))
				originalWant := lbool.True
				if reachable {
					originalWant = lbool.False
				}
				if original != originalWant {
					t.Fatal("original Horn semantics", original, originalWant)
				}
				for i := uint32(0); i < size; i++ {
					f, e := out.Goals[0].Form(i)
					fragmentMust(t, e)
					visit(f)
					fragmentMust(t, solver.AssertFormula(f))
				}
				if mode != "inline" && !sawP {
					t.Fatal("slice/plain regression did not exercise the source predicate")
				}
				got, e := solver.CheckSat(nil)
				fragmentMust(t, e)
				want := lbool.True
				// Z3 4.7.1 reproduces this legacy slice saturation defect:
				// the final ground Q argument blocks slicing too late to propagate
				// back to P. Its output drops both P columns and forces Q true.
				// Keep the source-compatibility oracle distinct from sound cases.
				if reachable || mode == "source-slice-order" {
					want = lbool.False
				}
				if got != want {
					t.Fatal("simplified Horn semantics", got, want)
				}
			})
		}
	}
}
