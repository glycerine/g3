package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedArrayReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 33\n= 0x42\nR\nP 0x41\nC 32\n= 0x43\n" +
		"R\nP 0x41\n# 17\nU 10\nC 36\n= 0x44\n" +
		"R\nP 0x41\nP 0x42\nP 0x42\nC 37\n= 0x45\n" +
		"R\nP 0x41\nU 2\nP 0x42\nP 0x43\np 2\nP 0x42\nC 38\n= 0x46\n" +
		"R\nP 0x41\n$ |a|\nP 0x45\nC 51\n= 0x47\nR\nP 0x41\n$ |b|\nP 0x46\nC 51\n= 0x48\n" +
		"R\nP 0x41\nI 1\nP 0x42\nC 150\n= 0x49\nR\nP 0x41\nI 2\nP 0x42\nC 150\n= 0x50\nR\nP 0x41\nC 54\n= 0x51\n" +
		"R\nP 0x41\nP 0x47\nP 0x49\nP 0x50\nC 130\n= 0x52\nR\nP 0x41\nP 0x52\nP 0x49\nC 128\n= 0x53\n" +
		"R\nP 0x41\nP 0x48\nU 2\nP 0x49\nP 0x51\np 2\nP 0x50\nC 131\n= 0x54\n" +
		"R\nP 0x41\nP 0x54\nU 2\nP 0x49\nP 0x51\np 2\nC 129\n= 0x55\n" +
		"R\nP 0x41\nP 0x42\nP 0x50\nC 132\n= 0x56\nR\nP 0x41\nP 0x56\nC 134\n= 0x57\n" +
		"R\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x58\n" +
		"R\nP 0x41\nP 0x58\nU 1\nP 0x47\np 1\nC 133\n= 0x59\n" +
		"R\nP 0x41\nP 0x44\nP 0x45\nP 0x46\nP 0x53\nP 0x55\nP 0x57\nP 0x59\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-arrays", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		finite, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		size, e := owner.context.FiniteDomainSortSize(finite.handle)
		if e != nil || size != 10 {
			t.Fatalf("finite domain %d: %v", size, e)
		}
		name, e := owner.results.manager.GetASTName(finite.handle)
		if e != nil || !name.IsNumerical() || name.Value != 17 {
			t.Fatalf("finite name %v: %v", name, e)
		}
		for pos, want := range map[uint32]uint32{2: 1, 3: 2} {
			s, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			n, e := owner.context.ArrayArity(s.handle)
			if e != nil || n != want {
				t.Fatalf("array arity %d want %d: %v", n, want, e)
			}
		}
		for _, pos := range []uint32{4, 5, 6} {
			term, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			sort, e := owner.context.SortOf(term.handle)
			if e != nil {
				return e
			}
			kind, e := owner.context.SortKind(sort)
			if e != nil || kind != APISortInt {
				t.Fatalf("result sort %v: %v", kind, e)
			}
		}
		mapped, e := r.astObject(owner, 7)
		if e != nil {
			return e
		}
		sort, e := owner.context.SortOf(mapped.handle)
		if e != nil {
			return e
		}
		arity, e := owner.context.ArrayArity(sort)
		if e != nil || arity != 1 {
			t.Fatalf("map sort arity %d: %v", arity, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}
