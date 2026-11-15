package ddnf

// Source: src/muz/ddnf/ddnf.cpp ddnf_mgr::insert, well_formed and
// imp::query / compile_rule1 IF_VERBOSE sites. AUDIT.md FUZZ-F5: compiled
// only into g3trace builds (diagnostic.VerboseProgress).

import (
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// verbose is IF_VERBOSE(level, verbose_stream() << text()); text runs only
// when the level is enabled.
func verbose(level uint32, text func() (string, error)) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	if enabled, err := diagnostic.ProcessVerboseEnabled(level); err != nil || !enabled {
		return err
	}
	s, err := text()
	if err != nil {
		return err
	}
	return diagnostic.ProgressVerbose(level, s)
}

// Verbose is the exported form for the engine's (ddnf.preprocess) and
// (ddnf.compile) lines.
func Verbose(level uint32, text string) error {
	return verbose(level, func() (string, error) { return text, nil })
}

func (c *Core) tbvText(vector *rel.TBV) (string, error) { return c.vectors.String(vector) }
