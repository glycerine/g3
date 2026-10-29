package main

// g3 executes SMT-LIB scripts through the native Go SMT backend.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/shell"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

func run(input io.Reader, output io.Writer) (result error) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, context.Close()) }()
	return smt2.ParseCommands(context, input, output)
}

func runArguments(args []string, input io.Reader, output io.Writer) (result error) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, context.Close()) }()
	globals, err := context.GlobalParameters()
	if err != nil {
		return err
	}
	command := shell.NewCommandLine(nil, nil, globals)
	if err = command.Parse(args); err != nil {
		return err
	}
	if command.Action == shell.CommandHelp {
		_, err = fmt.Fprintln(output, "g3 Go SMT-LIB/DIMACS runner\nUsage: g3 [-smt2|-dimacs] <file> | g3 -in\nGlobal parameter options: name=value; -p, -pd, -pm, -pp:name")
		return err
	}
	if command.Action == shell.CommandVersion {
		_, err = fmt.Fprintln(output, "g3 (Go SMT-LIB backend)")
		return err
	}
	if command.Action != shell.CommandContinue {
		return command.WriteTerminalAction(output)
	}
	filename, selected := command.Input.Selected()
	if selected && command.StandardInput {
		return fmt.Errorf("using standard input to read formula")
	}
	if !selected && !command.StandardInput {
		return fmt.Errorf("input file was not specified")
	}
	kind, err := command.ResolveInputKind()
	if err != nil {
		return err
	}
	if kind != shell.InputSMT2 && kind != shell.InputDIMACS {
		return fmt.Errorf("selected input frontend is not ported yet")
	}
	if selected {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, file.Close()) }()
		input = file
	}
	if err = context.RefreshGlobalParameters(); err != nil {
		return err
	}
	if kind == shell.InputDIMACS {
		frontend, err := shell.NewDIMACSFrontend(shell.DIMACSFrontendConfig{Globals: globals, Input: input, Writer: output, DisplayStatistics: command.DisplayStatistics})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, frontend.Close()) }()
		return frontend.Run()
	}
	total := stopwatch.New(nil)
	if command.DisplayStatistics {
		if err = total.Start(); err != nil {
			return err
		}
	}
	parseErr := smt2.ParseCommands(context, input, output)
	if command.DisplayStatistics {
		stopErr := total.Stop()
		seconds, secondsErr := total.Seconds()
		if stopErr == nil && secondsErr == nil {
			err = context.WriteStatisticsWithTotalTime(output, seconds)
		}
		err = errors.Join(stopErr, secondsErr, err)
	}
	return errors.Join(parseErr, err)
}

func main() {
	if err := runArguments(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stdout, "(error \"%s\")\n", strings.ReplaceAll(err.Error(), "\"", "\"\""))
		os.Exit(1)
	}
}
