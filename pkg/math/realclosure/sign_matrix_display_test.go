package realclosure

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type signMatrixFailWriter struct {
	bytes.Buffer
	at, calls int
	err       error
	short     bool
}

func (w *signMatrixFailWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls != w.at {
		return w.Buffer.Write(p)
	}
	if w.short {
		return w.Buffer.Write(p[:len(p)-1])
	}
	return 0, w.err
}

func (w *signMatrixFailWriter) WriteString(text string) (int, error) {
	return w.Write([]byte(text))
}

func TestSignMatrixSourceDisplayAndWriterPrefixes(t *testing.T) {
	var matrix signMatrix
	matrix.make(2, 3)
	matrix.setSmall(1, -20, 30000, 0, -4, 5)
	want := "2 x 3 Matrix\n   1  -20 30000\n   0   -4    5\n"
	var out bytes.Buffer
	if err := matrix.write(&out); err != nil || out.String() != want {
		t.Fatalf("default-width display = %q, %v", out.String(), err)
	}
	out.Reset()
	if err := matrix.display(&out, 0); err != nil || out.String() != "2 x 3 Matrix\n1 -20 30000\n0 -4 5\n" {
		t.Fatalf("zero-width display = %q, %v", out.String(), err)
	}

	actions := []string{"2", " x ", "3", " Matrix\n"}
	for _, row := range [][]string{{"1", "-20", "30000"}, {"0", "-4", "5"}} {
		for col, cell := range row {
			if col > 0 {
				actions = append(actions, " ")
			}
			for pad := len(cell); pad < 4; pad++ {
				actions = append(actions, " ")
			}
			actions = append(actions, cell)
		}
		actions = append(actions, "\n")
	}
	prefix := ""
	refused := errors.New("matrix writer refused")
	for at := 1; at <= len(actions); at++ {
		w := &signMatrixFailWriter{at: at, err: refused}
		if err := matrix.write(w); !errors.Is(err, refused) || w.String() != prefix || w.calls != at {
			t.Fatalf("write prefix %d = %q, calls=%d, err=%v; want %q", at, w.String(), w.calls, err, prefix)
		}
		prefix += actions[at-1]
	}
	if prefix != want {
		t.Fatal("independent insertion sequence differs from source display", prefix)
	}
	for _, at := range []int{1, 2, 4, 8, 12, len(actions)} {
		w := &signMatrixFailWriter{at: at, short: true}
		err := matrix.write(w)
		if !errors.Is(err, io.ErrShortWrite) || w.calls != at || !strings.HasPrefix(want, w.String()) {
			t.Fatalf("short prefix %d = %q, calls=%d, err=%v", at, w.String(), w.calls, err)
		}
	}
}
