package automata

// Source: src/math/automata/automaton.h:600-635 and 454-467.

import (
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// automatonTraceTags replaces the process TRACE tags in tests.
var automatonTraceTags *diagnostic.TraceTags

func (a *Automaton) sinkifyDeadStates() {
	dead := make(map[uint32]bool)
	for state := uint32(0); state < uint32(len(a.out)); state++ {
		if !a.IsFinal(state) {
			dead[state] = true
		}
	}
	for changed := true; changed; {
		changed = false
		var remove []uint32
		for state := range dead {
			for _, move := range a.out[state] {
				if !dead[move.Destination] {
					remove = append(remove, state)
					break
				}
			}
		}
		for _, state := range remove {
			delete(dead, state)
			changed = true
		}
	}
	// TRACE("seq", tout << "remove: " << dead_states << "\n";) then, per
	// dead state in uint_set (ascending) order, CTRACE("seq",
	// !m_delta[s].empty(), "live state s") (AUDIT.md math_misc_01-F7).
	states := make([]uint32, 0, len(dead))
	for state := range dead {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool { return states[i] < states[j] })
	tags := automatonTraceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("seq", "sinkify_dead_states", "src/math/automata/automaton.h", 596, func() bool { return true }, func(out io.Writer) error {
		text := make([]string, len(states))
		for i, state := range states {
			text[i] = strconv.FormatUint(uint64(state), 10)
		}
		_, err := io.WriteString(out, "remove: {"+strings.Join(text, ", ")+"}\n")
		return err
	})
	for _, state := range states {
		if err == nil {
			moves := len(a.out[state])
			err = tags.EmitConditional("seq", "sinkify_dead_states", "src/math/automata/automaton.h", 598, func() bool { return moves != 0 }, func(out io.Writer) error {
				_, err := io.WriteString(out, "live state "+strconv.FormatUint(uint64(state), 10)+"\n")
				return err
			})
		}
		a.out[state] = nil
	}
}

func (a *Automaton) trimTrailingStates() {
	for len(a.out) != 0 {
		state := uint32(len(a.out) - 1)
		if state == a.initial || len(a.in[state]) != 0 {
			return
		}
		a.RemoveFinal(state)
		a.out = a.out[:state]
	}
}
