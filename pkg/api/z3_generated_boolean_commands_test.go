package api

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedBooleanReplay(t *testing.T) {
	cases := []struct {
		id   int
		args string
		kind int32
	}{
		{57, "U 2\nP 0x42\nP 0x43\np 2\n", ast.ASTOpDistinct},
		{59, "P 0x42\nP 0x43\nP 0x42\n", ast.ASTOpIte},
		{60, "P 0x42\nP 0x43\n", ast.ASTOpIff},
		{61, "P 0x42\nP 0x43\n", ast.ASTOpImplies},
		{62, "P 0x42\nP 0x43\n", ast.ASTOpXor},
		{63, "U 2\nP 0x42\nP 0x43\np 2\n", ast.ASTOpAnd},
		{64, "U 2\nP 0x42\nP 0x43\np 2\n", ast.ASTOpOr},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\nR\nP 0x41\nC 55\n= 0x43\nR\nP 0x41\n" + tc.args + fmt.Sprintf("C %d\n= 0x44\nR\nP 0x41\nP 0x44\nC 900\n", tc.id)
			r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := r.Close(); err != nil {
					t.Error(err)
				}
			}()
			if err = RegisterZ3GeneratedContextCommands(r); err != nil {
				t.Fatal(err)
			}
			if err = RegisterZ3GeneratedASTCommands(r); err != nil {
				t.Fatal(err)
			}
			if err = r.RegisterCommand(900, "check-kind", func(r *Z3Replayer) error {
				owner, e := replayContext(r, 0)
				if e != nil {
					return e
				}
				value, e := r.astObject(owner, 1)
				if e != nil {
					return e
				}
				kind, e := owner.results.manager.GetDeclKind(value.handle)
				if e != nil || kind != tc.kind {
					t.Fatalf("kind %v, want %v: %v", kind, tc.kind, e)
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

func TestGeneratedBooleanReplayRejectsOversizedArray(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nU 1\np 0\nC 63\n"
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
	if err = r.Parse(); err == nil {
		t.Fatal("accepted array shorter than declared count")
	}
}
