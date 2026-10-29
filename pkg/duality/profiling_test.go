package duality

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/interp"
)

func profilerClock(p *Profiler, times ...time.Duration) {
	i := 0
	p.origin = time.Unix(100, 0)
	p.now = func() time.Time {
		if i >= len(times) {
			panic("profile clock exhausted")
		}
		t := p.origin.Add(times[i])
		i++
		return t
	}
}

func TestProfilerNestedTotalsAndInterpolation(t *testing.T) {
	ip := interp.NewProfiler()
	p := NewProfiler(ip)
	profilerClock(p, time.Second, 2*time.Second, 5*time.Second, 8*time.Second, 13*time.Second)
	if err := p.TimerStart("outer"); err != nil {
		t.Fatal(err)
	}
	if err := p.TimerStart("inner"); err != nil {
		t.Fatal(err)
	}
	if err := p.TimerStop("inner"); err != nil {
		t.Fatal(err)
	}
	if err := p.TimerStop("outer"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := p.Print(&out); err != nil {
		t.Fatal(err)
	}
	want := "TOTAL" + strings.Repeat(".", 65) + "7\n" +
		"  outer" + strings.Repeat(".", 63) + "7\n" +
		"    inner" + strings.Repeat(".", 61) + "3\n" +
		"TOTALS:\ninner 3\nouter 7\n"
	if !strings.HasPrefix(out.String(), want) {
		t.Fatalf("profile prefix:\n%s\nwant:\n%s", out.String(), want)
	}
	if !strings.Contains(out.String()[len(want):], "TOTAL") {
		t.Fatal("missing appended interpolation profile", out.String())
	}
}

func TestProfilerImbalancedStopRecoversLikeSource(t *testing.T) {
	p := NewProfiler(nil)
	profilerClock(p, time.Second, 2*time.Second, 7*time.Second, 9*time.Second)
	if err := p.TimerStart("outer"); err != nil {
		t.Fatal(err)
	}
	if err := p.TimerStart("lost"); err != nil {
		t.Fatal(err)
	}
	if err := p.TimerStop("outer"); err != nil {
		t.Fatal(err)
	}
	if p.current != p.top {
		t.Fatal("did not recover to top")
	}
	if got := p.top.children["outer"].elapsed; got != 6*time.Second {
		t.Fatal("outer elapsed", got)
	}
	if got := p.top.children["outer"].children["lost"].elapsed; got != 0 {
		t.Fatal("intervening timer was charged", got)
	}
	if err := p.TimerStop("missing"); err != nil || p.current != p.top {
		t.Fatal("missing stop recovery", err)
	}
}

func TestProfilerSortedOutputShowTimeAndWriterFailure(t *testing.T) {
	p := NewProfiler(nil)
	profilerClock(p, 3*time.Second, 4*time.Second, 5*time.Second, 6*time.Second, 11*time.Second)
	for _, name := range []string{"zeta", "alpha"} {
		if err := p.TimerStart(name); err != nil {
			t.Fatal(err)
		}
		if err := p.TimerStop(name); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := p.Print(&out); err != nil {
		t.Fatal(err)
	}
	if strings.Index(out.String(), "  alpha") > strings.Index(out.String(), "  zeta") {
		t.Fatal("children not deterministic", out.String())
	}
	var shown bytes.Buffer
	if err := p.ShowTime(&shown); err != nil || shown.String() != "11\n" {
		t.Fatal("show time", shown.String(), err)
	}
	if err := p.Print(failingWriter{}); !errors.Is(err, errWrite) {
		t.Fatal("writer failure", err)
	}
	if err := (*Profiler)(nil).TimerStart("x"); err == nil {
		t.Fatal("nil profiler accepted")
	}
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }
