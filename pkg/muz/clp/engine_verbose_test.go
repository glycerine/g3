package clp_test

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestCLPSearchVerboseLevels(t *testing.T) {
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	if err != nil {
		t.Fatal(err)
	}
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := diagnostic.SetProcessVerbosityLevel(oldLevel); err != nil {
			t.Error(err)
		}
		if err := diagnostic.SetProcessVerboseStream(oldStream); err != nil {
			t.Error(err)
		}
	})
	for _, tc := range []struct {
		level uint32
		want  string
	}{
		{0, ""},
		{1, "search 20 0\nsearch 19 1\n"},
		{2, "search 20 0\n0 clp-Q\nsearch 19 1\n1 clp-P\n"},
	} {
		stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
		if err != nil {
			t.Fatal(err)
		}
		if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
			t.Fatal(err)
		}
		if err = diagnostic.SetProcessVerbosityLevel(tc.level); err != nil {
			t.Fatal(err)
		}
		f := newFixture(t)
		engine, err := clp.NewEngine(f.m, f.set, f.solver, 20)
		if err != nil {
			t.Fatal(err)
		}
		status, err := engine.Query(f.app["Q"])
		if err != nil || status != lbool.True {
			t.Fatalf("level %d: %v, %v", tc.level, status, err)
		}
		if err = engine.Close(); err != nil {
			t.Fatal(err)
		}
		got, err := stream.String()
		if err != nil || got != tc.want {
			t.Fatalf("level %d: got %q, want %q, error %v", tc.level, got, tc.want, err)
		}
		if err = diagnostic.SetProcessVerboseStream(oldStream); err != nil {
			t.Fatal(err)
		}
		if err = stream.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCLPVerboseRuleUsesPrettyPrinterParameters(t *testing.T) {
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	if err != nil {
		t.Fatal(err)
	}
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := diagnostic.SetProcessVerbosityLevel(oldLevel); err != nil {
			t.Error(err)
		}
		if err := diagnostic.SetProcessVerboseStream(oldStream); err != nil {
			t.Error(err)
		}
		if err := stream.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
		t.Fatal(err)
	}
	if err = diagnostic.SetProcessVerbosityLevel(2); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	_, globals, err := f.m.RewriterParameterServices()
	if err != nil || globals == nil {
		t.Fatal("parameter services", err)
	}
	if err = globals.SourceSetName("pp.max_width", "12"); err != nil {
		t.Fatal(err)
	}
	boolean, err := f.m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	name, err := f.m.Symbols.Intern("long-clp-predicate")
	if err != nil {
		t.Fatal(err)
	}
	decl, err := f.m.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.rm.RegisterPredicate(decl); err != nil {
		t.Fatal(err)
	}
	truth, err := f.m.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	head, err := f.m.MkApp(decl, []ast.ASTHandle{truth, truth})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := f.rm.NewRule(head, nil, nil, "fact")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := base.NewRuleSet(f.rm)
	if err != nil {
		t.Fatal(err)
	}
	defer rules.Close()
	if err = rules.Add(rule); err != nil {
		t.Fatal(err)
	}
	if err = rule.Close(); err != nil {
		t.Fatal(err)
	}
	if ok, err := rules.Finalize(); err != nil || !ok {
		t.Fatal("finalize", ok, err)
	}
	engine, err := clp.NewEngine(f.m, rules, f.solver, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	status, err := engine.Query(head)
	if err != nil || status != lbool.True {
		t.Fatal("query", status, err)
	}
	got, err := stream.String()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "0 (long-clp-predicate\n") {
		t.Fatalf("max_width was ignored: %q", got)
	}
}
