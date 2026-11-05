package smt2

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/escaped"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// helpTactic emits tactic_cmds.cpp help_tactic's registry listing and
// combinator guide. The probe metadata is shared with the public probe API.
func (p *commandParser) helpTactic() (resultErr error) {
	if err := p.requireEnd(); err != nil {
		return err
	}
	var text strings.Builder
	text.WriteString("combinators:\n")
	text.WriteString("- (and-then <tactic>+) executes the given tactics sequencially.\n")
	text.WriteString("- (or-else <tactic>+) tries the given tactics in sequence until one of them succeeds (i.e., the first that doesn't fail).\n")
	text.WriteString("- (par-or <tactic>+) executes the given tactics in parallel until one of them succeeds (i.e., the first that doesn't fail).\n")
	text.WriteString("- (par-then <tactic1> <tactic2>) executes tactic1 and then tactic2 to every subgoal produced by tactic1. All subgoals are processed in parallel.\n")
	text.WriteString("- (try-for <tactic> <num>) executes the given tactic for at most <num> milliseconds, it fails if the execution takes more than <num> milliseconds.\n")
	text.WriteString("- (if <probe> <tactic> <tactic>) if <probe> evaluates to true, then execute the first tactic. Otherwise execute the second tactic.\n")
	text.WriteString("- (when <probe> <tactic>) shorthand for (if <probe> <tactic> skip).\n")
	text.WriteString("- (fail-if <probe>) fail if <probe> evaluates to true.\n")
	text.WriteString("- (using-params <tactic> <attribute>*) executes the given tactic using the given attributes, where <attribute> ::= <keyword> <value>. ! is a syntax sugar for using-params.\n")
	text.WriteString("builtin tactics:\n")

	m, err := p.context.Manager()
	if err != nil {
		return err
	}
	for _, name := range cmd_context.BuiltinTacticNames() {
		description, err := cmd_context.BuiltinTacticDescription(name)
		if err != nil {
			return fmt.Errorf("help-tactic %s: %w", name, err)
		}
		fmt.Fprintf(&text, "- %s %s\n", name, description)
		selected, err := p.context.NewBuiltinTacticWithOutput(name, p.out)
		if err != nil {
			return fmt.Errorf("help-tactic %s: %w", name, err)
		}
		if err = selected.IncRef(); err != nil {
			return errors.Join(err, selected.Cleanup())
		}
		func() {
			defer func() {
				if cause := selected.DecRef(); cause != nil {
					resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s release: %w", name, cause))
				}
			}()
			descriptors, cause := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
			if cause != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s: %w", name, cause))
				return
			}
			defer func() { resultErr = errors.Join(resultErr, descriptors.Close()) }()
			if cause = selected.CollectParamDescrs(descriptors); cause != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s: %w", name, cause))
				return
			}
			size, cause := descriptors.Size()
			if cause != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s: %w", name, cause))
				return
			}
			if size == 0 {
				return
			}
			options, cause := descriptors.Display(4, false, true)
			if cause != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s: %w", name, cause))
				return
			}
			text.WriteString(options)
		}()
		if resultErr != nil {
			return resultErr
		}
	}
	text.WriteString("builtin probes:\n")
	for _, probe := range tactic.BuiltinProbeDescriptors() {
		fmt.Fprintf(&text, "- %s %s\n", probe.Name, probe.Description)
	}
	contents := text.String()
	quoted, err := escaped.String(&contents, true, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(p.out, "\"%s\"\n", quoted)
	return err
}
