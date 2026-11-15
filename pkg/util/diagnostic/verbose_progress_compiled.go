//go:build g3trace

package diagnostic

// VerboseProgress gates the IF_VERBOSE progress lines ported for AUDIT.md
// FUZZ-F5 ((smt.searching), the timeit lines, setup and model dumps, ...).
// Owner decision (2026-11-15): they are compiled only into g3trace builds,
// so the untagged build pays nothing for them. Call sites test this
// constant directly so the untagged compiler removes them.
const VerboseProgress = true
