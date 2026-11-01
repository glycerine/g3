package spacer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestSpacerModelEvaluatorCatchesSourceResourceExceptions(t *testing.T) {
	for _, key := range []string{"max_steps", "max_memory"} {
		t.Run(key, func(t *testing.T) {
			m, globals := newSpacerTestServices(t)
			owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
			depthMust(t, err)
			defer func() { depthMust(t, owner.Close()) }()
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			a, err := m.MkFreshConst("limited-implicant", boolean)
			depthMust(t, err)
			literal, err := m.MkNot(a)
			depthMust(t, err)
			input, err := m.NewRef(literal)
			depthMust(t, err)
			defer func() { depthMust(t, m.DelRef(&input)) }()
			md, err := model.NewModel(m)
			depthMust(t, err)
			defer func() { depthMust(t, md.Close()) }()
			no, err := m.MkFalse()
			depthMust(t, err)
			decl, err := m.GetDecl(a)
			depthMust(t, err)
			depthMust(t, md.Core().RegisterConstant(decl, no))
			params, err := globals.Context.NewValue()
			depthMust(t, err)
			defer func() { depthMust(t, globals.Context.Del(&params)) }()
			depthMust(t, globals.Context.SetUIntName(&params, &key, 0))
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			tags.Enable("spacer_model_evaluator")
			config := owner.contexts[0].config.ModelEvaluator
			config.Params = &params
			config.Trace = tags
			expectedMessage := messages.MaxSteps
			if key == "max_memory" {
				expectedMessage = messages.MaxMemory
				block, err := config.Memory.Allocate(16)
				depthMust(t, err)
				defer func() { depthMust(t, config.Memory.Deallocate(&block)) }()
			}

			before := m.Stats().LiveReferences
			evaluator, err := newSpacerModelEvaluator(md.Core(), config)
			depthMust(t, err)
			truth, falsehood, err := evaluateSpacerModelBoolean(m, evaluator, config, literal)
			closeErr := evaluator.Close()
			depthMust(t, err)
			depthMust(t, closeErr)
			if truth || falsehood {
				t.Fatal("failed evaluation was not unknown", truth, falsehood)
			}

			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("caught evaluator exception leaked references", got, before)
			}
			if tags.ShouldTrace("spacer_model_evaluator") && !strings.Contains(output.String(), expectedMessage+"\n") {
				t.Fatal("missing evaluator exception trace", output.String())
			}
			config.Params = nil
			result, err := computeImplicantLiterals(m, md.Core(), config, []ast.ASTHandle{literal})
			depthMust(t, err)
			defer func() { depthMust(t, releaseNormalizeRefs(m, result)) }()
			if len(result) != 1 {
				t.Fatal("unlimited retry did not recover the literal", len(result))
			}
			actual, err := m.RefGet(result[0])
			depthMust(t, err)
			if actual != literal {
				t.Fatal("retry changed the literal")
			}
		})
	}
}
