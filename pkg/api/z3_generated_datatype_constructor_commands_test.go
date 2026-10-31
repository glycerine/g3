package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedDatatypeConstructorReplayLifecycle(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n# 7\n$ |is-box|\nU 1\n$ |field|\ns 1\nP 0x42\np 1\nU 0\nu 1\nC 42\n= 0x43\n" +
		"R\nP 0x41\nU 1\nP 0x43\np 1\nC 45\n= 0x44\nR\nP 0x41\nP 0x44\nC 46\nR\nP 0x41\nP 0x43\nC 900\n" +
		"R\nP 0x41\n$ |nil|\n$ |is-nil|\nU 0\ns 0\np 0\nu 0\nC 42\n= 0x46\nR\nP 0x41\nP 0x46\nC 43\n" +
		"R\nP 0x41\nU 1\nP 0x43\np 1\nC 45\n= 0x45\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-constructor-after-list-delete", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		id, e := r.Object(1)
		if e != nil {
			return e
		}
		c := r.datatypeConstructors[id]
		if c == nil || c.owner != owner {
			return ErrZ3ReplayReference
		}
		if e = c.value.open(); e != nil {
			return e
		}
		if !c.value.definition.Name.IsNumerical() || len(c.value.sortRefs) != 1 {
			t.Fatal("constructor metadata")
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var constructor *replayDatatypeConstructor
	var list *replayDatatypeList
	for _, value := range r.datatypeConstructors {
		constructor = value
	}
	for _, value := range r.datatypeLists {
		list = value
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if !checked || constructor == nil || !constructor.value.closed || list == nil || !list.value.closed {
		t.Fatal("incomplete constructor cleanup")
	}
}

func TestGeneratedDatatypeConstructorReplayRejectsShortArrays(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\n$ |box|\n$ |is-box|\nU 1\ns 0\np 0\nu 0\nC 42\n"
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
	if err = r.Parse(); err == nil || !strings.Contains(err.Error(), ErrZ3ReplayReference.Error()) {
		t.Fatal("short constructor arrays", err)
	}
}

func TestGeneratedDatatypeConstructorReplayBuildAndQuery(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |box|\n$ |is-box|\nU 1\n$ |value|\ns 1\nP 0x42\np 1\nU 0\nu 1\nC 42\n= 0x43\n" +
		"R\nP 0x41\n$ |Box|\nU 1\nP 0x43\np 1\nC 44\n= 0x44\n" +
		"R\nP 0x41\nP 0x43\nU 1\nP 0\nP 0\nP 0\np 1\nC 48\n* 0x45 3\n* 0x46 4\n@ 0x47 5 0\n" +
		"R\nP 0x41\nP 0x45\nC 9\nR\nP 0x41\nP 0x46\nC 9\nR\nP 0x41\nP 0x47\nC 9\nR\nP 0x41\nP 0x43\nC 43\n" +
		"R\nP 0x41\nC 54\n= 0x48\nR\nP 0x41\nP 0x45\nU 1\nP 0x48\np 1\nC 50\n= 0x49\n" +
		"R\nP 0x41\nP 0x47\nU 1\nP 0x49\np 1\nC 50\n= 0x50\nR\nP 0x41\nP 0x50\nC 287\n= 0x51\n" +
		"R\nP 0x41\nP 0x46\nU 1\nP 0x49\np 1\nC 50\n= 0x52\nR\nP 0x41\nP 0x52\nC 287\n= 0x53\n" +
		"R\nP 0x41\nP 0x51\nP 0x53\nC 900\nR\nP 0x41\nP 0x45\nC 10\nR\nP 0x41\nP 0x46\nC 10\nR\nP 0x41\nP 0x47\nC 10\n"
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
	checked := false
	if err = r.RegisterCommand(900, "check-datatype-accessors", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos := uint32(1); pos <= 2; pos++ {
			value, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			if !owner.results.manager.IsTrue(value.handle) {
				t.Fatal("constructor query result", pos)
			}
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
		t.Fatal("datatype outputs not checked")
	}
}

func TestGeneratedDatatypeConstructorReplayMutualRecursion(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\n" +
		"R\nP 0x41\n$ |zero|\n$ |is-zero|\nU 0\ns 0\np 0\nu 0\nC 42\n= 0x42\n" +
		"R\nP 0x41\n$ |even-next|\n$ |is-even-next|\nU 1\n$ |odd-pred|\ns 1\nP 0\np 1\nU 1\nu 1\nC 42\n= 0x43\n" +
		"R\nP 0x41\n$ |odd-next|\n$ |is-odd-next|\nU 1\n$ |even-pred|\ns 1\nP 0\np 1\nU 0\nu 1\nC 42\n= 0x44\n" +
		"R\nP 0x41\nU 2\nP 0x42\nP 0x43\np 2\nC 45\n= 0x45\nR\nP 0x41\nU 1\nP 0x44\np 1\nC 45\n= 0x46\n" +
		"R\nP 0x41\nU 2\n$ |Even|\n$ |Odd|\ns 2\nP 0\nP 0\np 2\nP 0x45\nP 0x46\np 2\nC 47\n@ 0x47 3 0\n@ 0x48 3 1\n" +
		"R\nP 0x41\nP 0x47\nC 9\nR\nP 0x41\nP 0x48\nC 9\nR\nP 0x41\nP 0x45\nC 46\nR\nP 0x41\nP 0x46\nC 46\n" +
		"R\nP 0x41\nP 0x42\nC 43\nR\nP 0x41\nP 0x43\nC 43\nR\nP 0x41\nP 0x44\nC 43\n" +
		"R\nP 0x41\nP 0x47\nP 0x48\nC 900\nR\nP 0x41\nP 0x47\nC 10\nR\nP 0x41\nP 0x48\nC 10\n"
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
	checked := false
	if err = r.RegisterCommand(900, "check-mutual-datatypes", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		even, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		odd, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		for _, row := range []struct {
			sort, target *replayASTOwner
			constructor  uint32
		}{{even, odd, 1}, {odd, even, 0}} {
			accessor, e := owner.context.DatatypeAccessor(row.sort.handle, row.constructor, 0)
			if e != nil {
				return e
			}
			rangeSort, e := owner.results.manager.GetRange(accessor)
			if e != nil {
				return e
			}
			if rangeSort != row.target.handle {
				t.Fatal("mutual recursive accessor sort")
			}
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
		t.Fatal("mutual datatype outputs not checked")
	}
}
