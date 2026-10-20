package messages

// PosError is an error at a source position. Its text is "Satır {line}: {error}" (or with the
// column, "Satır {line}, Sütun {col}: {error}") in the active language; the position stays a
// field, so frames and tools read it from the error, never from the text. A line below 1 means
// the position is unknown: the text is the wrapped error's and Pos looks further down the chain.
type PosError struct {
	Line int // 1'den başlar
	Col  int // 0: bilinmiyor
	Err  error
}

// At returns err at the given source position (col 0 when unknown).
func At(line, col int, err error) *PosError { return &PosError{Line: line, Col: col, Err: err} }

func (e *PosError) Error() string {
	if code, args, ok := e.message(); ok {
		return Text(code, args...)
	}
	return errorText(e.Err)
}

// message konumlu metnin kodu ve değerleridir; satır bilinmiyorsa ok false'tur.
func (e *PosError) message() (Code, []any, bool) {
	switch {
	case e.Line <= 0:
		return 0, nil, false
	case e.Col > 0:
		return H2004, []any{e.Line, e.Col, e.Err}, true
	}
	return H2001, []any{e.Line, e.Err}, true
}

func (e *PosError) Unwrap() error { return e.Err }

// Pos returns the position of the first PosError with a known line in err's chain.
func Pos(err error) (line, col int, ok bool) {
	for err != nil {
		if p, isPos := err.(*PosError); isPos && p.Line > 0 {
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
