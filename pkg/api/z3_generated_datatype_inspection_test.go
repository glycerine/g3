package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedDatatypeInspectionReplayBorrowedResults(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |Pair|\nU 2\n$ |left|\n$ |right|\ns 2\nP 0x42\nP 0x42\np 2\nP 0\nP 0\nP 0\np 2\nC 39\n= 0x43\n* 0x44 5\n@ 0x45 6 0\n@ 0x46 6 1\n" +
		"R\nP 0x41\nC 900\nR\nP 0x41\nP 0x43\nC 212\n= 0x47\nR\nP 0x41\nP 0x43\nC 213\n" +
		"R\nP 0x41\nP 0x43\nU 1\nC 214\n= 0x48\nR\nP 0x41\nP 0x43\nC 215\n" +
		"R\nP 0x41\nP 0x43\nU 0\nC 216\n= 0x49\nR\nP 0x41\nP 0x43\nU 0\nC 217\n= 0x50\n" +
		"R\nP 0x41\nP 0x43\nU 0\nU 0\nC 218\n= 0x51\n" +
		"R\nP 0x41\nP 0x44\nP 0x47\nP 0x46\nP 0x48\nP 0x44\nP 0x49\nP 0x45\nP 0x51\nC 901\n"
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
	var trail int
	if err = r.RegisterCommand(900, "capture-datatype-trail", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		trail = len(owner.results.astTrail)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(901, "check-datatype-results", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		if len(owner.results.astTrail) != trail {
			t.Fatal("datatype inspection changed AST trail")
		}
		for pos := uint32(1); pos <= 8; pos += 2 {
			a, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			b, e := r.astObject(owner, pos+1)
			if e != nil {
				return e
			}
			if a.handle != b.handle {
				t.Fatal("datatype identity", pos, a.handle, b.handle)
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
		t.Fatal("datatype inspection not checked")
	}
}

func TestGeneratedRelationInspectionReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\nP 0x42\nC 900\n= 0x43\nR\nP 0x41\nP 0x43\nC 220\n" +
		"R\nP 0x41\nP 0x43\nU 0\nC 221\n= 0x44\nR\nP 0x41\nP 0x42\nP 0x44\nC 901\n"
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
	if err = r.RegisterCommand(900, "make-relation-sort", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		column, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		m := owner.results.manager
		if _, e = m.RegisterDatalogPlugin(); e != nil {
			return e
		}
		sort, e := m.MkDatalogRelationSort([]ast.ASTHandle{column.handle})
		if e != nil {
			return e
		}
		return r.storeAST(owner, sort)
	}); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(901, "check-relation-column", func(r *Z3Replayer) error {
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
		if a.handle != b.handle {
			t.Fatal("relation column", a.handle, b.handle)
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
		t.Fatal("relation column not checked")
	}
}

func TestGeneratedDatatypeUpdateReplayReportsSourceError(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |Pair|\nU 1\n$ |field|\ns 1\nP 0x42\np 1\nP 0\nP 0\np 1\nC 39\n= 0x43\n* 0x44 5\n@ 0x45 6 0\n" +
		"R\nP 0x41\nC 54\n= 0x46\nR\nP 0x41\nP 0x44\nU 1\nP 0x46\np 1\nC 50\n= 0x47\n" +
		"R\nP 0x41\nP 0x45\nP 0x47\nP 0x46\nC 219\n"
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
	if err = r.Parse(); err == nil || !strings.Contains(err.Error(), ast.ErrASTDomain.Error()) {
		t.Fatal("source field-update error", err)
	}
}
