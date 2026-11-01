package spacer

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
)

var errExtendedPrettyWriter = errors.New("extended pretty writer refused output")

type extendedPrettyFailingWriter struct{}

func (extendedPrettyFailingWriter) Write([]byte) (int, error) { return 0, errExtendedPrettyWriter }

func TestSpacerExtendedPrettyFailureOwnershipAndRetry(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 128 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, globals := newSpacerTestServices(t)
			owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := owner.Close(); err != nil {
					t.Error(err)
				}
			}()
			truth, err := m.MkTrue()
			if err != nil {
				t.Fatal(err)
			}
			input, err := m.NewRef(truth)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := m.DelRef(&input); err != nil {
					t.Error(err)
				}
			}()
			before := m.Stats().LiveReferences
			p := &PredicateTransformer{manager: m, owner: owner}
			var output bytes.Buffer
			m.Budget().FailAfter(stop)
			err = p.writeExtendedPretty(&output, truth)
			m.Budget().FailAfter(-1)
			if err != nil {
				var construction memory.ConstructionFailure
				if errors.As(err, &construction) {
					if cleanup := construction.Cleanup(); cleanup != nil {
						t.Fatal(cleanup)
					}
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected rewrite failure", err)
				}
				failures++
			} else {
				succeeded = true
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatalf("rewrite references = %d, want %d", got, before)
			}
			formats, err := m.GetFormatManager()
			if err != nil {
				t.Fatal(err)
			}
			if got := formats.Stats().LiveReferences; got != 0 {
				t.Fatal("failed rendering kept format references", got)
			}
			ref, err := rewriteExtendedPretty(m, truth)
			if err != nil {
				t.Fatal("retry", err)
			}
			if err = m.DelRef(&ref); err != nil {
				t.Fatal(err)
			}
			output.Reset()
			if err = p.writeExtendedPretty(&output, truth); err != nil || output.String() != "true" {
				t.Fatal("render retry", output.String(), err)
			}
			if err = p.writeExtendedPretty(extendedPrettyFailingWriter{}, truth); !errors.Is(err, errExtendedPrettyWriter) {
				t.Fatal("writer failure", err)
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatalf("writer failure references = %d, want %d", got, before)
			}
			if got := formats.Stats().LiveReferences; got != 0 {
				t.Fatal("writer failure kept format references", got)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("allocation frontier did not reach both failure and success", failures, succeeded)
	}
}
