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
