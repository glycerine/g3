//go:build !g3trace

package diagnostic

// VerboseProgress: see verbose_progress_compiled.go. Untagged builds omit
// the AUDIT.md FUZZ-F5 progress lines.
const VerboseProgress = false
