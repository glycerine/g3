package api

// Source: api_datalog_spacer.inc joins symbol::str() spellings with semicolons;
// generated api_commands.cpp C654 executes that call without storing its result.

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestGeneratedFixedpointReplaySerializesTraceRuleSymbols(t *testing.T) {
	var log strings.Builder
	command := func(id int, args, result string) {
		fmt.Fprintf(&log, "R\n%sC %d\n", args, id)
		if result != "" {
			fmt.Fprintf(&log, "= %s\n", result)
		}
	}
	command(6, "P 0\n", "0x41")
	command(32, "P 0x41\n", "0x42")
	command(511, "P 0x41\n", "0x43")
	command(512, "P 0x41\nP 0x43\n", "")
	command(13, "P 0x41\n", "0x44")
	command(14, "P 0x41\nP 0x44\n", "")
	command(19, "P 0x41\nP 0x44\n$ |engine|\n$ |spacer|\n", "")
	for _, name := range []string{"xform.inline_eager", "xform.inline_linear", "xform.subsumption_checker", "xform.slice"} {
		command(16, "P 0x41\nP 0x44\n$ |"+name+"|\nI 0\n", "")
	}
	command(530, "P 0x41\nP 0x43\nP 0x44\n", "")
	command(15, "P 0x41\nP 0x44\n", "")
	var atoms, formulas []string
	for i, token := range []string{"N", "$ ||", "# 7", "$ |k!7|"} {
		decl, atom := fmt.Sprintf("0x%x", 0x50+i), fmt.Sprintf("0x%x", 0x60+i)
		command(49, fmt.Sprintf("P 0x41\n$ |trace-symbol-%d|\nU 0\np 0\nP 0x42\n", i), decl)
		command(526, "P 0x41\nP 0x43\nP "+decl+"\n", "")
		command(50, "P 0x41\nP "+decl+"\nU 0\np 0\n", atom)
		formula := atom
		if i != 0 {
			formula = fmt.Sprintf("0x%x", 0x70+i)
			command(61, "P 0x41\nP "+atoms[i-1]+"\nP "+atom+"\n", formula)
		}
		atoms = append(atoms, atom)
		formulas = append(formulas, formula)
		command(514, "P 0x41\nP 0x43\nP "+formula+"\n"+token+"\n", "")
	}
	command(517, "P 0x41\nP 0x43\nP "+atoms[3]+"\n", "")
	command(653, "P 0x41\nP 0x43\n", "0x80")
	command(448, "P 0x41\nP 0x80\n", "")
	command(654, "P 0x41\nP 0x43\n", "")
	args := "P 0x41\nP 0x43\nP 0x80\n"
	for i := len(formulas) - 1; i >= 0; i-- {
		args += "P " + formulas[i] + "\n"
	}
	command(900, args, "")
	command(449, "P 0x41\nP 0x80\n", "")
	command(513, "P 0x41\nP 0x43\n", "")

	r, err := NewZ3Replayer(strings.NewReader(log.String()), io.Discard)
	astTestMust(t, err)
	defer func() { astTestMust(t, r.Close()) }()
	for _, register := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedParameterCommands} {
		astTestMust(t, register(r))
	}
	checked := false
	astTestMust(t, r.RegisterCommand(900, "check-serialized-trace-symbols", func(r *Z3Replayer) error {
		f, err := replayGetFixedpoint(r)
		if err != nil {
			return err
		}
		manager := f.owner.results.manager
		// The semicolon aggregate is absent from every log token. C654 must
		// already have interned it, even though replay discards scalar results.
		before := manager.Symbols.Stats()
		if _, err = manager.Symbols.Intern("<null>;k!7;k!7;;<null>"); err != nil {
			return err
		}
		if after := manager.Symbols.Stats(); after != before {
			return fmt.Errorf("C654 did not intern the source trace-name aggregate: before %+v, after %+v", before, after)
		}
		trace, err := replayVectorAt(r, f.owner, 2)
		if err != nil {
			return err
		}
		n, err := trace.value.Len()
		if err != nil {
			return err
		}
		if n != 5 {
			return fmt.Errorf("C653 trace has %d formulas, want query rule and four chain rules", n)
		}
		for i := uint32(1); i < n; i++ {
			got, err := trace.value.At(i)
			if err != nil {
				return err
			}
			want, err := r.astObject(f.owner, i+2)
			if err != nil {
				return err
			}
			if got != want.handle {
				return fmt.Errorf("C653 trace formula %d does not follow source derivation order", i)
			}
		}
		checked = true
		return nil
	}))
	astTestMust(t, r.Parse())
	if !checked {
		t.Fatal("serialized trace not checked")
	}
}
