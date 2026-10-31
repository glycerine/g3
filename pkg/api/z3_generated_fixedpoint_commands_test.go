package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGeneratedFixedpointReplaySnapshotsAndLifecycle(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |R|\nU 0\np 0\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\nC 511\n= 0x44\nR\nP 0x41\nP 0x44\nC 512\nR\nP 0x41\nP 0x44\nP 0x43\nC 526\n" +
		"R\nP 0x41\nP 0x44\nC 536\nR\nP 0x41\nP 0x44\nC 537\n" +
		"R\nP 0x41\nP 0x44\nC 528\n= 0x45\nR\nP 0x41\nP 0x45\nC 448\nR\nP 0x41\nP 0x44\nC 529\n= 0x46\nR\nP 0x41\nP 0x46\nC 448\n" +
		"R\nP 0x41\nP 0x44\nC 525\n= 0x47\nR\nP 0x41\nP 0x47\nC 438\nR\nP 0x41\nP 0x44\nC 513\n" +
		"R\nP 0x41\nP 0x45\nP 0x46\nP 0x47\nC 900\nR\nP 0x41\nP 0x45\nC 449\nR\nP 0x41\nP 0x46\nC 449\nR\nP 0x41\nP 0x47\nC 439\n"
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
	if err = r.RegisterCommand(900, "check-fixedpoint-snapshots", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos := uint32(1); pos <= 2; pos++ {
			v, e := replayVectorAt(r, owner, pos)
			if e != nil {
				return e
			}
			size, e := v.value.Len()
			if e != nil {
				return e
			}
			if size != 0 {
				t.Fatal("nonempty snapshot", pos, size)
			}
		}
		id, e := r.Object(3)
		if e != nil {
			return e
		}
		stats := r.statistics[id]
		if stats == nil || stats.owner != owner {
			return ErrZ3ReplayReference
		}
		if _, e = stats.value.Size(); e != nil {
			return e
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
		t.Fatal("fixedpoint snapshots not checked")
	}
}

func TestGeneratedFixedpointReplayClosesIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 511\n= 0x42\nR\nP 0x41\nP 0x42\nC 512\n"
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
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var f *replayFixedpoint
	for _, value := range r.fixedpoints {
		f = value
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if f == nil || !f.value.closed || !f.object.closed {
		t.Fatal("unreleased fixedpoint")
	}
}

func TestGeneratedFixedpointReplayFactsAndRelationQuery(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |R|\nU 0\np 0\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\nC 511\n= 0x44\nR\nP 0x41\nP 0x44\nC 512\nR\nP 0x41\nP 0x44\nC 901\nR\nP 0x41\nP 0x44\nP 0x43\nC 526\n" +
		"R\nP 0x41\nP 0x44\nP 0x43\nU 1\n$ |tr_hashtable|\ns 1\nC 527\n" +
		"R\nP 0x41\nP 0x44\nP 0x43\nU 0\nu 0\nC 515\nR\nP 0x41\nC 54\n= 0x45\nR\nP 0x41\nP 0x44\nP 0x45\nC 516\n" +
		"R\nP 0x41\nP 0x44\nU 1\nP 0x43\np 1\nC 518\n" +
		"R\nP 0x41\nP 0x44\nP 0x43\nC 522\nR\nP 0x41\nP 0x44\nI -1\nP 0x43\nP 0x45\nC 524\nR\nP 0x41\nP 0x44\nP 0x43\nP 0x45\nC 655\n" +
		"R\nP 0x41\nP 0x44\nI -1\nP 0x43\nC 523\n= 0x47\n" +
		"R\nP 0x41\nP 0x43\nU 0\np 0\nC 50\n= 0x46\nR\nP 0x41\nP 0x44\nU 1\nP 0x46\np 1\nC 533\n" +
		"R\nP 0x41\nP 0x44\nP 0x46\nC 517\n" +
		"R\nP 0x41\nP 0x44\nP 0x43\nP 0x46\nP 0x47\nC 900\nR\nP 0x41\nP 0x44\nC 513\n"
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
	if err = r.RegisterCommand(901, "set-pdr-engine", func(r *Z3Replayer) error {
		f, e := replayGetFixedpoint(r)
		if e != nil {
			return e
		}
		return f.owner.context.SetFixedpointEngine("pdr")
	}); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-fixedpoint-query", func(r *Z3Replayer) error {
		f, e := replayGetFixedpoint(r)
		if e != nil {
			return e
		}
		decl, e := r.astObject(f.owner, 2)
		if e != nil {
			return e
		}
		status, e := f.value.QueryRelations([]ast.ASTHandle{decl.handle})
		if e != nil {
			return e
		}
		if status != lbool.True {
			t.Fatal("query status", status)
		}
		query, e := r.astObject(f.owner, 3)
		if e != nil {
			return e
		}
		cover, e := r.astObject(f.owner, 4)
		if e != nil {
			return e
		}
		if !f.owner.results.manager.IsTrue(cover.handle) {
			t.Fatal("cover is not true")
		}
		printed, e := f.value.SMT2(query.handle)
		if e != nil {
			return e
		}
		if !strings.Contains(printed, "(query") {
			t.Fatal("missing query in fixedpoint display", printed)
		}
		status, e = f.value.QueryExpression(query.handle)
		if e != nil {
			return e
		}
		if status != lbool.True {
			t.Fatal("expression query status", status)
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
		t.Fatal("fixedpoint query not checked")
	}
}

func TestGeneratedFixedpointReplaySpacerReachable(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |R|\nU 0\np 0\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\nC 511\n= 0x44\nR\nP 0x41\nP 0x44\nC 512\nR\nP 0x41\nP 0x44\nC 901\n" +
		"R\nP 0x41\nP 0x44\nP 0x43\nC 526\nR\nP 0x41\nP 0x44\nP 0x43\nU 0\nu 0\nC 515\n" +
		"R\nP 0x41\nP 0x44\nU 1\nP 0x43\np 1\nC 518\nR\nP 0x41\nP 0x44\nP 0x43\nC 656\n= 0x45\n" +
		"R\nP 0x41\nP 0x44\nC 513\nR\nP 0x41\nP 0x45\nC 900\n"
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
	if err = r.RegisterCommand(901, "set-spacer-engine", func(r *Z3Replayer) error {
		f, e := replayGetFixedpoint(r)
		if e != nil {
			return e
		}
		return f.owner.context.SetFixedpointEngine("spacer")
	}); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-reachable-result", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		value, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		if !owner.results.manager.IsTrue(value.handle) {
			t.Fatal("reachable nullary fact should be true")
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
		t.Fatal("reachable result not checked")
	}
}

func TestGeneratedFixedpointReplayRejectsShortRepresentationArray(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |R|\nU 0\np 0\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\nC 511\n= 0x44\nR\nP 0x41\nP 0x44\nC 512\nR\nP 0x41\nP 0x44\nP 0x43\nC 526\n" +
		"R\nP 0x41\nP 0x44\nP 0x43\nU 2\n$ |tr_hashtable|\ns 1\nC 527\n"
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
		t.Fatal("short representation array", err)
	}
}

func TestGeneratedFixedpointReplayNamedRuleUpdate(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |R|\nU 0\np 0\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\nP 0x43\nU 0\np 0\nC 50\n= 0x44\nR\nP 0x41\nC 511\n= 0x45\nR\nP 0x41\nP 0x45\nC 512\n" +
		"R\nP 0x41\nP 0x45\nP 0x43\nC 526\nR\nP 0x41\nP 0x45\nP 0x44\n$ |rule|\nC 514\n" +
		"R\nP 0x41\nP 0x45\nP 0x44\n$ |rule|\nC 521\nR\nP 0x41\nP 0x45\nU 1\nP 0x43\np 1\nC 518\n" +
		"R\nP 0x41\nP 0x45\nC 528\n= 0x46\nR\nP 0x41\nP 0x46\nC 448\nR\nP 0x41\nP 0x45\nC 513\nR\nP 0x41\nP 0x46\nP 0x44\nC 900\nR\nP 0x41\nP 0x46\nC 449\n"
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
	if err = r.RegisterCommand(900, "check-rule-snapshot", func(r *Z3Replayer) error {
		v, e := replayGetVector(r)
		if e != nil {
			return e
		}
		n, e := v.value.Len()
		if e != nil {
			return e
		}
		if n != 1 {
			t.Fatal("updated rule count", n)
		}
		rule, e := v.value.At(0)
		if e != nil {
			return e
		}
		want, e := r.astObject(v.owner, 2)
		if e != nil {
			return e
		}
		if rule != want.handle {
			t.Fatal("updated rule changed")
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
		t.Fatal("rule snapshot not checked")
	}
}

func TestGeneratedFixedpointReplayRejectsUnsupportedRuleSymbol(t *testing.T) {
	for _, name := range []string{"# 7", "$ ||"} {
		log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\nR\nP 0x41\nC 511\n= 0x43\nR\nP 0x41\nP 0x43\nC 512\n" +
			"R\nP 0x41\nP 0x43\nP 0x42\n" + name + "\nC 514\n"
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
		err = r.Parse()
		if err == nil || !strings.Contains(err.Error(), "rule names are not supported") {
			t.Fatal("rule symbol", name, err)
		}
		if err = r.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGeneratedFixedpointReplayUnnamedRule(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |R|\nU 0\np 0\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\nP 0x43\nU 0\np 0\nC 50\n= 0x44\nR\nP 0x41\nC 511\n= 0x45\nR\nP 0x41\nP 0x45\nC 512\n" +
		"R\nP 0x41\nP 0x45\nP 0x43\nC 526\nR\nP 0x41\nP 0x45\nP 0x44\nN\nC 514\nR\nP 0x41\nP 0x45\nU 1\nP 0x43\np 1\nC 518\n"
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
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedFixedpointReplayParameterDescriptions(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 511\n= 0x42\nR\nP 0x41\nP 0x42\nC 512\n" +
		"R\nP 0x41\nP 0x42\nC 531\nR\nP 0x41\nP 0x42\nC 532\n= 0x43\nR\nP 0x41\nP 0x43\nC 22\n" +
		"R\nP 0x41\nP 0x42\nC 513\nR\nP 0x41\nP 0x43\nC 900\nR\nP 0x41\nP 0x43\nC 23\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedParameterCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-fixedpoint-descriptions", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		d, e := replayDescriptions(r, 1, owner)
		if e != nil {
			return e
		}
		n, e := d.value.Size()
		if e != nil {
			return e
		}
		if n == 0 {
			t.Fatal("empty fixedpoint parameter catalog")
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
		t.Fatal("descriptions not checked")
	}
}
