//go:build g3trace

package api

// replayerVerboseTicks: z3_replayer::parse's IF_VERBOSE(1) progress line, a
// new verbose port, is compiled in g3trace builds.
const replayerVerboseTicks = true
