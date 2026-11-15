package base_test

// Source: src/muz/base/dl_rule_transformer.cpp rule_transformer::operator()
// IF_VERBOSE(1) "(transform <typeid>..." lines and the destratified-negation
// warning (AUDIT.md muz_base_02-F2).

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type namedReplacementTransformPlugin struct {
	replacementTransformPlugin
}

func (p *namedReplacementTransformPlugin) SourceTypeName() string {
	return base.DatalogPluginTypeName("mk_magic_sets")
}

type namedNoOpTransformPlugin struct {
	recordingTransformPlugin
}

func (p *namedNoOpTransformPlugin) SourceTypeName() string {
	return base.DatalogPluginTypeName("mk_coi_filter")
}

func captureRuleTransformDiagnostics(t *testing.T) (*utext.NumericTextStream, *bytes.Buffer) {
	t.Helper()
	level, err := diagnostic.ProcessVerbosityLevel()
	ruleMust(t, err)
	previous, err := diagnostic.BorrowProcessVerboseStream()
	ruleMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	ruleMust(t, err)
	warnings := diagnostic.BorrowProcessWarningService()
	previousWarnings := warnings.WarningStream()
	var buf bytes.Buffer
	ruleMust(t, warnings.SetWarningStream(&buf))
	ruleMust(t, diagnostic.SetProcessVerboseStream(stream))
	ruleMust(t, diagnostic.SetProcessVerbosityLevel(1))
	t.Cleanup(func() {
		ruleMust(t, diagnostic.SetProcessVerbosityLevel(level))
		ruleMust(t, diagnostic.SetProcessVerboseStream(previous))
		ruleMust(t, warnings.SetWarningStream(previousWarnings))
		ruleMust(t, stream.Close())
	})
	return stream, &buf
}

func TestRuleTransformerDestratifyWarning(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	fact := f.rule("P", nil, nil)
	ruleMust(t, set.Add(fact))
	ruleMust(t, fact.Close())
	valid, err := set.Finalize()
	ruleMust(t, err)
	if !valid {
		t.Fatal("transform source rejected")
	}
	bad, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	for _, rule := range []*base.Rule{
		f.rule("P", []string{"Q"}, []bool{true}),
		f.rule("Q", []string{"P"}, nil),
	} {
		ruleMust(t, bad.Add(rule))
		ruleMust(t, rule.Close())
	}
	var order []string
	noop := &namedNoOpTransformPlugin{recordingTransformPlugin{priority: 0, id: "coi", order: &order}}
	plugin := &namedReplacementTransformPlugin{replacementTransformPlugin{next: bad, destratify: true}}
	transformer := base.NewRuleTransformer()
	ruleMust(t, transformer.Register(noop))
	ruleMust(t, transformer.Register(plugin))
	t.Cleanup(func() { ruleMust(t, transformer.Close()) })
	verbose, warnings := captureRuleTransformDiagnostics(t)
	modified, err := transformer.Apply(set)
	ruleMust(t, err)
	if modified {
		t.Fatal("destratifying result replaced the source")
	}
	if got, want := warnings.String(), "WARNING: a rule transformation skipped because it destratified negation\n"; got != want {
		t.Fatalf("warning %q, want %q", got, want)
	}
	text, err := verbose.String()
	ruleMust(t, err)
	want := "(transform N7datalog13mk_magic_setsE...no-op 0s)\n(transform N7datalog13mk_coi_filterE...no-op 0s)\n"
	if text != want {
		t.Fatalf("verbose %q, want %q", text, want)
	}
}
