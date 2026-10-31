package api

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGeneratedParameterizedBitVectorReplay(t *testing.T) {
	cases := []struct {
		id    int
		args  string
		width uint32
		value uint64
	}{
		{107, "U 3\nU 0\nP 0x43\n", 4, 1}, {108, "U 4\nP 0x43\n", 12, 3969},
		{109, "U 4\nP 0x43\n", 12, 129}, {110, "U 2\nP 0x45\n", 8, 170},
		{114, "U 1\nP 0x43\n", 8, 3}, {115, "U 1\nP 0x43\n", 8, 192},
		{118, "U 8\nP 0x47\n", 8, 255},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nU 8\nC 35\n= 0x42\nR\nP 0x41\nU 129\nP 0x42\nC 151\n= 0x43\n" +
				"R\nP 0x41\nU 4\nC 35\n= 0x44\nR\nP 0x41\nU 10\nP 0x44\nC 151\n= 0x45\nR\nP 0x41\nC 33\n= 0x46\nR\nP 0x41\nI -1\nP 0x46\nC 150\n= 0x47\nR\nP 0x41\n" + tc.args + fmt.Sprintf("C %d\n= 0x48\nR\nP 0x41\nP 0x48\nC 900\n", tc.id)
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
			if err = r.RegisterCommand(900, "check-parameterized-bv", func(r *Z3Replayer) (err error) {
				owner, err := replayContext(r, 0)
				if err != nil {
					return err
				}
				actual, err := r.astObject(owner, 1)
				if err != nil {
					return err
				}
				sort, err := owner.context.SortOf(actual.handle)
				if err != nil {
					return err
				}
				width, err := owner.context.BitVectorSortSize(sort)
				if err != nil || width != tc.width {
					t.Fatalf("width %d want %d: %v", width, tc.width, err)
				}
				m := owner.results.manager
				var pins []ast.ASTRef
				defer func() {
					for i := len(pins) - 1; i >= 0; i-- {
						err = errors.Join(err, m.DelRef(&pins[i]))
					}
				}()
				expected, err := owner.context.UintNumeral(tc.value, sort)
				if err != nil {
					return err
				}
				ref, err := m.NewRef(expected)
				if err != nil {
					return err
				}
				pins = append(pins, ref)
				eq, err := owner.context.Equal(actual.handle, expected)
				if err != nil {
					return err
				}
				ref, err = m.NewRef(eq)
				if err != nil {
					return err
				}
				pins = append(pins, ref)
				neq, err := owner.context.Not(eq)
				if err != nil {
					return err
				}
				ref, err = m.NewRef(neq)
				if err != nil {
					return err
				}
				pins = append(pins, ref)
				solver, err := owner.context.NewSolverForLogic("QF_BV")
				if err != nil {
					return err
				}
				defer func() { err = errors.Join(err, solver.Close()) }()
				if err = solver.Assert(neq); err != nil {
					return err
				}
				status, err := solver.Check()
				if err != nil {
					return err
				}
				if status != lbool.False {
					t.Fatalf("value %d not established: %v", tc.value, status)
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

func TestGeneratedBitVectorToIntegerReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nU 8\nC 35\n= 0x42\nR\nP 0x41\nU 255\nP 0x42\nC 151\n= 0x43\nR\nP 0x41\nP 0x43\nI 0\nC 119\n= 0x44\nR\nP 0x41\nP 0x43\nI 1\nC 119\n= 0x45\nR\nP 0x41\nP 0x44\nP 0x45\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-bv2int", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		a, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		b, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		if a.handle == b.handle {
			t.Fatal("signed and unsigned conversions were merged")
		}
		for _, h := range []ast.ASTHandle{a.handle, b.handle} {
			sort, e := owner.context.SortOf(h)
			if e != nil {
				return e
			}
			kind, e := owner.context.SortKind(sort)
			if e != nil || kind != APISortInt {
				t.Fatalf("conversion sort %v: %v", kind, e)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}
