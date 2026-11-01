package spacer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSpacerImplicantITEShortCircuitsSourceModelProbes(t *testing.T) {
	for _, name := range []string{"both-true", "both-false", "condition-true", "condition-false"} {
		t.Run(name, func(t *testing.T) {
			m, globals := newSpacerTestServices(t)
			owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
			depthMust(t, err)
			defer func() { depthMust(t, owner.Close()) }()
			var pins []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				depthMust(t, err)
				ref, err := m.NewRef(h)
				depthMust(t, err)
				pins = append(pins, ref)
				return h
			}
			boolean := own(m.MkBoolSort())
			constant := func(name string) ast.ASTHandle {
				symbol, err := m.Symbols.Intern(name)
				depthMust(t, err)
				return own(m.MkConst(symbol, boolean))
			}
			c, a, b := constant("ite-probe-c"), constant("ite-probe-a"), constant("ite-probe-b")
			truth, falsity := own(m.MkTrue()), own(m.MkFalse())
			formula := own(m.MkIte(c, a, b))
			md, err := model.NewModel(m)
			depthMust(t, err)
			defer func() { depthMust(t, md.Close()) }()
			values := []ast.ASTHandle{truth, truth, falsity}
			expected := []ast.ASTHandle{a, c}
			conditionCalls := 4 // One condition probe and three positive-literal probes.
			elseCalls := 1
			switch name {
			case "both-true":
				values = []ast.ASTHandle{ast.ASTHandle{}, truth, truth}
				expected = []ast.ASTHandle{b, a}
				conditionCalls = 0
				elseCalls = 4
			case "both-false":
				values = []ast.ASTHandle{ast.ASTHandle{}, falsity, falsity}
				formula = own(m.MkNot(formula))
				expected = []ast.ASTHandle{own(m.MkNot(b)), own(m.MkNot(a))}
				conditionCalls = 0
				elseCalls = 3
			case "condition-false":
				values[0] = falsity
				formula = own(m.MkNot(formula))
				expected = []ast.ASTHandle{own(m.MkNot(b)), own(m.MkNot(c))}
				conditionCalls = 4 // Two condition probes and two negative-literal probes.
				elseCalls = 3
			}
			for i, term := range []ast.ASTHandle{c, a, b} {
				if values[i].IsNull() {
					continue
				}
				decl, err := m.GetDecl(term)
				depthMust(t, err)
				depthMust(t, md.Core().RegisterConstant(decl, values[i]))
			}
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			tags.Enable("model_evaluator")
			config := owner.contexts[0].config.ModelEvaluator
			config.Trace = tags
			picked, err := computeImplicantLiterals(m, md.Core(), config, []ast.ASTHandle{formula})
			depthMust(t, err)
			defer func() { depthMust(t, releaseNormalizeRefs(m, picked)) }()
			if len(picked) != len(expected) {
				t.Fatal("wrong ITE implicant length", len(picked))
			}
			for i, ref := range picked {
				h, err := m.RefGet(ref)
				depthMust(t, err)
				if h != expected[i] {
					t.Fatal("wrong ITE implicant literal", i, h, expected[i])
				}
			}
			if tags.ShouldTrace("model_evaluator") {
				probe := "-------- [model_evaluator] operator() src/model/model_evaluator.cpp:572 ---------\nite-probe-c\n"
				if got := strings.Count(output.String(), probe); got != conditionCalls {
					t.Fatalf("condition evaluated %d times, want %d\n%s", got, conditionCalls, output.String())
				}
				elseProbe := "-------- [model_evaluator] operator() src/model/model_evaluator.cpp:572 ---------\nite-probe-b\n"
				if got := strings.Count(output.String(), elseProbe); got != elseCalls {
					t.Fatalf("else branch evaluated %d times, want %d", got, elseCalls)
				}
			}
		})
	}
}
