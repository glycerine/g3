package api

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGeneratedBitVectorReplaySemantics(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nU 8\nC 35\n= 0x42\n" +
		"R\nP 0x41\nI 5\nP 0x42\nC 150\n= 0x43\nR\nP 0x41\nI 3\nP 0x42\nC 150\n= 0x44\nR\nP 0x41\nU 255\nP 0x42\nC 151\n= 0x45\n" +
		"R\nP 0x41\nI 1\nP 0x42\nC 150\n= 0x46\nR\nP 0x41\nI 8\nP 0x42\nC 150\n= 0x47\nR\nP 0x41\nI 40\nP 0x42\nC 150\n= 0x48\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nC 83\n= 0x49\nR\nP 0x41\nP 0x43\nP 0x44\nC 90\n= 0x50\nR\nP 0x41\nP 0x43\nP 0x44\nC 111\n= 0x51\n" +
		"R\nP 0x41\nP 0x45\nP 0x44\nC 98\n= 0x52\nR\nP 0x41\nP 0x45\nP 0x44\nC 99\n= 0x53\n" +
		"R\nP 0x41\nP 0x49\nP 0x50\nP 0x51\nP 0x52\nP 0x53\nP 0x46\nP 0x47\nP 0x48\nC 900\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-bitvector-semantics", func(r *Z3Replayer) (err error) {
		owner, err := replayContext(r, 0)
		if err != nil {
			return err
		}
		m := owner.results.manager
		args := make([]ast.ASTHandle, 8)
		for i := range args {
			a, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			args[i] = a.handle
		}
		var pins []ast.ASTRef
		defer func() {
			for i := len(pins) - 1; i >= 0; i-- {
				err = errors.Join(err, m.DelRef(&pins[i]))
			}
		}()
		pin := func(h ast.ASTHandle) error {
			ref, e := m.NewRef(h)
			if e == nil {
				pins = append(pins, ref)
			}
			return e
		}
		var facts []ast.ASTHandle
		for _, pair := range [][2]int{{0, 5}, {1, 6}, {2, 7}} {
			h, e := owner.context.Equal(args[pair[0]], args[pair[1]])
			if e != nil {
				return e
			}
			if e = pin(h); e != nil {
				return e
			}
			facts = append(facts, h)
		}
		notUnsigned, e := owner.context.Not(args[3])
		if e != nil {
			return e
		}
		if e = pin(notUnsigned); e != nil {
			return e
		}
		facts = append(facts, notUnsigned, args[4])
		conjunction, e := owner.context.And(facts...)
		if e != nil {
			return e
		}
		if e = pin(conjunction); e != nil {
			return e
		}
		negated, e := owner.context.Not(conjunction)
		if e != nil {
			return e
		}
		if e = pin(negated); e != nil {
			return e
		}
		solver, e := owner.context.NewSolverForLogic("QF_BV")
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, solver.Close()) }()
		if e = solver.Assert(negated); e != nil {
			return e
		}
		status, e := solver.Check()
		if e != nil {
			return e
		}
		if status != lbool.False {
			t.Fatalf("bit-vector identities did not hold: %v", status)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}
