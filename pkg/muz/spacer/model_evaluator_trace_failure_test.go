package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSpacerModelEvaluatorTraceWriterFailureKeepsCaughtExceptionUnknown(t *testing.T) {
	m, globals := newSpacerTestServices(t)
	owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	depthMust(t, err)
	defer func() { depthMust(t, owner.Close()) }()
	boolean, err := m.MkBoolSort()
	depthMust(t, err)
	a, err := m.MkFreshConst("trace-failure-implicant", boolean)
	depthMust(t, err)
	literal, err := m.MkNot(a)
	depthMust(t, err)
	pin, err := m.NewRef(literal)
	depthMust(t, err)
	defer func() { depthMust(t, m.DelRef(&pin)) }()
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()
	no, err := m.MkFalse()
	depthMust(t, err)
	declaration, err := m.GetDecl(a)
	depthMust(t, err)
	depthMust(t, md.Core().RegisterConstant(declaration, no))
	params, err := globals.Context.NewValue()
	depthMust(t, err)
	defer func() { depthMust(t, globals.Context.Del(&params)) }()
	key := "max_steps"
	depthMust(t, globals.Context.SetUIntName(&params, &key, 0))
	tags := diagnostic.NewTraceTagsWithWriter(extendedPrettyFailingWriter{})
	tags.Enable("spacer_model_evaluator")
	config := owner.contexts[0].config.ModelEvaluator
	config.Params, config.Trace = &params, tags
	before := m.Stats().LiveReferences
	evaluator, err := newSpacerModelEvaluator(md.Core(), config)
	depthMust(t, err)
	truth, falsehood, err := evaluateSpacerModelBoolean(m, evaluator, config, literal)
	depthMust(t, err)
	depthMust(t, evaluator.Close())
	if truth || falsehood {
		t.Fatal("caught evaluator exception was not unknown")
	}
	if m.Stats().LiveReferences != before {
		t.Fatal("trace writer failure leaked evaluator references")
	}
	config.Params = nil
	result, err := computeImplicantLiterals(m, md.Core(), config, []ast.ASTHandle{literal})
	depthMust(t, err)
	defer func() { depthMust(t, releaseNormalizeRefs(m, result)) }()
	if len(result) != 1 {
		t.Fatal("fresh evaluator did not recover after trace failure", len(result))
	}
}
