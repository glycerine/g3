package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestCommandLineModuleOrderMatchesCpp: AUDIT.md util_params_01-F1. gparams
// display walks m_module_param_descrs, a dictionary over core_hashtable, in
// slot order, so -p, -pd and -pm list the [module] sections in z3's
// hash-table order, not registration order.
func TestCommandLineModuleOrderMatchesCpp(t *testing.T) {
	const want = "pi pp fixedpoint model sat opt lp nnf algebraic combined_solver rcf model_evaluator ackermannization nlsat parser rewriter interp smt sls"
	for _, flag := range []string{"-pm", "-p", "-pd"} {
		var out bytes.Buffer
		if err := runArguments([]string{flag}, strings.NewReader(""), &out); err != nil {
			t.Fatal(flag, err)
		}
		var modules []string
		for _, line := range strings.Split(out.String(), "\n") {
			name, ok := strings.CutPrefix(line, "[module] ")
			if !ok {
				continue
			}
			if i := strings.IndexAny(name, ", "); i >= 0 {
				name = name[:i]
			}
			modules = append(modules, name)
		}
		if got := strings.Join(modules, " "); got != want {
			t.Fatalf("%s module order\n got %s\nwant %s", flag, got, want)
		}
		if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
			cpp, err := exec.Command(reference, flag).Output()
			if err != nil {
				t.Fatal(flag, err)
			}
			if !bytes.Equal(out.Bytes(), cpp) {
				t.Fatalf("%s: Go/C++ output differs", flag)
			}
		}
	}
}
