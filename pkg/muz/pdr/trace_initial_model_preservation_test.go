package pdr

// Source: model_node::get_rule in src/muz/pdr/pdr_context.cpp:790-824
// selects a rule from an existing model without replacing that model.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func initialTraceModelFixture(t *testing.T, proof bool) (*ast.ASTManager, *Context, ast.ASTHandle, *ModelNode, *normalizedRule) {
	t.Helper()
	m, context, rules := classifierContextFixture(t)
	closureMust(t, context.SetFullTraceAnswer(true))
	closureMust(t, context.SetResultValidation(false))
	closureMust(t, context.SetGenerateProofTrace(proof))
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	name, err := m.Symbols.Intern("trace-initial-model-P")
	closureMust(t, err)
	declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	closureMust(t, rules.RegisterPredicate(declaration))
	set, err := base.NewRuleSet(rules)
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, set.Close()) })
	for _, value := range []bool{false, true} {
		argument, err := m.MkFalse()
		name := "false-fact"
		if value {
			argument, err = m.MkTrue()
			name = "true-fact"
		}
		closureMust(t, err)
		head, err := m.MkApp(declaration, []ast.ASTHandle{argument})
		closureMust(t, err)
		rule, err := rules.NewRule(head, nil, nil, name)
		closureMust(t, err)
		closureMust(t, set.Add(rule))
		closureMust(t, rule.Close())
	}
	closureMust(t, context.Initialize(set))
	status, err := context.Solve(declaration)
	closureMust(t, err)
	if status != lbool.True {
		t.Fatal("initial facts were not reachable", status)
	}
	root, found, err := context.search.Root()
	closureMust(t, err)
	if !found || root.model == nil || root.selected != nil || root.rule != nil || root.level != 0 {
		t.Fatal("solve did not leave an unselected initial root with its actual model")
	}
	selected, err := root.transformer.selectRule(root.model)
	closureMust(t, err)
	if len(selected.predecessors) != 0 {
		t.Fatal("existing model did not already select an initial rule")
	}
	return m, context, declaration, root, selected
}

func initialTraceSolverChecks(t *testing.T, root *ModelNode) uint32 {
	t.Helper()
	var values statistics.Statistics
	closureMust(t, root.transformer.solver.context.solver.CollectStatistics(&values))
	var checks uint32
	for i := 0; i < values.Size(); i++ {
		if values.Key(i) == "num checks" && values.IsUint(i) {
			checks += values.UintValue(i)
		}
	}
	return checks
}

func TestTraceInitialModelPreservedByAnswerAndProof(t *testing.T) {
	for _, proof := range []bool{false, true} {
		name := "answer"
		if proof {
			name = "proof"
		}
		t.Run(name, func(t *testing.T) {
			m, context, declaration, root, selected := initialTraceModelFixture(t, proof)
			original, references := root.model, root.model.References
			checks := initialTraceSolverChecks(t, root)
			expected, err := selected.rule.Head()
			closureMust(t, err)
			for attempt := 0; attempt < 2; attempt++ {
				var result ast.ASTRef
				if proof {
					result, err = context.Proof(declaration)
				} else {
					result, err = context.Answer(declaration)
				}
				closureMust(t, err)
				if root.model != original || original.Closed || original.References != references {
					t.Fatal("trace export replaced or changed ownership of the solved model", attempt, original.Closed, original.References, references)
				}
				if root.selected != selected || root.rule == nil || initialTraceSolverChecks(t, root) != checks {
					t.Fatal("trace export failed to cache the existing model's initial rule without another solver check", attempt)
				}
				handle, err := m.RefGet(result)
				closureMust(t, err)
				if proof {
					closureMust(t, context.validateProofHandle(handle))
					handle, err = m.GetProofFact(handle)
					closureMust(t, err)
				}
				if !proof {
					// get_trace never keeps the root head (the query!N
					// predicate in the source), so a fact root exports true.
					truth, err := m.MkTrue()
					closureMust(t, err)
					if handle != truth {
						t.Fatal("answer kept the root head of the trace", attempt, handle)
					}
					closureMust(t, m.DelRef(&result))
					continue
				}
				conjuncts, err := m.FlattenAnd([]ast.ASTHandle{handle})
				closureMust(t, err)
				if len(conjuncts) != 1 || conjuncts[0] != expected {
					t.Fatal("trace did not export the initial fact selected by the solved model", attempt, conjuncts, expected)
				}
				closureMust(t, m.DelRef(&result))
			}
		})
	}
}

func TestTraceInitialSelectionAllocationFailurePreservesSolvedModel(t *testing.T) {
	m, context, _, root, selected := initialTraceModelFixture(t, false)
	original, references := root.model, root.model.References
	checks := initialTraceSolverChecks(t, root)
	var padding []ast.ASTRef
	t.Cleanup(func() {
		m.Budget().FailAfter(-1)
		closureMust(t, closeASTRefs(m, padding))
	})
	// Caller-owned empty references exhaust available slots without changing
	// the solved model, forcing selectRule's actual temporary reference growth.
	for i := 0; ; i++ {
		if i == 1024 {
			t.Fatal("reference-page boundary was not reached")
		}
		m.Budget().FailAfter(0)
		ref, err := m.NewRef(ast.ASTHandle{})
		m.Budget().FailAfter(-1)
		if errors.Is(err, arena.ErrInjectedFailure) {
			break
		}
		closureMust(t, err)
		padding = append(padding, ref)
	}
	counts := func() [4]uint32 {
		s := m.Stats()
		return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
	}
	before := counts()
	m.Budget().FailAfter(0)
	result, cause := context.selectedTraceRule(root)
	m.Budget().FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) || result != nil || root.selected != nil || root.rule != nil {
		t.Fatal("failed initial selection published a partial rule", result, cause)
	}
	if counts() != before || root.model != original || original.Closed || original.References != references || initialTraceSolverChecks(t, root) != checks {
		t.Fatal("failed initial selection changed the solved model or AST ownership", before, counts())
	}
	result, err := context.selectedTraceRule(root)
	closureMust(t, err)
	if result != selected || root.selected != selected || root.rule == nil || root.model != original || original.Closed || original.References != references || initialTraceSolverChecks(t, root) != checks {
		t.Fatal("initial selection retry did not reuse the solved model")
	}
}
