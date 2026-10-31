package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedDatatypeReplayMultipleResults(t *testing.T) {
	log := "R\nP 0\nC 7\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\nP 0x42\nC 9\nR\nP 0x41\nC 13\n= 0x60\n" +
		"R\nP 0x41\n$ |Pair|\nU 2\n$ |left|\n# 42\ns 2\nP 0x42\nP 0x42\np 2\nP 0\nP 0\nP 0\np 2\nC 39\n= 0x43\n* 0x44 5\n@ 0x45 6 0\n@ 0x46 6 1\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nP 0x45\nP 0x46\nC 900\n" +
		"R\nP 0x41\n$ |Choice|\nU 2\n$ |red|\n# 7\ns 2\nP 0\nP 0\np 2\nP 0\nP 0\np 2\nC 40\n= 0x47\n@ 0x48 4 0\n@ 0x49 4 1\n@ 0x50 5 0\n@ 0x51 5 1\n" +
		"R\nP 0x41\nP 0x47\nP 0x48\nP 0x49\nP 0x50\nP 0x51\nC 901\n" +
		"R\nP 0x41\n$ |BooleanList|\nP 0x42\nP 0\nP 0\nP 0\nP 0\nP 0\nP 0\nC 41\n= 0x52\n* 0x53 3\n* 0x54 4\n* 0x55 5\n* 0x56 6\n* 0x57 7\n* 0x58 8\n" +
		"R\nP 0x41\nP 0x52\nP 0x53\nP 0x54\nP 0x55\nP 0x56\nP 0x57\nP 0x58\nC 902\nR\nP 0x41\nP 0x42\nC 10\nR\nP 0x41\nC 8\n"
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
	if err = RegisterZ3GeneratedParameterCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-tuple", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		if previous := r.parameterSets[r.heap[0x60]]; previous == nil || !previous.object.closed {
			t.Fatal("datatype construction did not release the previous temporary object")
		}
		sort, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		ctor, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		arity, e := owner.context.DeclarationArity(ctor.handle)
		if e != nil || arity != 2 {
			t.Fatalf("tuple constructor arity %d: %v", arity, e)
		}
		actual, e := owner.context.TupleConstructor(sort.handle)
		if e != nil || actual != ctor.handle {
			t.Fatal("tuple constructor mismatch", e)
		}
		field, e := r.astObject(owner, 4)
		if e != nil {
			return e
		}
		sym, e := owner.results.manager.GetASTName(field.handle)
		if e != nil || !sym.IsNumerical() || sym.Value != 42 {
			t.Fatalf("field symbol %v: %v", sym, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-enumeration", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		second, e := r.astObject(owner, 3)
		if e != nil {
			return e
		}
		sym, e := owner.results.manager.GetASTName(second.handle)
		if e != nil || !sym.IsNumerical() || sym.Value != 7 {
			t.Fatalf("constructor name %v: %v", sym, e)
		}
		for _, pos := range []uint32{1, 2, 4, 5} {
			if _, e = r.astObject(owner, pos); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(902, "check-list", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos := uint32(1); pos <= 7; pos++ {
			if _, e = r.astObject(owner, pos); e != nil {
				return e
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
