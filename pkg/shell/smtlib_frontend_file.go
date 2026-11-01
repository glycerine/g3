package shell

// Source: src/shell/smtlib_frontend.cpp:39-44 file frontend selection.

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/cmd_context"
)

type SMTLIBFileFrontendConfig struct {
	Context           *cmd_context.Context
	Output            io.Writer
	Diagnostics       io.Writer
	DisplayStatistics bool
	Clock             func() (uint64, error)
}

// OpenSMTLIBFrontend opens name and transfers the file lifetime to the
// returned frontend. Failed construction closes the file before returning.
func OpenSMTLIBFrontend(name string, c SMTLIBFileFrontendConfig) (f *SMTLIBFrontend, result error) {
	if name == "" || c.Output == nil {
		return nil, ErrShellOptionService
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("failed to open file '%s': %w", name, err)
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, file.Close())
		}
	}()
	f, err = NewSMTLIBFrontend(SMTLIBFrontendConfig{
		Context:           c.Context,
		Input:             file,
		Output:            c.Output,
		Diagnostics:       c.Diagnostics,
		DisplayStatistics: c.DisplayStatistics,
		Clock:             c.Clock,
	})
	if err != nil {
		return nil, err
	}
	f.inputClose = file
	return f, nil
}
