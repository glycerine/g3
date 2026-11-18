package api

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (r *Z3Replayer) Parse() error {
	if r == nil || r.closed || r.scanner == nil {
		return ErrZ3ReplayState
	}
	s := r.scanner
	// z3_replayer::parse: IF_VERBOSE(1) counts every loop iteration and
	// prints a progress line to std::cout each TICK_FREQUENCY of them.
	var counter uint64
	var tick uint32
	for {
		if replayerVerboseTicks {
			if on, e := diagnostic.ProcessVerboseEnabled(1); e == nil && on {
				counter++
				tick++
				if tick >= 100000 {
					if _, e = fmt.Fprintf(r.output, "[replayer] %d operations executed\n", counter); e != nil {
						return e
					}
					tick = 0
				}
			}
		}
		s.blanks()
		if s.eof {
			return nil
		}
		command := s.current
		commandLine := s.line
		if !z3ReplayCommandByte(command) {
			// The default case throws before next(): the line is the
			// command's own.
			return &Z3ReplayError{Line: commandLine, Message: "unknown log command"}
		}
		s.next()
		s.blanks()
		var err error
		switch command {
		case 'V':
			_, err = s.quoted('"')
		case 'R':
			r.resetArguments()
		case 'P':
			var p uint64
			p, err = s.hexadecimal()
			if err == nil {
				err = r.pushObject(p)
			}
		case 'S':
			var v string
			v, err = s.quoted('"')
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayString, s: v})
			}
		case 'N':
			r.args = append(r.args, z3ReplayValue{kind: replaySymbol, nullSymbol: true})
		case '$':
			var v string
			v, err = s.quoted('|')
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replaySymbol, s: v})
			}
		case '#':
			var v uint64
			v, err = s.uint64()
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replaySymbol, s: fmt.Sprint(v), u: v, numericSymbol: true})
			}
		case 'I':
			var v int64
			v, err = s.int64()
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayInt64, i: v})
			}
		case 'U':
			var v uint64
			v, err = s.uint64()
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayUint64, u: v})
			}
		case 'F':
			var v float64
			v, err = s.floating(32)
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayFloat, f: float32(v)})
			}
		case 'D':
			var v float64
			v, err = s.floating(64)
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayDouble, d: v})
			}
		case 'p', 's', 'u', 'i':
			var n uint64
			n, err = s.uint64()
			if err == nil {
				kind := replayObject
				if command == 's' {
					kind = replaySymbol
				}
				if command == 'u' {
					kind = replayUint64
				}
				// 'i' int arrays, as upstream fed977b49 ("fix #1782") logs
				// int array arguments (AUDIT.md api_03-F3).
				if command == 'i' {
					kind = replayInt64
				}
				err = r.pushArray(n, kind)
			}
		case 'C':
			var id uint64
			id, err = s.uint64()
			if err == nil {
				entry, found := r.commands[uint32(id)]
				if !found {
					err = r.failure("invalid command")
				} else {
					err = entry.call(r)
					var commandErr *Z3ReplayCommandError
					if errors.As(err, &commandErr) {
						_, writeErr := fmt.Fprintln(r.output, "[z3 exception]: "+commandErr.Message)
						err = writeErr
					} else if err != nil && !isZ3ReplayStructuralError(err) {
						// The API call failed inside its own context: the C
						// API sets the error code (or catches the exception)
						// and invokes the context's error handler, returns
						// null, and the replay goes on.
						if owner := r.commandContext(); owner != nil {
							owner.results.handleAPIError(err)
							r.result = 0
							err = r.handlerErr
						}
					}
					if err == nil && r.handlerErr != nil {
						err = r.handlerErr
					}
					if err == nil && r.exit != nil {
						return r.exit
					}
				}
			}
		case '=':
			var p uint64
			p, err = s.hexadecimal()
			if err == nil {
				r.saveResult(p)
			}
		case '*':
			var p, pos uint64
			p, err = s.hexadecimal()
			if err == nil {
				s.blanks()
				pos, err = s.uint64()
			}
			if err == nil {
				err = r.saveArgument(p, pos)
			}
		case '@':
			var p, pos, idx uint64
			p, err = s.hexadecimal()
			if err == nil {
				s.blanks()
				pos, err = s.uint64()
			}
			if err == nil {
				s.blanks()
				idx, err = s.uint64()
			}
			if err == nil {
				err = r.saveArrayElement(p, pos, idx)
			}
		case 'M':
			var message string
			message, err = s.quoted('"')
			if err == nil {
				_, err = fmt.Fprintln(r.output, message)
			}
		default:
			err = fmt.Errorf("unknown log command")
		}
		if err != nil {
			if _, ok := err.(*Z3ReplayError); ok {
				return err
			}
			return r.failure(err.Error())
		}
	}
}

// commandContext returns the context passed as argument 0 of the current
// command, the context of every API call that can set an error code.
func (r *Z3Replayer) commandContext() *replayContextOwner {
	if len(r.args) == 0 || r.args[0].kind != replayObject {
		return nil
	}
	return r.contexts[r.args[0].u]
}

func z3ReplayCommandByte(c byte) bool {
	switch c {
	case 'V', 'R', 'P', 'S', 'N', '$', '#', 'I', 'U', 'F', 'D', 'p', 's', 'u', 'i', 'C', '=', '*', '@', 'M':
		return true
	}
	return false
}
