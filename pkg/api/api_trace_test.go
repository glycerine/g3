package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/shell"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/params"
)

// Source: src/api/api_context.cpp:408-419 and shell/main.cpp:251. The public
// API and shell option borrow one process tag table, including C-string names.
func TestAPIGlobalTraceControlsShareShellProcessTags(t *testing.T) {
	tags := diagnostic.BorrowProcessTraceTags()
	const name = "g3-api-trace-binding"
	DisableTrace(name)
	t.Cleanup(func() {
		DisableTrace(name)
		if err := engine.FinalizeDefaultProcess(); err != nil {
			t.Fatal(err)
		}
	})
	if tags.IsEnabled(name) {
		t.Fatal("preexisting test trace tag")
	}
	if err := EnableTrace(name + "\x00ignored"); err != nil {
		t.Fatal(err)
	}
	if params.BorrowProcessGlobalParamsService() == nil {
		t.Fatal("trace enable did not initialize the generated process")
	}
	symbols := params.BorrowProcessGlobalParamsService().Context.Symbols()
	count := symbols.Stats().Symbols
	if _, err := symbols.Intern(name); err != nil {
		t.Fatal(err)
	}
	if symbols.Stats().Symbols != count {
		t.Fatal("trace tag was not interned before enable")
	}
	if !tags.IsEnabled(name) || !tags.IsEnabled(name+"\x00other") {
		t.Fatal("source C-string enable")
	}
	if shell.NewCommandLine(nil, nil).Trace != tags {
		t.Fatal("shell and API use different process trace tables")
	}
	DisableTrace(name + "\x00ignored")
	if tags.IsEnabled(name) {
		t.Fatal("source C-string disable")
	}
}
