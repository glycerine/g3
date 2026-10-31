package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedModelUniverseVectorReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\n$ |U|\nC 31\n= 0x42\n" +
		"R\nP 0x41\n$ |u|\nP 0x42\nC 51\n= 0x43\nR\nP 0x41\nC 295\n= 0x44\nR\nP 0x41\nP 0x44\nC 296\n" +
		"R\nP 0x41\nP 0x44\nP 0x42\nP 0x43\nC 900\nR\nP 0x41\nP 0x44\nP 0x42\nC 308\n= 0x45\n" +
		"R\nP 0x41\nP 0x45\nC 448\nR\nP 0x41\nP 0x44\nC 297\n" +
		"R\nP 0x41\nP 0x45\nU 0\nC 451\n= 0x46\nR\nP 0x41\nP 0x45\nP 0x43\nP 0x46\nC 901\nR\nP 0x41\nP 0x45\nC 449\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedModelCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.RegisterCommand(900, "register-universe", func(r *Z3Replayer) error {
		m, e := replayGetModel(r)
		if e != nil {
			return e
		}
		sort, e := r.astObject(m.owner, 2)
		if e != nil {
			return e
		}
		value, e := r.astObject(m.owner, 3)
		if e != nil {
			return e
		}
		return m.value.RegisterUniverse(sort.handle, []ast.ASTHandle{value.handle})
	}); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(901, "check-universe-vector", func(r *Z3Replayer) error {
		v, e := replayGetVector(r)
		if e != nil {
			return e
		}
		a, e := r.astObject(v.owner, 2)
		if e != nil {
			return e
		}
		b, e := r.astObject(v.owner, 3)
		if e != nil {
			return e
		}
		n, e := v.value.Len()
		if e != nil {
			return e
		}
		if n != 1 || a.handle != b.handle {
			t.Fatal("universe vector", n, a.handle, b.handle)
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("universe was not checked")
	}
}

func TestGeneratedSolverAssertionAndCoreVectorsReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |x|\nP 0x42\nC 51\n= 0x43\n" +
		"R\nP 0x41\nP 0x43\nC 58\n= 0x44\nR\nP 0x41\nC 408\n= 0x45\nR\nP 0x41\nP 0x45\nC 416\n" +
		"R\nP 0x41\nP 0x45\nP 0x43\nC 422\nR\nP 0x41\nP 0x45\nC 424\n= 0x46\nR\nP 0x41\nP 0x46\nC 448\n" +
		"R\nP 0x41\nP 0x45\nU 1\nP 0x44\np 1\nC 428\nR\nP 0x41\nP 0x45\nC 433\n= 0x47\nR\nP 0x41\nP 0x47\nC 448\n" +
		"R\nP 0x41\nP 0x45\nC 420\nR\nP 0x41\nP 0x46\nU 0\nC 451\n= 0x48\nR\nP 0x41\nP 0x46\nP 0x43\nP 0x48\nC 900\n" +
		"R\nP 0x41\nP 0x47\nU 0\nC 451\n= 0x49\nR\nP 0x41\nP 0x47\nP 0x44\nP 0x49\nC 900\n" +
		"R\nP 0x41\nP 0x46\nC 449\nR\nP 0x41\nP 0x47\nC 449\nR\nP 0x41\nP 0x45\nC 417\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedSolverCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checks := 0
	if err = r.RegisterCommand(900, "check-solver-vector", func(r *Z3Replayer) error {
		v, e := replayGetVector(r)
		if e != nil {
			return e
		}
		a, e := r.astObject(v.owner, 2)
		if e != nil {
			return e
		}
		b, e := r.astObject(v.owner, 3)
		if e != nil {
			return e
		}
		n, e := v.value.Len()
		if e != nil {
			return e
		}
		if n != 1 || a.handle != b.handle {
			t.Fatal("solver vector", n, a.handle, b.handle)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checks != 2 {
		t.Fatal("missing solver vector checks", checks)
	}
}

func TestGeneratedSolverConsequencesReplayAppendsOutput(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |p|\nP 0x42\nC 51\n= 0x43\n" +
		"R\nP 0x41\nC 54\n= 0x44\nR\nP 0x41\nC 408\n= 0x45\nR\nP 0x41\nP 0x45\nC 416\nR\nP 0x41\nP 0x45\nP 0x43\nC 422\n" +
		"R\nP 0x41\nC 447\n= 0x46\nR\nP 0x41\nP 0x46\nC 448\nR\nP 0x41\nC 447\n= 0x47\nR\nP 0x41\nP 0x47\nC 448\n" +
		"R\nP 0x41\nP 0x47\nP 0x43\nC 454\nR\nP 0x41\nC 447\n= 0x48\nR\nP 0x41\nP 0x48\nC 448\nR\nP 0x41\nP 0x48\nP 0x44\nC 454\n" +
		"R\nP 0x41\nP 0x45\nP 0x46\nP 0x47\nP 0x48\nC 430\nR\nP 0x41\nP 0x45\nC 417\n" +
		"R\nP 0x41\nP 0x48\nP 0x44\nP 0x42\nC 900\nR\nP 0x41\nP 0x46\nC 449\nR\nP 0x41\nP 0x47\nC 449\nR\nP 0x41\nP 0x48\nC 449\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedSolverCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-consequence-vector", func(r *Z3Replayer) error {
		v, e := replayGetVector(r)
		if e != nil {
			return e
		}
		existing, e := r.astObject(v.owner, 2)
		if e != nil {
			return e
		}
		boolSort, e := r.astObject(v.owner, 3)
		if e != nil {
			return e
		}
		size, e := v.value.Len()
		if e != nil {
			return e
		}
		if size != 2 {
			t.Fatal("consequences were not appended", size)
		}
		first, e := v.value.At(0)
		if e != nil {
			return e
		}
		second, e := v.value.At(1)
		if e != nil {
			return e
		}
		sort, e := v.owner.context.SortOf(second)
		if e != nil {
			return e
		}
		if first != existing.handle || sort != boolSort.handle {
			t.Fatal("consequence vector contents")
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("consequences were not checked")
	}
}
