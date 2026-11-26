package shell

// Source: src/shell/dimacs_frontend.cpp:49-59,126-137.
import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *DIMACSFrontend) writeResult(s *sat.Solver, status lbool.LBool, tracking [][]sat.Literal) error {
	switch status {
	case lbool.True:
		if _, err := io.WriteString(f.output, "sat\n"); err != nil {
			return err
		}
		model, err := s.AppendModel(nil)
		if err != nil {
			return err
		}
		return writeDIMACSModel(f.output, model)
	case lbool.False:
		if _, err := io.WriteString(f.output, "unsat\n"); err != nil {
			return err
		}
		if tracking != nil {
			return f.writeCore(s, tracking)
		}
		return nil
	default:
		_, err := io.WriteString(f.output, "unknown\n")
		return err
	}
}

func writeDIMACSModel(out io.Writer, model sat.Model) error {
	var buffer [4096]byte
	chunk := buffer[:0]
	flush := func() error {
		n, err := out.Write(chunk)
		if err != nil {
			return err
		}
		if n != len(chunk) {
			return io.ErrShortWrite
		}
		chunk = buffer[:0]
		return nil
	}
	// Room for a signed decimal integer, its space and the final newline.
	const maxSuffix = 22
	for i := 1; i < len(model); i++ {
		var literal int
		switch model[i] {
		case lbool.True:
			literal = i
		case lbool.False:
			literal = -i
		default:
			continue
		}
		if len(chunk) > len(buffer)-maxSuffix {
			if err := flush(); err != nil {
				return err
			}
		}
		chunk = strconv.AppendInt(chunk, int64(literal), 10)
		chunk = append(chunk, ' ')
	}
	chunk = append(chunk, '\n')
	return flush()
}
