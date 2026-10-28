package messages

import (
	"errors"
	"reflect"
	"testing"
)

func TestAtRangeKeepsTheEndOnlyWhenTheColumnsAreKnown(t *testing.T) {
	err := New(H0100, New(H0009))
	cases := []struct {
		name string
		in   Range
		want Range
	}{
		{"whole range", Range{Line: 3, Col: 5, EndLine: 3, EndCol: 7}, Range{Line: 3, Col: 5, EndLine: 3, EndCol: 7}},
		{"no end", Range{Line: 3, Col: 5}, Range{Line: 3, Col: 5}},
		{"no column", Range{Line: 3, EndLine: 3, EndCol: 7}, Range{Line: 3}},
		{"no end column", Range{Line: 3, Col: 5, EndLine: 3}, Range{Line: 3, Col: 5}},
	}
	for _, c := range cases {
		if got := AtRange(c.in, err).Range(); got != c.want {
			t.Errorf("%s: AtRange(%+v).Range() = %+v, want %+v", c.name, c.in, got, c.want)
		}
	}
}

func TestAtRangeTextIsTheTextOfAt(t *testing.T) {
	err := New(H0100, New(H0009))
	for _, c := range []struct{ line, col int }{{3, 5}, {3, 0}} {
		plain := At(c.line, c.col, err).Error()
		ranged := AtRange(Range{Line: c.line, Col: c.col, EndLine: c.line, EndCol: c.col + 2}, err).Error()
		if plain != ranged {
			t.Errorf("line %d col %d: text %q with a range, %q without", c.line, c.col, ranged, plain)
		}
	}
}

func TestFindReturnsTheWholeRecord(t *testing.T) {
	rel := Related{Range: Range{Line: 1, Col: 1, EndLine: 1, EndCol: 4}, Err: New(H0009)}
	fix := Fix{Range: Range{Line: 3, Col: 5, EndLine: 3, EndCol: 7}, Replacement: "#", Applicable: true}
	pos := AtRange(Range{Line: 3, Col: 5, EndLine: 3, EndCol: 7}, New(H0100, New(H0009))).WithRelated(rel).WithFix(fix)
	wrapped := errors.Join(errors.New("other"), pos)
	got, ok := Find(wrapped)
	if !ok || got != pos {
		t.Fatalf("Find(joined) = %v, %v, want the PosError itself", got, ok)
	}
	if !reflect.DeepEqual(got.Related, []Related{rel}) || !reflect.DeepEqual(got.Fixes, []Fix{fix}) {
		t.Errorf("related %+v, fixes %+v", got.Related, got.Fixes)
	}
	if _, ok := Find(At(0, 0, New(H0009))); ok {
		t.Errorf("a PosError without a line is a position, Find must look further")
	}
}

func TestInnerCodeSkipsTheWrappers(t *testing.T) {
	inner := New(H0009)
	for _, err := range []error{inner, New(H0100, inner), At(2, 3, New(H0100, inner))} {
		if got, ok := InnerCode(err); !ok || got != H0009 {
			t.Errorf("InnerCode(%v) = %v, %v, want H0009", err, got, ok)
		}
	}
	if _, ok := InnerCode(errors.New("plain")); ok {
		t.Errorf("a plain error has no code")
	}
}
