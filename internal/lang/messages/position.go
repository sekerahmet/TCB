package messages

// PosError is an error at a source position. Its text is "Satır {line}: {error}" (or with the
// column, "Satır {line}, Sütun {col}: {error}") in the active language; the position stays a
// field, so frames and tools read it from the error, never from the text.
type PosError struct {
	Line int // 1'den başlar
	Col  int // 0: bilinmiyor
	Err  error
}

// At returns err at the given source position (col 0 when unknown).
func At(line, col int, err error) *PosError { return &PosError{Line: line, Col: col, Err: err} }

func (e *PosError) Error() string {
	if e.Col > 0 {
		return Text(H2004, e.Line, e.Col, e.Err)
	}
	return Text(H2001, e.Line, e.Err)
}

func (e *PosError) Unwrap() error { return e.Err }

// Pos returns the position of the first PosError in err's chain.
func Pos(err error) (line, col int, ok bool) {
	for err != nil {
		if p, isPos := err.(*PosError); isPos {
			return p.Line, p.Col, true
		}
		switch u := err.(type) {
		case interface{ Unwrap() error }:
			err = u.Unwrap()
		case interface{ Unwrap() []error }:
			for _, e := range u.Unwrap() {
				if l, c, found := Pos(e); found {
					return l, c, true
				}
			}
			return 0, 0, false
		default:
			return 0, 0, false
		}
	}
	return 0, 0, false
}
