package ast

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestPsortASTEncodingSelectionLifecycle(t *testing.T) {
	m := astTestManager(t)
	context, e := NewPsortASTContext(m)
	astTestMust(t, e)
	runtime, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
	astTestMust(t, e)
	network, e := NewPsortASTNetwork(context, runtime, m.BudgetField, nil)
	astTestMust(t, e)
	for encoding := PsortASTGrouped; encoding <= PsortASTCircuit; encoding++ {
		astTestMust(t, network.SetEncoding(encoding))
		if got, e := network.Encoding(); e != nil || got != encoding {
			t.Fatal("encoding selection", encoding, got, e)
		}
	}
	if e = network.SetEncoding(PsortASTCircuit + 1); !errors.Is(e, ErrASTDomain) {
		t.Fatal("invalid encoding accepted", e)
	}
	astTestMust(t, network.Close())
	if e = network.SetEncoding(PsortASTGrouped); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("closed encoding mutation", e)
	}
	astTestMust(t, runtime.Close())
	astTestMust(t, context.Close())
}
