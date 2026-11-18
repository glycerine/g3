//go:build !g3trace

package sat

// satSyncVerbose is exchange_par's "(sat-sync ...)" verbose line, printed
// only in g3trace builds.
func satSyncVerbose(numOut, numIn uint32) error { return nil }
