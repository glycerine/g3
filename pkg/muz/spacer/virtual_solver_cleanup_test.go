package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestSpacerManagerResumesRefusedVirtualSolverClose(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	owner, err := NewManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	depthMust(t, err)
	defer func() { depthMust(t, owner.Close()) }()
	var views []*VirtualSolver
	for i := 0; i < 128; i++ {
		view, err := owner.Fresh(0)
		depthMust(t, err)
		views = append(views, view)
	}
	var pending *VirtualSolver
	for _, virtual := range views {
		guard := virtual.predicate
		manager.Budget().FailAfter(0)
		err = virtual.Close()
		manager.Budget().FailAfter(-1)
		if err == nil {
			continue
		}
		if !errors.Is(err, arena.ErrInjectedFailure) {
			t.Fatal("expected guard disabling allocation refusal", err)
		}
		if !virtual.closing || virtual.closed || virtual.predicate != guard {
			t.Fatal("refused close lost its pending guard owner")
		}
		if _, err := manager.RefGet(guard); err != nil {
			t.Fatal("refused close released its guard", err)
		}
		if _, err := virtual.CheckSat(nil); !errors.Is(err, ErrVirtualSolverOwner) {
			t.Fatal("closing solver accepted work", err)
		}
		pending = virtual
		break
	}
	if pending == nil {
		t.Fatal("fixture did not reach a guard-disabling allocation boundary")
	}
	for _, view := range views {
		if view != pending {
			depthMust(t, view.Close())
		}
	}
	depthMust(t, owner.Close())
	if !pending.closed || pending.closing {
		t.Fatal("manager did not finish pending close")
	}
	depthMust(t, owner.Close())
}
