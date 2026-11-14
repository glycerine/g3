package smt2

// Source differential: src/cmd_context/basic_cmds.cpp get_info_cmd and
// set_info_cmd.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2GetInfoBuiltinsAndPersistentStatus(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(get-info :name)(get-info :authors)(get-info :version)
(get-info :error-behavior)(get-info :status)(get-info :assertion-stack-levels)
(set-info :status sat)(push 2)(get-info :status)(get-info :assertion-stack-levels)
(pop 2)(reset)(get-info :status)(get-info :assertion-stack-levels)`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	want := "(:name \"Z3\")\n" +
		"(:authors \"Leonardo de Moura, Nikolaj Bjorner and Christoph Wintersteiger\")\n" +
		"(:version \"4.7.1\")\n" +
		"(:error-behavior continued-execution)\n(:status unknown)\n(:assertion-stack-levels 0)\n" +
		"(:status sat)\n(:assertion-stack-levels 2)\n(:status sat)\n(:assertion-stack-levels 0)\n"
	if output.String() != want {
		t.Fatal("get-info output", output.String(), "want", want)
	}
}

func TestSMT2InfoStatusValidationAndUnsupportedLocation(t *testing.T) {
	for name, row := range map[string]struct {
		script, output, message string
	}{
		"sat-contradicts-unsat": {`(set-info :status unsat)(check-sat)`, "sat\n", "check annotation that says unsat"},
		"unsat-contradicts-sat": {`(set-info :status sat)(assert false)(check-sat)`, "unsat\n", "check annotation that says sat"},
		"invalid-status":        {`(set-info :status maybe)`, "", "invalid ':status' attribute"},
	} {
		t.Run(name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			})
			var output bytes.Buffer
			err = ParseCommands(context, strings.NewReader(row.script), &output)
			if err == nil || !strings.Contains(err.Error(), row.message) || output.String() != row.output {
				t.Fatal("status validation", err, output.String())
			}
		})
	}

	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	var output, diagnostic bytes.Buffer
	if err = context.SetDiagnosticOutput(&diagnostic); err != nil {
		t.Fatal(err)
	}
	if err = ParseCommands(context, strings.NewReader(`(get-info :not-there)`), &output); err != nil {
		t.Fatal(err)
	}
	// print_unsupported (AUDIT.md FUZZ-F6).
	if output.String() != "unsupported\n" || diagnostic.String() != "; :not-there line: 1 position: 1\n" {
		t.Fatal("unsupported get-info", output.String(), diagnostic.String())
	}
}
