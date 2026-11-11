package text

import (
	"bufio"
	"os"

	"github.com/glycerine/g3/pkg/arena"
)

// InitEmbeddedFile initializes an embedded formatter without a separate header
// allocation. The enclosing file owner retains responsibility for its fd.
func (s *NumericTextStream) InitEmbeddedFile(b *arena.Budget) {
	*s = NumericTextStream{Budget: b, fileMode: true, state: NumericStreamState{Precision: 6, IntegerBase: 10, Fill: ' '}}
	s.self = s
}
func (s *NumericTextStream) BindFile(fd *os.File, failed bool) { s.File = fd; s.fileFail = failed }

// BindBufferedFile is BindFile with a filebuf-like buffer in front of fd, so
// each formatted byte is not its own write. CloseBorrowedFile flushes it.
func (s *NumericTextStream) BindBufferedFile(fd *os.File) {
	s.File, s.fileFail = fd, false
	s.writer = bufio.NewWriterSize(fd, 1<<16)
}

func (s *NumericTextStream) CloseBorrowedFile() {
	if w, ok := s.writer.(*bufio.Writer); ok && s.File != nil {
		if e := w.Flush(); e != nil {
			s.FileBad = true
		}
		s.writer = nil
	}
	if s.File != nil {
		if e := s.File.Close(); e != nil {
			s.fileFail = true
		}
		s.File = nil
	}
}

// CheckOpen validates the actual stream while holding its owner lock.
func (s *NumericTextStream) CheckOpen() error {
	if e := s.lock(); e != nil {
		return e
	}
	s.mu.Unlock()
	return nil
}

// CheckIdentity validates the initialized owner without requiring an open file.
// Borrowed input ties may retain a closed output and fail only when flushed.
func (s *NumericTextStream) CheckIdentity() error {
	if s == nil || s.self != s {
		return ErrNumericStreamOwner
	}
	return nil
}
