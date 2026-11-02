package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestFixedpointMutationCleanupFailureInvalidatesSnapshots(t *testing.T) {
	for _, kind := range []string{"local-clear", "local-set", "module", "engine-switch"} {
		t.Run(kind, func(t *testing.T) {
			f := newSpacerGeneralizerExpressionFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			status, err := e.QueryExpression(f.query)
			must(err)
			if status != lbool.False {
				t.Fatal("expected unreachable expression query", status)
			}
			declaration, err := m.GetDecl(f.query)
			must(err)
			context := e.params
			parameters, err := context.NewValue()
			must(err)
			defer func() { must(context.Del(&parameters)) }()
			must(context.Assign(&parameters, &e.queryParams))
			name := "spacer.random_seed"
			must(context.SetUIntName(&parameters, &name, 7))
			mutate := func() error {
				switch kind {
				case "local-clear":
					return e.SetQueryParameters(nil)
				case "local-set":
					return e.SetQueryParameters(&parameters)
				case "module":
					return e.ParametersUpdated()
				default:
					return e.SelectEngine("pdr")
				}
			}
			if kind == "module" {
				must(e.globals.SourceSetName("fixedpoint.spacer.random_seed", "7"))
			}
			prior := e.queryClone
			m.Budget().FailAfter(0)
			err = mutate()
			m.Budget().FailAfter(-1)
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("expected disposal refusal", err)
			}
			if e.queryClone != prior || prior.closed {
				t.Fatal("mutation lost unfinished disposal owner")
			}
			if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("failed mutation exposed an old result", err)
			}
			if _, err := e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("failed mutation exposed an old answer", err)
			}
			if _, err := e.NumLevels(declaration); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("failed mutation exposed disposing query frames", err)
			}
			must(mutate())
			if !prior.closed || e.queryClone != nil {
				t.Fatal("mutation retry did not complete disposal")
			}
			if kind == "local-set" {
				seed, err := context.GetUIntName(&e.queryParams, &name, 0)
				must(err)
				if seed != 7 {
					t.Fatal("parameter retry lost the requested update", seed)
				}
			}
			if kind == "module" {
				settings, err := e.fixedpointParamView()
				must(err)
				seed, err := settings.UInt("spacer.random_seed", 0)
				closeErr := settings.Close()
				must(err)
				must(closeErr)
				if seed != 7 {
					t.Fatal("module update retry lost its new setting", seed)
				}
			}
			status, err = e.QueryPredicate(declaration)
			must(err)
			if status != lbool.True {
				t.Fatal("query did not recover after mutation retry", status)
			}
		})
	}
}
