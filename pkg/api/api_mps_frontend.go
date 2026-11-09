package api

// Source: src/shell/lp_frontend.cpp and lp_frontend.h.

import (
	"bytes"
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/shell"
)

type apiMPSInvocation struct {
	command  *cmd_context.Context
	frontend *shell.LPFrontend
	output   bytes.Buffer
}

func newAPIMPSInvocation(input *apiMPSInput, options *MPSRunOptions) (*apiMPSInvocation, error) {
	command, err := cmd_context.NewAuxiliaryContext()
	if err != nil {
		return nil, err
	}
	result := &apiMPSInvocation{command: command}
	globals, err := command.GlobalParameters()
	if err != nil {
		return nil, errors.Join(err, command.Close())
	}
	if options != nil {
		values := map[string]string{
			"lp.min":         strconv.FormatBool(options.Minimum),
			"lp.print_stats": strconv.FormatBool(options.PrintStatistics),
			"lp.rep_freq":    strconv.FormatUint(uint64(options.ReportFrequency), 10),
		}
		// Fixed order (the set calls are independent; Go map order is random).
		for _, name := range []string{"lp.min", "lp.print_stats", "lp.rep_freq"} {
			if err = globals.SourceSetName(name, values[name]); err != nil {
				return nil, errors.Join(err, command.Close())
			}
		}
	}
	result.frontend, err = shell.NewLPFrontend(shell.LPFrontendConfig{
		Globals: globals, Input: input.reader, Output: &result.output, Name: input.name,
	})
	if err != nil {
		return nil, errors.Join(err, command.Close())
	}
	return result, nil
}

func (i *apiMPSInvocation) close() error {
	if i == nil {
		return nil
	}
	var err error
	if i.frontend != nil {
		err = i.frontend.Close()
		i.frontend = nil
	}
	if i.command != nil {
		err = errors.Join(err, i.command.Close())
		i.command = nil
	}
	return err
}
