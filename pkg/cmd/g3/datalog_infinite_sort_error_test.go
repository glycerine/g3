package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

// gaps_misc2: a rule over an int domain fails rule_properties'
// check_infinite_sorts with a default_exception that read_datalog does not
// catch; main prints "ERROR: " << ex.msg() (the rule displayed) and exits
// with ERR_INTERNAL_FATAL. The fact P("5") is a relation tuple
// (rule_manager::is_fact), so the reported rule is Q's.
func TestDatalogInfiniteSortErrorText(t *testing.T) {
	path := filepath.Join("test_vectors", "datalog_int_infinite_sort.dl")
	var out bytes.Buffer
	err := runArguments([]string{path}, strings.NewReader(""), &out)
	want := "Rule contains infinite sorts in rule <null>:\nQ(#0) :- \n P(#0),\n (= (:var 0) 5).\n"
	if err == nil || z3exception.SourceMessage(err) != want || out.Len() != 0 {
		t.Fatalf("err %q output %q", err, out.String())
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cmd := exec.Command(reference, path)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		_ = cmd.Run()
		if got := stdout.String() + stderr.String(); got != "ERROR: "+want+"\n" {
			t.Fatalf("C++ output %q", got)
		}
	}
}
