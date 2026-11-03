package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalCallbackState struct {
	manager          *ast.ASTManager
	nonempty         map[ast.ASTHandle]bool
	operations       []int32
	cloneOutputs     []ast.ASTHandle
	joinArguments    [][]ast.ASTHandle
	joinParameters   [][]uint32
	refusalOperation int32
	refusalAt        int
	refusal          error
}

type externalIntervalNestedConstructionFailure struct {
	runtime *memory.Runtime
	refusal error
	calls   int
}

func (f *externalIntervalNestedConstructionFailure) Error() string {
	return "nested product construction failed"
}
func (f *externalIntervalNestedConstructionFailure) Unwrap() error        { return f.refusal }
func (f *externalIntervalNestedConstructionFailure) OriginalCause() error { return f.refusal }
func (f *externalIntervalNestedConstructionFailure) CleanupCause() error  { return f.refusal }
func (f *externalIntervalNestedConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.runtime
}
func (f *externalIntervalNestedConstructionFailure) Cleanup() error {
	f.calls++
	if f.calls == 1 {
		return f.refusal
	}
	return nil
}

func (s *externalIntervalCallbackState) callbacks() rel.ExternalRelationCallbacks {
	return rel.ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			op, err := s.manager.GetDeclKind(declaration)
			if err != nil {
				return err
			}
			s.operations = append(s.operations, op)
			if s.refusal != nil && op == s.refusalOperation && operationCount(s.operations, op) == s.refusalAt {
				return s.refusal
			}
			switch op {
			case ast.DatalogOpEmpty:
				s.nonempty[outputs[0]] = false
			case ast.DatalogOpClone:
				s.nonempty[outputs[0]] = s.nonempty[arguments[0]]
				s.cloneOutputs = append(s.cloneOutputs, outputs[0])
			case ast.DatalogOpUnion:
				s.nonempty[outputs[0]] = s.nonempty[arguments[0]] || s.nonempty[arguments[1]]
			case ast.DatalogOpStore:
				s.nonempty[outputs[0]] = true
			default:
				return fmt.Errorf("unexpected external/interval assignment operation %d", op)
			}
			return nil
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			op, err := s.manager.GetDeclKind(declaration)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			s.operations = append(s.operations, op)
			if s.refusal != nil && op == s.refusalOperation && operationCount(s.operations, op) == s.refusalAt {
				return ast.ASTHandle{}, s.refusal
			}
			switch op {
			case ast.DatalogOpJoin:
				if len(arguments) != 2 {
					return ast.ASTHandle{}, fmt.Errorf("unexpected external/interval JOIN argument count %d", len(arguments))
				}
				parameters, cause := s.manager.GetDeclParametersCopy(declaration)
				if cause != nil {
					return ast.ASTHandle{}, cause
				}
				indices := make([]uint32, 0, len(parameters))
				for i := range parameters {
					value, parameterErr := s.manager.ParameterInt(parameters[i])
					deleteErr := s.manager.DelParameter(&parameters[i])
					if parameterErr != nil || deleteErr != nil {
						return ast.ASTHandle{}, errors.Join(parameterErr, deleteErr)
					}
					indices = append(indices, uint32(value))
				}
				s.joinArguments = append(s.joinArguments, append([]ast.ASTHandle(nil), arguments...))
				s.joinParameters = append(s.joinParameters, indices)
				sort, cause := s.manager.GetRange(declaration)
				if cause != nil {
					return ast.ASTHandle{}, cause
				}
				result, cause := s.manager.MkFreshConst("external-interval-join", sort)
				if cause != nil {
					return ast.ASTHandle{}, cause
				}
				s.nonempty[result] = s.nonempty[arguments[0]] && s.nonempty[arguments[1]]
				return result, nil
			case ast.DatalogOpIsEmpty:
				if len(arguments) != 1 {
					return ast.ASTHandle{}, fmt.Errorf("unexpected external/interval ISEMPTY argument count %d", len(arguments))
				}
				return s.manager.MkBoolVal(!s.nonempty[arguments[0]])
			case ast.DatalogOpComplement:
				if len(arguments) != 1 {
					return ast.ASTHandle{}, fmt.Errorf("unexpected external/interval COMPLEMENT argument count %d", len(arguments))
				}
				s.nonempty[arguments[0]] = true
				return arguments[0], nil
			default:
				return ast.ASTHandle{}, fmt.Errorf("unexpected external/interval reduction operation %d", op)
			}
		},
	}
}

func newExternalIntervalDatalogFixture(t *testing.T) (*ast.ASTManager, *PropositionalEngine, *externalIntervalCallbackState) {
	t.Helper()
	manager, globals := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	t.Cleanup(func() {
		if !engine.closed {
			fragmentMust(t, engine.Close())
		}
	})
	fragmentMust(t, engine.SelectEngine("datalog"))
	state := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	fragmentMust(t, engine.SetExternalRelationCallbacks(state.callbacks()))
	return manager, engine, state
}

func declareExternalIntervalPredicate(t *testing.T, manager *ast.ASTManager, engine *PropositionalEngine, label string, sorts []ast.ASTHandle, kinds []string) ast.ASTHandle {
	t.Helper()
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	name, err := manager.Symbols.Intern(label)
	fragmentMust(t, err)
	declaration, err := manager.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(declaration))
	fragmentMust(t, engine.SetPredicateRepresentation(declaration, kinds))
	return declaration
}

func operationCount(operations []int32, operation int32) int {
	count := 0
	for _, current := range operations {
		if current == operation {
			count++
		}
	}
	return count
}

func TestExternalIntervalDatalogDirectFactsAndQueries(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			requested := append([]string(nil), kinds...)
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-direct-p-"+kinds[0], []ast.ASTHandle{integer}, requested)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-direct-q-"+kinds[0], []ast.ASTHandle{integer}, requested)
			boolean, err := manager.MkBoolSort()
			fragmentMust(t, err)
			ordinaryName, err := manager.Symbols.Intern("external-interval-unrelated-" + kinds[0])
			fragmentMust(t, err)
			ordinary, err := manager.MkFuncDecl(ordinaryName, nil, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, err)
			fragmentMust(t, engine.RegisterPredicate(ordinary))
			requested[0] = "changed-by-caller"
			if engine.predicates[p].requestedRepresentations[0] != kinds[0] || len(callbacks.operations) != 0 {
				t.Fatal("selection changed callback state or aliased caller", engine.predicates[p].requestedRepresentations, callbacks.operations)
			}

			status, err := engine.QueryPredicates([]ast.ASTHandle{p, q})
			fragmentMust(t, err)
			if status != lbool.False || operationCount(callbacks.operations, ast.DatalogOpEmpty) != 2 {
				t.Fatal("empty product query", status, callbacks.operations)
			}
			wantEmptyChecks := 0
			if kinds[0] == "external_relation" {
				wantEmptyChecks = 2
			}
			if got := operationCount(callbacks.operations, ast.DatalogOpIsEmpty); got != wantEmptyChecks {
				t.Fatal("component-order empty short circuit", got, wantEmptyChecks, callbacks.operations)
			}
			if _, err = engine.Answer(p); !errors.Is(err, ErrExternalIntervalDatalogFragment) {
				t.Fatal("mixed formula answer was not rejected", err)
			}

			seven, err := manager.ArithmeticIntegerNumeralText("7")
			fragmentMust(t, err)
			fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{seven}))
			status, err = engine.QueryPredicates([]ast.ASTHandle{q, p})
			fragmentMust(t, err)
			if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpStore) != 1 {
				t.Fatal("nonempty imprecise product query", status, callbacks.operations)
			}
			reason, err := engine.ReasonUnknown()
			fragmentMust(t, err)
			if reason != "approximated" {
				t.Fatal("imprecise product status", reason)
			}
			if _, err = engine.Answer(p); !errors.Is(err, ErrExternalIntervalDatalogFragment) {
				t.Fatal("nonempty mixed formula answer was not rejected", err)
			}
			if _, err = engine.RelationsAnswer(); !errors.Is(err, ErrExternalIntervalDatalogFragment) {
				t.Fatal("mixed relation-array formula answer was not rejected", err)
			}

			fragmentMust(t, engine.AddTableFact(q, []uint32{9}))
			status, err = engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpStore) != 2 {
				t.Fatal("table fact did not enter product", status, callbacks.operations)
			}
			status, err = engine.QueryPredicates(nil)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("zero-output relation query", status)
			}
		})
	}
}

func TestExternalIntervalDatalogRejectsUnsupportedPathsBeforeCallbacks(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-unsupported", []ast.ASTHandle{integer}, kinds)
	value, err := manager.ArithmeticIntegerNumeralText("3")
	fragmentMust(t, err)
	application, err := manager.MkApp(p, []ast.ASTHandle{value})
	fragmentMust(t, err)

	status, err := engine.QueryExpression(application)
	if status != lbool.Undef || !errors.Is(err, ErrExternalIntervalDatalogFragment) || engine.queryClone != nil || engine.externalIntervalDatalog != nil || len(callbacks.operations) != 0 {
		t.Fatal("expression query reached clone or callbacks", status, err, engine.queryClone, engine.externalIntervalDatalog, callbacks.operations)
	}
	variable, err := manager.MkVar(0, integer)
	fragmentMust(t, err)
	variableHead, err := manager.MkApp(p, []ast.ASTHandle{variable})
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(variableHead, "unsupported-product-variable-rule"))
	status, err = engine.QueryPredicate(p)
	if status != lbool.Undef || !errors.Is(err, ErrExternalIntervalDatalogFragment) || engine.externalIntervalDatalog != nil || len(callbacks.operations) != 0 {
		t.Fatal("ruleful query reached product construction", status, err, engine.externalIntervalDatalog, callbacks.operations)
	}
}

func TestExternalIntervalDatalogSynchronizesLatePredicatesWithoutReplacingFacts(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-late-p", []ast.ASTHandle{integer}, kinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
	if engine.externalIntervalDatalog == nil {
		t.Fatal("first fact did not activate product owner")
	}
	if err = engine.SetPredicateRepresentation(p, []string{"interval_relation", "external_relation"}); !errors.Is(err, ErrExternalIntervalDatalogFragment) {
		t.Fatal("materialized product order changed", err)
	}

	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-late-q", []ast.ASTHandle{integer}, kinds)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{two}))
	status, err := engine.QueryPredicates([]ast.ASTHandle{p, q})
	fragmentMust(t, err)
	if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpEmpty) != 2 || operationCount(callbacks.operations, ast.DatalogOpStore) != 2 {
		t.Fatal("late predicate sync replaced or lost facts", status, callbacks.operations)
	}
}

func TestExternalIntervalDatalogSelectionPreconditions(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, engine.Close()) })
	fragmentMust(t, engine.SelectEngine("datalog"))
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	name, err := manager.Symbols.Intern("external-interval-plugin-precondition")
	fragmentMust(t, err)
	p, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(p))
	kinds := []string{"external_relation", "interval_relation"}
	if err = engine.SetPredicateRepresentation(p, kinds); !errors.Is(err, ErrExternalIntervalDatalogFragment) || len(engine.predicates[p].requestedRepresentations) != 0 {
		t.Fatal("uninstalled external plugin selected", err, engine.predicates[p].requestedRepresentations)
	}
	callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	fragmentMust(t, engine.SetExternalRelationCallbacks(callbacks.callbacks()))
	fragmentMust(t, engine.SetPredicateRepresentation(p, kinds))

	name, err = manager.Symbols.Intern("external-interval-wrong-sort")
	fragmentMust(t, err)
	wrong, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(wrong))
	if err = engine.SetPredicateRepresentation(wrong, kinds); !errors.Is(err, ErrExternalIntervalDatalogFragment) {
		t.Fatal("non-arithmetic product signature selected", err)
	}

	if len(callbacks.operations) != 0 {
		t.Fatal("selection preflight invoked callbacks", callbacks.operations)
	}
}

func TestExternalIntervalDatalogRejectsPreexistingFiniteFacts(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	name, err := manager.Symbols.Intern("external-interval-preexisting-nullary")
	fragmentMust(t, err)
	nullary, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(nullary))
	fragmentMust(t, engine.AddTableFact(nullary, nil))
	if err = engine.SetPredicateRepresentation(nullary, []string{"external_relation", "interval_relation"}); !errors.Is(err, ErrExternalIntervalDatalogFragment) {
		t.Fatal("preexisting finite fact was silently replaced", err)
	}
	if len(callbacks.operations) != 0 {
		t.Fatal("preexisting-fact preflight invoked callbacks", callbacks.operations)
	}
}

func TestExternalIntervalDatalogClosesProductsBeforePluginAndRetries(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	fragmentMust(t, engine.SelectEngine("datalog"))
	callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	fragmentMust(t, engine.SetExternalRelationCallbacks(callbacks.callbacks()))
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-close-order", []ast.ASTHandle{integer}, []string{"external_relation", "interval_relation"})
	value, err := manager.ArithmeticIntegerNumeralText("11")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{value}))
	standalone, err := engine.NewExternalRelation([]ast.ASTHandle{integer})
	fragmentMust(t, err)

	if err = engine.Close(); !errors.Is(err, rel.ErrTableRelation) || engine.externalIntervalDatalog != nil || engine.external == nil || engine.closed {
		t.Fatal("first close did not retire product before live plugin user", err, engine.externalIntervalDatalog, engine.external, engine.closed)
	}
	fragmentMust(t, standalone.Close())
	fragmentMust(t, engine.Close())
	if !engine.closed || engine.external != nil || engine.externalIntervalDatalog != nil {
		t.Fatal("close retry left product/plugin owners", engine.closed, engine.external, engine.externalIntervalDatalog)
	}
}

func TestExternalIntervalDatalogConstructionCleanupRetriesNestedOwnerFirst(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-nested-cleanup", []ast.ASTHandle{integer}, []string{"external_relation", "interval_relation"})
	predicates, selected, err := engine.externalIntervalPredicates()
	fragmentMust(t, err)
	if !selected {
		t.Fatal("product predicate not selected")
	}
	activation, err := newExternalIntervalDatalogActivation(manager, engine.external, predicates)
	fragmentMust(t, err)
	refusal := errors.New("nested product cleanup refused")
	nested := &externalIntervalNestedConstructionFailure{runtime: manager.NumeralService().RuntimeOwner(), refusal: refusal}
	cause := finishExternalIntervalDatalogConstruction(nested, activation)
	var receipt *externalIntervalDatalogConstructionFailure
	if !errors.As(cause, &receipt) || nested.calls != 1 || !errors.Is(receipt.CleanupCause(), refusal) {
		t.Fatal("nested cleanup refusal was not preserved", cause, nested.calls)
	}
	status, queryErr := activation.query([]ast.ASTHandle{p})
	fragmentMust(t, queryErr)
	if status != lbool.False {
		t.Fatal("nested refusal changed activation before retry", status)
	}
	fragmentMust(t, receipt.Cleanup())
	if nested.calls != 2 || activation.open() == nil {
		t.Fatal("cleanup retry did not advance nested then activation", nested.calls, activation.open())
	}
	if operationCount(callbacks.operations, ast.DatalogOpEmpty) != 1 {
		t.Fatal("cleanup retry reconstructed product", callbacks.operations)
	}
}
