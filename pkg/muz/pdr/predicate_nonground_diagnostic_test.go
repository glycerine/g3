package pdr

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPDRRejectsNonGroundTailBeforeSimplificationAndCanRetry(t *testing.T) {
	m, context, rules := classifierContextFixture(t)
	invalid, _ := classifierRuleSet(t, m, rules, "quantified-boolean")
	valid, _ := classifierRuleSet(t, m, rules, "fact")
	priorLevel, err := diagnostic.ProcessVerbosityLevel()
	closureMust(t, err)
	priorStream, err := diagnostic.BorrowProcessVerboseStream()
	closureMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	closureMust(t, err)
	defer func() {
		closureMust(t, diagnostic.SetProcessVerbosityLevel(priorLevel))
		closureMust(t, diagnostic.SetProcessVerboseStream(priorStream))
		closureMust(t, stream.Close())
	}()
	closureMust(t, diagnostic.SetProcessVerboseStream(stream))
	closureMust(t, diagnostic.SetProcessVerbosityLevel(0))
	originalMode, err := m.ProofMode()
	closureMust(t, err)
	err = context.Initialize(invalid)
	if err == nil || !strings.Contains(err.Error(), "PDR cannot solve non-ground tails: (= (forall") {
		t.Fatal("source non-ground exception missing offending tail", err)
	}
	output, err := stream.String()
	closureMust(t, err)
	if !strings.HasPrefix(output, "PDR cannot solve non-ground tails: (= (forall") || !strings.HasSuffix(output, ")\n") {
		t.Fatal("source verbose diagnostic missing offending tail", output)
	}
	initialized, err := context.Initialized()
	closureMust(t, err)
	if initialized {
		t.Fatal("failed PDR initialization was published")
	}
	mode, err := m.ProofMode()
	closureMust(t, err)
	if mode != originalMode {
		t.Fatal("failed PDR initialization changed proof mode")
	}
	closureMust(t, context.Initialize(valid))
}

type nonGroundRefusingWriter struct{}

func (nonGroundRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestPDRNonGroundTailVerboseRefusalPreservesException(t *testing.T) {
	m, context, rules := classifierContextFixture(t)
	invalid, _ := classifierRuleSet(t, m, rules, "quantified-boolean")
	priorLevel, err := diagnostic.ProcessVerbosityLevel()
	closureMust(t, err)
	priorStream, err := diagnostic.BorrowProcessVerboseStream()
	closureMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget(), BorrowedWriter: nonGroundRefusingWriter{}})
	closureMust(t, err)
	defer func() {
		closureMust(t, diagnostic.SetProcessVerbosityLevel(priorLevel))
		closureMust(t, diagnostic.SetProcessVerboseStream(priorStream))
		closureMust(t, stream.Close())
	}()
	closureMust(t, diagnostic.SetProcessVerboseStream(stream))
	closureMust(t, diagnostic.SetProcessVerbosityLevel(0))
	err = context.Initialize(invalid)
	if err == nil || !strings.Contains(err.Error(), "PDR cannot solve non-ground tails: (= (forall") {
		t.Fatal("stream refusal replaced source exception", err)
	}
}
