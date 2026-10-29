package shell

// Source: src/shell/main.cpp:49-53 and invalid-option branch.

import "io"

func WriteCommandError(out io.Writer, message string) error {
	if out == nil {
		return ErrShellOptionService
	}
	return writeShellText(out, "Error: "+message+"\nFor usage information: z3 -h\n")
}
