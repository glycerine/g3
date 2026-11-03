package api

// Source: api_datalog.cpp add/update pass typed symbols to dl_context.cpp;
// update_rule searches normalized rules without consuming queued AddRule inputs.

import (
	"fmt"
	"io"
	"strings"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestGeneratedFixedpointReplayPreservesRuleSymbolKinds(t *testing.T) {
	for _, pending := range []bool{false, true} {
		phase := "update-after-flush"
		if pending {
			phase = "update-before-flush"
		}
		t.Run(phase, func(t *testing.T) {
			tokens := []string{"N", "$ ||", "# 7", "$ |k!7|"}
			type checkpoint struct {
				formulas []string
				names    []int
			}
			var checkpoints []checkpoint
			var log strings.Builder
			command := func(id int, args, result string) {
				fmt.Fprintf(&log, "R\n%sC %d\n", args, id)
				if result != "" {
					fmt.Fprintf(&log, "= %s\n", result)
				}
			}
			check := func(formulas []string, names []int) {
				index := len(checkpoints)
				checkpoints = append(checkpoints, checkpoint{append([]string(nil), formulas...), append([]int(nil), names...)})
				args := fmt.Sprintf("P 0x41\nP 0x43\nU %d\n", index)
				for _, formula := range formulas {
					args += "P " + formula + "\n"
				}
				command(900, args, "")
			}
			command(6, "P 0\n", "0x41")
			command(32, "P 0x41\n", "0x42")
			command(511, "P 0x41\n", "0x43")
			command(512, "P 0x41\nP 0x43\n", "")
			var declarations, facts, replacements []string
			for i := 0; i < 5; i++ {
				declaration, atom := fmt.Sprintf("0x%x", 0x50+i), fmt.Sprintf("0x%x", 0x60+i)
				command(49, fmt.Sprintf("P 0x41\n$ |rule-symbol-P%d|\nU 0\np 0\nP 0x42\n", i), declaration)
				command(50, "P 0x41\nP "+declaration+"\nU 0\np 0\n", atom)
				command(526, "P 0x41\nP 0x43\nP "+declaration+"\n", "")
				declarations = append(declarations, declaration)
				facts = append(facts, atom)
			}
			for i, token := range tokens {
				formula := fmt.Sprintf("0x%x", 0x70+i)
				command(61, "P 0x41\nP "+facts[4]+"\nP "+facts[i]+"\n", formula)
				replacements = append(replacements, formula)
				command(514, "P 0x41\nP 0x43\nP "+facts[i]+"\n"+token+"\n", "")
			}
			query := func() {
				args := "P 0x41\nP 0x43\nU 4\n"
				for _, declaration := range declarations[:4] {
					args += "P " + declaration + "\n"
				}
				command(518, args+"p 4\n", "")
			}
			names := []int{0, 1, 2, 3}
			current := append([]string(nil), facts[:4]...)
			check(current, names)
			if !pending {
				query()
				check(current, names)
			}
			for i, token := range tokens {
				command(521, "P 0x41\nP 0x43\nP "+replacements[i]+"\n"+token+"\n", "")
				if pending {
					forms := append(append([]string(nil), replacements[:i+1]...), facts[:4]...)
					labels := append(append([]int(nil), names[:i+1]...), names...)
					check(forms, labels)
				} else {
					current[i] = replacements[i]
					check(current, names)
				}
			}
			if pending {
				query()
				check(append(append([]string(nil), replacements...), facts[:4]...), append(append([]int(nil), names...), names...))
			}
			command(513, "P 0x41\nP 0x43\n", "")
			r, err := NewZ3Replayer(strings.NewReader(log.String()), io.Discard)
			astTestMust(t, err)
			defer func() { astTestMust(t, r.Close()) }()
			astTestMust(t, RegisterZ3GeneratedContextCommands(r))
			astTestMust(t, RegisterZ3GeneratedASTCommands(r))
			checked := 0
			astTestMust(t, r.RegisterCommand(900, "check-typed-rule-symbols", func(r *Z3Replayer) error {
				f, err := replayGetFixedpoint(r)
				if err != nil {
					return err
				}
				index, err := r.Uint(2)
				if err != nil {
					return err
				}
				if int(index) != checked || int(index) >= len(checkpoints) {
					return fmt.Errorf("unexpected rule checkpoint %d", index)
				}
				want := checkpoints[index]
				manager := f.owner.results.manager
				empty, err := manager.Symbols.Intern("")
				if err != nil {
					return err
				}
				spelling, err := manager.Symbols.Intern("k!7")
				if err != nil {
					return err
				}
				expectedNames := []usymbol.Symbol{{}, empty, usymbol.Numerical(7), spelling}
				forms, labels, err := f.value.command.FixedpointNamedRulesWithSymbols()
				if err != nil {
					return err
				}
				defer func() {
					for i := range forms {
						astTestMust(t, manager.DelRef(&forms[i]))
					}
				}()
				if len(forms) != len(want.formulas) || len(labels) != len(forms) {
					return fmt.Errorf("checkpoint %d rule count %d names %d, want %d", index, len(forms), len(labels), len(want.formulas))
				}
				for i, reference := range forms {
					expected, err := r.astObject(f.owner, uint32(i+3))
					if err != nil {
						return err
					}
					actual, err := manager.RefGet(reference)
					if err != nil {
						return err
					}
					if actual != expected.handle || labels[i] != expectedNames[want.names[i]] {
						return fmt.Errorf("checkpoint %d rule %d changed formula/slot/name: formula %v want %v, name %#v want %#v", index, i, actual, expected.handle, labels[i], expectedNames[want.names[i]])
					}
				}
				checked++
				return nil
			}))
			astTestMust(t, r.Parse())
			if checked != len(checkpoints) {
				t.Fatal("rule symbol checkpoints missed", checked, len(checkpoints))
			}
		})
	}
}
