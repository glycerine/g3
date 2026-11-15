package base

// Source: src/muz/base/dl_rule_transformer.cpp rule_transformer::operator()
// IF_VERBOSE(1) lines and the destratified-negation warning.

import (
	"fmt"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// RuleTransformPluginTypeNamer is implemented by plugins that report the
// GCC typeid(p).name() of their source class (see DatalogPluginTypeName).
type RuleTransformPluginTypeNamer interface {
	SourceTypeName() string
}

// DatalogPluginTypeName is the Itanium-mangled typeid name of a class in
// namespace datalog, for example N7datalog13mk_coi_filterE.
func DatalogPluginTypeName(class string) string {
	return "N7datalog" + strconv.Itoa(len(class)) + class + "E"
}

func ruleTransformPluginTypeName(plugin RuleTransformPlugin) string {
	if named, ok := plugin.(RuleTransformPluginTypeNamer); ok {
		return named.SourceTypeName()
	}
	return fmt.Sprintf("%T", plugin)
}

func ruleTransformVerbose(write func(*verboseWriter) error) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return write(&verboseWriter{out: out})
}

type verboseWriter struct {
	out interface {
		AppendCString(string) error
		AppendFloat64(float64) error
		AppendUint64(uint64) error
	}
}

// ruleTransformSeconds is stopwatch::get_seconds with the source's
// "if (sec < 0.001) sec = 0.0".
func ruleTransformSeconds(seconds float64) float64 {
	if seconds < 0.001 {
		return 0.0
	}
	return seconds
}

func ruleTransformVerboseStart(plugin RuleTransformPlugin) error {
	return ruleTransformVerbose(func(w *verboseWriter) error {
		if err := w.out.AppendCString("(transform "); err != nil {
			return err
		}
		if err := w.out.AppendCString(ruleTransformPluginTypeName(plugin)); err != nil {
			return err
		}
		return w.out.AppendCString("...")
	})
}

func ruleTransformVerboseNoOp(seconds float64) error {
	return ruleTransformVerbose(func(w *verboseWriter) error {
		if err := w.out.AppendCString("no-op "); err != nil {
			return err
		}
		if err := w.out.AppendFloat64(seconds); err != nil {
			return err
		}
		return w.out.AppendCString("s)\n")
	})
}

func ruleTransformVerboseRules(rules uint64, seconds float64) error {
	return ruleTransformVerbose(func(w *verboseWriter) error {
		if err := w.out.AppendUint64(rules); err != nil {
			return err
		}
		if err := w.out.AppendCString(" rules "); err != nil {
			return err
		}
		if err := w.out.AppendFloat64(seconds); err != nil {
			return err
		}
		return w.out.AppendCString("s)\n")
	})
}
