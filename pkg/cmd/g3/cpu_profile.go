package main

// Developer diagnostic, not part of the z3 port: with G3_CPUPROFILE=FILE set,
// g3 writes a Go CPU profile of the whole run to FILE (inspect it with
// `go tool pprof`). Unset by default, so normal runs are unaffected.

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"
	"syscall"
)

// startCPUProfile starts profiling when G3_CPUPROFILE is set and returns the
// function that stops it and closes the file. While profiling, SIGINT and
// SIGTERM also stop the profile before exiting.
func startCPUProfile() func() {
	path := os.Getenv("G3_CPUPROFILE")
	if path == "" {
		return func() {}
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "G3_CPUPROFILE:", err)
		return func() {}
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Fprintln(os.Stderr, "G3_CPUPROFILE:", err)
		_ = f.Close()
		return func() {}
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			pprof.StopCPUProfile()
			_ = f.Close()
		})
	}
	// A run cut short by timeout(1) or ^C still writes its profile.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		stop()
		os.Exit(130)
	}()
	return stop
}
