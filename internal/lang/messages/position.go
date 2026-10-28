package messages

// PosError is an error at a source position. Its text is "Satır {line}: {error}" (or with the
// column, "Satır {line}, Sütun {col}: {error}") in the active language; the position stays a
// field, so frames and tools read it from the error, never from the text. A line below 1 means
// the position is unknown: the text is the wrapped error's and Pos looks further down the chain.
//
// The record beyond the start (the end, related locations and fixes, docs/spec/02_sozcuk_yapisi.md
// 2.13) is data for tools: the text above never shows it. Build it with At, AtRange and the With
// methods (span.go).
type PosError struct {
	Line int // 1'den başlar
	Col  int // 0: bilinmiyor
	// EndLine and EndCol are the position after the last character (exclusive, like LSP and rustc);
	// 0 when the end is unknown. EndCol is 0 whenever Col is.
	EndLine, EndCol int
	Err             error
	Related         []Related
	Fixes           []Fix
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
	if p, found := Find(err); found {
		return p.Line, p.Col, true
	}
	return 0, 0, false
}

// Find returns the first PosError with a known line in err's chain, whole: with its end, related
// locations and fixes (Pos returns only the start).
func Find(err error) (*PosError, bool) {
	for err != nil {
		if p, isPos := err.(*PosError); isPos && p.Line > 0 {
			return p, true
		}
		switch u := err.(type) {
		case interface{ Unwrap() error }:
			err = u.Unwrap()
		case interface{ Unwrap() []error }:
			for _, e := range u.Unwrap() {
				if p, found := Find(e); found {
					return p, true
				}
			}
			return nil, false
		default:
			return nil, false
		}
	}
	return nil, false
}
