package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp:153 transformed-rule diagnostic.

import (
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type pdrVerboseRuleWriter struct{ stream *utext.NumericTextStream }

func (w pdrVerboseRuleWriter) Write(body []byte) (int, error) {
	if err := w.stream.AppendCString(string(body)); err != nil {
		return 0, err
	}
	return len(body), nil
}

func writePDRQueryRules(rules *base.RuleSet) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || !enabled {
		return err
	}
	outputs, err := rules.OutputPredicates()
	if err != nil || len(outputs) == 0 {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return rules.WriteDisplay(pdrVerboseRuleWriter{stream: stream})
}
