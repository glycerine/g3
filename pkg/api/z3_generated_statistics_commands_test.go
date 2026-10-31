package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedStatisticsReplaySurvivesSolverRelease(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 408\n= 0x42\nR\nP 0x41\nP 0x42\nC 416\n" +
		"R\nP 0x41\nP 0x42\nC 427\nR\nP 0x41\nP 0x42\nC 435\n= 0x43\nR\nP 0x41\nP 0x43\nC 438\nR\nP 0x41\nP 0x42\nC 417\n" +
		"R\nP 0x41\nP 0x43\nC 437\nR\nP 0x41\nP 0x43\nC 440\n" +
		"R\nP 0x41\nC 900\n= 0x44\nR\nP 0x41\nP 0x44\nC 438\n" +
		"R\nP 0x41\nP 0x44\nU 0\nC 441\nR\nP 0x41\nP 0x44\nU 0\nC 442\nR\nP 0x41\nP 0x44\nU 1\nC 443\n" +
		"R\nP 0x41\nP 0x44\nU 0\nC 444\nR\nP 0x41\nP 0x44\nU 1\nC 445\nR\nP 0x41\nP 0x44\nC 901\n" +
		"R\nP 0x41\nP 0x43\nC 439\nR\nP 0x41\nP 0x44\nC 439\n"
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
	if err = r.RegisterCommand(900, "create-typed-statistics", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		value, e := newAPIStatistics(owner.context.smtContextState, apiStatisticsFixture())
		if e != nil {
			return e
		}
		return replayStoreStatistics(r, owner, value)
	}); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(901, "check-statistics-values", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		id, e := r.Object(1)
		if e != nil {
			return e
		}
		stats := r.statistics[id]
		if stats == nil || stats.owner != owner {
			return ErrZ3ReplayReference
		}
		a, e := stats.value.UintValue(0)
		if e != nil {
			return e
		}
		b, e := stats.value.DoubleValue(1)
		if e != nil {
			return e
		}
		if a != 7 || b != 2.5 {
			t.Fatal("statistics values", a, b)
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
		t.Fatal("statistics not checked")
	}
}
