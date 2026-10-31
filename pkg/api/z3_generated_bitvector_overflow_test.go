package api

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGeneratedBitVectorOverflowReplay(t *testing.T) {
	cases := []struct {
		id     int
		a, b   uint32
		signed string
		unary  bool
	}{
		{120, 255, 1, "I 0\n", false}, {120, 127, 1, "I 1\n", false},
		{121, 128, 255, "", false}, {122, 127, 255, "", false}, {123, 0, 1, "I 0\n", false},
		{124, 128, 255, "", false}, {125, 128, 0, "", true}, {126, 16, 16, "I 0\n", false}, {127, 192, 3, "", false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d-%d-%d-%q", tc.id, tc.a, tc.b, tc.signed), func(t *testing.T) {
			log := fmt.Sprintf("R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nU 8\nC 35\n= 0x42\nR\nP 0x41\nU %d\nP 0x42\nC 151\n= 0x43\nR\nP 0x41\nU %d\nP 0x42\nC 151\n= 0x44\nR\nP 0x41\nP 0x43\n", tc.a, tc.b)
			if !tc.unary {
				log += "P 0x44\n"
			}
			log += tc.signed + fmt.Sprintf("C %d\n= 0x45\nR\nP 0x41\nP 0x45\nC 900\n", tc.id)
			r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if err = RegisterZ3GeneratedContextCommands(r); err != nil {
				t.Fatal(err)
			}
			if err = RegisterZ3GeneratedASTCommands(r); err != nil {
				t.Fatal(err)
			}
			if err = r.RegisterCommand(900, "check-overflow", func(r *Z3Replayer) (err error) {
				owner, err := replayContext(r, 0)
				if err != nil {
					return err
				}
				predicate, err := r.astObject(owner, 1)
				if err != nil {
					return err
				}
				solver, err := owner.context.NewSolverForLogic("QF_BV")
				if err != nil {
					return err
				}
				defer func() { err = errors.Join(err, solver.Close()) }()
				if err = solver.Assert(predicate.handle); err != nil {
					return err
				}
				status, err := solver.Check()
				if err != nil {
					return err
				}
				if status != lbool.False {
					t.Fatalf("overflow predicate should be false: %v", status)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err = r.Parse(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
