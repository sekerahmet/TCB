package messages

// Range is a stretch of source text (docs/spec/02_sozcuk_yapisi.md 2.1, 2.13): lines and columns
// start at 1, a column counts Unicode characters, the end is exclusive (the position after the
// last character). A zero Col means the column is unknown; a zero EndLine means the end is.
type Range struct {
	Line, Col       int
	EndLine, EndCol int
}

// Related is another place an error is about ("first defined here"). File is empty when it is the
// file of the error itself; the writer of a tool record fills the others (an imported module).
type Related struct {
	File string
	Range
	Err error
}

// Fix is a suggested change: the text of Range is replaced by Replacement. Applicable says it is
// right as it stands, so a tool may apply it without asking (the two examples of 2.13: a '//' at the
// start of a line to '#', 0X1F to 0x1F); otherwise it is a suggestion that may be wrong or that
// needs more changes than the one place (a name suggested by its likeness, a name's other uses).
type Fix struct {
	Range
	Replacement string
	Applicable  bool
}

// AtRange returns err at r.
func AtRange(r Range, err error) *PosError {
	e := &PosError{Line: r.Line, Col: r.Col, Err: err}
	if r.Line > 0 && r.Col > 0 && r.EndLine > 0 && r.EndCol > 0 {
		e.EndLine, e.EndCol = r.EndLine, r.EndCol
	}
	return e
}

// Range is the place e is at, with its end.
func (e *PosError) Range() Range {
	return Range{Line: e.Line, Col: e.Col, EndLine: e.EndLine, EndCol: e.EndCol}
}

// WithRelated adds related places and returns e.
func (e *PosError) WithRelated(rel ...Related) *PosError {
	e.Related = append(e.Related, rel...)
	return e
}

// WithFix adds suggested fixes and returns e.
func (e *PosError) WithFix(fixes ...Fix) *PosError {
	e.Fixes = append(e.Fixes, fixes...)
	return e
}

// InnerCode is the code a tool record carries (2.13): the innermost message of err, not the codes
// that wrap it (the parser wraps every syntax error in H0100, the checker an undefined name in
// H1010; a record with the wrapper's code could not tell them apart). It goes down the cause slots
// only (Root). ok is false when err holds no coded message.
func InnerCode(err error) (Code, bool) {
	m, ok := Root(err)
	if !ok {
		return 0, false
	}
	return m.Code, true
}
